// Summary: Completion handlers split from handlers.go to reduce file size and isolate feature logic.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
)

type completionPlan struct {
	params       CompletionParams
	above        string
	current      string
	below        string
	funcCtx      string
	docStr       string
	hasExtra     bool
	extraText    string
	inlinePrompt bool
	inParams     bool
	manualInvoke bool
	cacheKey     string
}

func (s *Server) handleCompletion(req Request) {
	var p CompletionParams
	var docStr string
	if err := json.Unmarshal(req.Params, &p); err == nil {
		// Log trigger information for every completion request from client
		tk, tch := extractTriggerInfo(p)
		logging.Logf("lsp ", "completion trigger kind=%d char=%q uri=%s line=%d char=%d",
			tk, tch, p.TextDocument.URI, p.Position.Line, p.Position.Character)
		above, current, below, funcCtx := s.lineContext(p.TextDocument.URI, p.Position)
		docStr = s.buildDocString(p, above, current, below, funcCtx)
		if s.logContext {
			s.logCompletionContext(p, above, current, below, funcCtx)
		}
		if s.llmClient != nil {
			newFunc := s.isDefiningNewFunction(p.TextDocument.URI, p.Position)
			extra, has := s.buildAdditionalContext(newFunc, p.TextDocument.URI, p.Position)
			items, ok := s.tryLLMCompletion(p, above, current, below, funcCtx, docStr, has, extra)
			if ok {
				s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
				return
			}
		}
	}
	items := s.fallbackCompletionItems(docStr)
	s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
}

// extractTriggerInfo returns the LSP completion TriggerKind and TriggerCharacter
// if provided by the client; when absent it returns zeros.
func extractTriggerInfo(p CompletionParams) (kind int, ch string) {
	if p.Context == nil {
		return 0, ""
	}
	var ctx struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter,omitempty"`
	}
	if raw, ok := p.Context.(json.RawMessage); ok {
		_ = json.Unmarshal(raw, &ctx)
	} else {
		b, _ := json.Marshal(p.Context)
		_ = json.Unmarshal(b, &ctx)
	}
	return ctx.TriggerKind, ctx.TriggerCharacter
}

// --- completion helpers ---

func (s *Server) buildDocString(p CompletionParams, above, current, below, funcCtx string) string {
	return fmt.Sprintf("file: %s\nline: %d\nabove: %s\ncurrent: %s\nbelow: %s\nfunction: %s",
		p.TextDocument.URI, p.Position.Line, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
}

func (s *Server) logCompletionContext(p CompletionParams, above, current, below, funcCtx string) {
	logging.Logf("lsp ", "completion ctx uri=%s line=%d char=%d above=%q current=%q below=%q function=%q",
		p.TextDocument.URI, p.Position.Line, p.Position.Character, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
}

func (s *Server) tryLLMCompletion(p CompletionParams, above, current, below, funcCtx, docStr string, hasExtra bool, extraText string) ([]CompletionItem, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	plan, items, handled := s.prepareCompletionPlan(p, above, current, below, funcCtx, docStr, hasExtra, extraText)
	if handled {
		return items, true
	}

	if items, ok := s.tryProviderNativeCompletion(current, p, above, below, funcCtx, docStr, hasExtra, extraText, plan.inParams); ok {
		return items, true
	}

	return s.executeChatCompletion(ctx, plan)
}

func (s *Server) prepareCompletionPlan(p CompletionParams, above, current, below, funcCtx, docStr string, hasExtra bool, extraText string) (completionPlan, []CompletionItem, bool) {
	plan := completionPlan{
		params:    p,
		above:     above,
		current:   current,
		below:     below,
		funcCtx:   funcCtx,
		docStr:    docStr,
		hasExtra:  hasExtra,
		extraText: extraText,
	}
	_, _, openChar, closeChar := s.inlineMarkers()
	plan.inlinePrompt = lineHasInlinePrompt(current, openChar, closeChar)
	if !plan.inlinePrompt && !s.isTriggerEvent(p, current) {
		logging.Logf("lsp ", "%scompletion skip=no-trigger line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return plan, []CompletionItem{}, true
	}
	if s.shouldSuppressForChatTriggerEOL(current, p) {
		return plan, []CompletionItem{}, true
	}
	plan.inParams = inParamList(current, p.Position.Character)
	plan.manualInvoke = parseManualInvoke(p.Context)
	plan.cacheKey = s.completionCacheKey(p, above, current, below, funcCtx, plan.inParams, hasExtra, extraText)
	if cleaned, ok := s.completionCacheGet(plan.cacheKey); ok && strings.TrimSpace(cleaned) != "" {
		logging.Logf("lsp ", "completion cache hit uri=%s line=%d char=%d preview=%s%s%s",
			p.TextDocument.URI, p.Position.Line, p.Position.Character,
			logging.AnsiGreen, logging.PreviewForLog(cleaned), logging.AnsiBase)
		return plan, s.makeCompletionItems(cleaned, plan.inParams, current, p, docStr), true
	}
	if isBareDoubleOpen(current, openChar, closeChar) || isBareDoubleOpen(below, openChar, closeChar) {
		logging.Logf("lsp ", "%scompletion skip=empty-double-semicolon line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return plan, []CompletionItem{}, true
	}
	if !plan.inParams && !s.prefixHeuristicAllows(plan.inlinePrompt, current, p, plan.manualInvoke) {
		logging.Logf("lsp ", "%scompletion skip=short-prefix line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return plan, []CompletionItem{}, true
	}
	return plan, nil, false
}

func (s *Server) executeChatCompletion(ctx context.Context, plan completionPlan) ([]CompletionItem, bool) {
	messages := s.buildCompletionMessages(plan.inlinePrompt, plan.hasExtra, plan.extraText, plan.inParams, plan.params, plan.above, plan.current, plan.below, plan.funcCtx)
	sentSize := 0
	for _, m := range messages {
		sentSize += len(m.Content)
	}
	s.incSentCounters(sentSize)
	opts := s.llmRequestOpts()
	s.waitForDebounce(ctx)
	if !s.waitForThrottle(ctx) {
		return nil, false
	}
	client := s.currentLLMClient()
	if client == nil {
		return nil, false
	}
	logging.Logf("lsp ", "completion llm=requesting model=%s", client.DefaultModel())
	text, err := client.Chat(ctx, messages, opts...)
	if err != nil {
		logging.Logf("lsp ", "llm completion error: %v", err)
		s.logLLMStats()
		return nil, false
	}
	s.incRecvCounters(len(text))
	s.logLLMStats()
	trimmed := strings.TrimSpace(text)
	cleaned := s.postProcessCompletion(trimmed, plan.current[:plan.params.Position.Character], plan.current)
	if cleaned == "" {
		return nil, false
	}
	s.completionCachePut(plan.cacheKey, cleaned)
	items := s.makeCompletionItems(cleaned, plan.inParams, plan.current, plan.params, plan.docStr)
	return items, true
}

// parseManualInvoke inspects the LSP completion context and reports whether the user manually invoked completion.
func parseManualInvoke(ctx any) bool {
	if ctx == nil {
		return false
	}
	var c struct {
		TriggerKind int `json:"triggerKind"`
	}
	if raw, ok := ctx.(json.RawMessage); ok {
		_ = json.Unmarshal(raw, &c)
	} else {
		b, _ := json.Marshal(ctx)
		_ = json.Unmarshal(b, &c)
	}
	return c.TriggerKind == 1
}

// shouldSuppressForChatTriggerEOL returns true when a chat trigger like ">" follows ?, !, :, or ; at EOL.
func (s *Server) shouldSuppressForChatTriggerEOL(current string, p CompletionParams) bool {
	t := strings.TrimRight(current, " \t")
	suffix, prefixes, _ := s.chatConfig()
	if suffix == "" {
		return false
	}
	if strings.HasSuffix(t, suffix) {
		if len(t) < len(suffix)+1 {
			return false
		}
		prev := string(t[len(t)-len(suffix)-1])
		for _, pf := range prefixes {
			if prev == pf {
				logging.Logf("lsp ", "completion skip=chat-trigger-eol uri=%s line=%d", p.TextDocument.URI, p.Position.Line)
				return true
			}
		}
	}
	return false
}

// prefixHeuristicAllows applies minimal prefix rules unless inlinePrompt or structural triggers apply.
func (s *Server) prefixHeuristicAllows(inlinePrompt bool, current string, p CompletionParams, manualInvoke bool) bool {
	// Determine the effective cursor index within current line, clamped, and
	// skip over trailing spaces/tabs to support cases like "type Matrix| ".
	idx := p.Position.Character
	if idx > len(current) {
		idx = len(current)
	}
	allowNoPrefix := inlinePrompt
	if idx > 0 {
		ch := current[idx-1]
		if ch == '.' || ch == ':' || ch == '/' || ch == '_' || ch == ')' {
			allowNoPrefix = true
		}
	}
	if allowNoPrefix {
		return true
	}
	// Walk left over whitespace
	j := idx
	for j > 0 {
		c := current[j-1]
		if c == ' ' || c == '\t' {
			j--
			continue
		}
		break
	}
	start := computeWordStart(current, j)
	min := 1
	if manualInvoke {
		if v := s.manualInvokeMinPrefix(); v >= 0 {
			min = v
		}
	}
	return j-start >= min
}

// tryProviderNativeCompletion attempts provider-native completion and returns items when successful.
func (s *Server) tryProviderNativeCompletion(current string, p CompletionParams, above, below, funcCtx, docStr string, hasExtra bool, extraText string, inParams bool) ([]CompletionItem, bool) {
	client := s.currentLLMClient()
	cc, ok := client.(llm.CodeCompleter)
	if !ok {
		return nil, false
	}
	before, after := s.docBeforeAfter(p.TextDocument.URI, p.Position)
	path := strings.TrimPrefix(p.TextDocument.URI, "file://")
	// Build provider-native prompt from template
	cfg := s.currentConfig()
	_, _, openChar, closeChar := s.inlineMarkers()
	prompt := renderTemplate(cfg.PromptNativeCompletion, map[string]string{
		"path":   path,
		"before": before,
	})
	lang := ""
	temp := 0.0
	if cfg.CodingTemperature != nil {
		temp = *cfg.CodingTemperature
	}
	prov := ""
	if client != nil {
		prov = client.Name()
	}
	logging.Logf("lsp ", "completion path=codex provider=%s uri=%s", prov, path)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	// Debounce and throttle prior to provider-native call
	s.waitForDebounce(ctx2)
	if !s.waitForThrottle(ctx2) {
		return nil, false
	}
	// Count approximate payload sizes: prompt+after sent; first suggestion received
	sentBytes := len(prompt) + len(after)
	suggestions, err := cc.CodeCompletion(ctx2, prompt, after, 1, lang, temp)
	if err == nil && len(suggestions) > 0 {
		// Update counters and heartbeat
		s.incSentCounters(sentBytes)
		s.incRecvCounters(len(suggestions[0]))
		// Contribute to global stats (provider-native path)
		if client != nil {
			_ = stats.Update(ctx2, client.Name(), client.DefaultModel(), sentBytes, len(suggestions[0]))
		}
		s.logLLMStats()
		cleaned := strings.TrimSpace(suggestions[0])
		if cleaned != "" {
			cleaned = stripDuplicateAssignmentPrefix(current[:p.Position.Character], cleaned)
			if cleaned != "" {
				cleaned = stripDuplicateGeneralPrefix(current[:p.Position.Character], cleaned)
			}
			if cleaned != "" && hasDoubleOpenTrigger(current, openChar, closeChar) {
				indent := leadingIndent(current)
				if indent != "" {
					cleaned = applyIndent(indent, cleaned)
				}
			}
			if strings.TrimSpace(cleaned) != "" {
				key := s.completionCacheKey(p, above, current, below, funcCtx, inParams, hasExtra, extraText)
				s.completionCachePut(key, cleaned)
				return s.makeCompletionItems(cleaned, inParams, current, p, docStr), true
			}
		}
	} else if err != nil {
		logging.Logf("lsp ", "completion path=codex error=%v (falling back to chat)", err)
		// Still emit a heartbeat for visibility, even on error
		s.incSentCounters(sentBytes)
		s.logLLMStats()
	}
	return nil, false
}

// waitForDebounce sleeps until there has been no input activity for at least
// completionDebounce. If debounce is zero or ctx is done, it returns promptly.
func (s *Server) waitForDebounce(ctx context.Context) {
	d := s.completionDebounce()
	if d <= 0 {
		return
	}
	for {
		s.mu.RLock()
		last := s.lastInput
		s.mu.RUnlock()
		if last.IsZero() {
			return
		}
		since := time.Since(last)
		if since >= d {
			return
		}
		rem := d - since
		timer := time.NewTimer(rem)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			// loop and re-evaluate in case input occurred during sleep
		}
	}
}

// waitForThrottle enforces a minimum spacing between LLM calls. Returns false
// if the context is canceled while waiting.
func (s *Server) waitForThrottle(ctx context.Context) bool {
	interval := s.completionThrottle()
	if interval <= 0 {
		return true
	}
	var wait time.Duration
	for {
		s.mu.Lock()
		next := s.lastLLMCall.Add(interval)
		now := time.Now()
		if now.Before(next) {
			wait = next.Sub(now)
			s.mu.Unlock()
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
				// try again to set the next call time
				continue
			}
		}
		// we are allowed to proceed now; record this call as the latest
		s.lastLLMCall = now
		s.mu.Unlock()
		return true
	}
}

// buildCompletionMessages constructs the LLM messages for completion.
func (s *Server) buildCompletionMessages(inlinePrompt, hasExtra bool, extraText string, inParams bool, p CompletionParams, above, current, below, funcCtx string) []llm.Message {
	vars := map[string]string{
		"file":     p.TextDocument.URI,
		"function": funcCtx,
		"above":    above,
		"current":  current,
		"below":    below,
		"char":     fmt.Sprintf("%d", p.Position.Character),
	}
	cfg := s.currentConfig()
	sys := cfg.PromptCompletionSystemGeneral
	userTpl := cfg.PromptCompletionUserGeneral
	if inParams {
		sys = cfg.PromptCompletionSystemParams
		userTpl = cfg.PromptCompletionUserParams
	}
	if inlinePrompt && strings.TrimSpace(cfg.PromptCompletionSystemInline) != "" {
		sys = cfg.PromptCompletionSystemInline
	}
	user := renderTemplate(userTpl, vars)
	messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	if hasExtra && strings.TrimSpace(extraText) != "" {
		extra := renderTemplate(cfg.PromptCompletionExtraHeader, map[string]string{"context": extraText})
		if strings.TrimSpace(extra) == "" {
			extra = extraText
		}
		messages = append(messages, llm.Message{Role: "user", Content: extra})
	}
	return messages
}

// postProcessCompletion normalizes and deduplicates completion text and applies indentation rules.
func (s *Server) postProcessCompletion(text string, leftOfCursor string, currentLine string) string {
	cleaned := stripCodeFences(text)
	if cleaned != "" && strings.ContainsRune(cleaned, '`') {
		if inline := stripInlineCodeSpan(cleaned); strings.TrimSpace(inline) != "" {
			cleaned = inline
		}
	}
	if cleaned != "" {
		cleaned = stripDuplicateAssignmentPrefix(leftOfCursor, cleaned)
	}
	if cleaned != "" {
		cleaned = stripDuplicateGeneralPrefix(leftOfCursor, cleaned)
	}
	_, _, openChar, closeChar := s.inlineMarkers()
	if cleaned != "" && hasDoubleOpenTrigger(currentLine, openChar, closeChar) {
		if indent := leadingIndent(currentLine); indent != "" {
			cleaned = applyIndent(indent, cleaned)
		}
	}
	return cleaned
}
