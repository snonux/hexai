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
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	inlinePrompt := lineHasInlinePrompt(current)
	if !inlinePrompt && !s.isTriggerEvent(p, current) {
		logging.Logf("lsp ", "%scompletion skip=no-trigger line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return []CompletionItem{}, true
	}
	if s.shouldSuppressForChatTriggerEOL(current, p) {
		return []CompletionItem{}, true
	}

	inParams := inParamList(current, p.Position.Character)
	manualInvoke := parseManualInvoke(p.Context)

	// Cache fast-path
	key := s.completionCacheKey(p, above, current, below, funcCtx, inParams, hasExtra, extraText)
	if cleaned, ok := s.completionCacheGet(key); ok && strings.TrimSpace(cleaned) != "" {
		logging.Logf("lsp ", "completion cache hit uri=%s line=%d char=%d preview=%s%s%s",
			p.TextDocument.URI, p.Position.Line, p.Position.Character,
			logging.AnsiGreen, logging.PreviewForLog(cleaned), logging.AnsiBase)
		return s.makeCompletionItems(cleaned, inParams, current, p, docStr), true
	}
	if isBareDoubleOpen(current) || isBareDoubleOpen(below) {
		logging.Logf("lsp ", "%scompletion skip=empty-double-semicolon line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return []CompletionItem{}, true
	}

	if !inParams && !s.prefixHeuristicAllows(inlinePrompt, current, p, manualInvoke) {
		logging.Logf("lsp ", "%scompletion skip=short-prefix line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return []CompletionItem{}, true
	}

	// Provider-native path
	if items, ok := s.tryProviderNativeCompletion(current, p, above, below, funcCtx, docStr, hasExtra, extraText, inParams); ok {
		return items, true
	}

	// Chat path
	messages := s.buildCompletionMessages(inlinePrompt, hasExtra, extraText, inParams, p, above, current, below, funcCtx)
	// Counters and options
	sentSize := 0
	for _, m := range messages {
		sentSize += len(m.Content)
	}
	s.incSentCounters(sentSize)
	opts := []llm.RequestOption{llm.WithMaxTokens(s.maxTokens)}
	if s.codingTemperature != nil {
		opts = append(opts, llm.WithTemperature(*s.codingTemperature))
	}
	// Debounce and throttle before making the LLM call
	s.waitForDebounce(ctx)
	if !s.waitForThrottle(ctx) {
		return nil, false
	}
	logging.Logf("lsp ", "completion llm=requesting model=%s", s.llmClient.DefaultModel())

	text, err := s.llmClient.Chat(ctx, messages, opts...)
	if err != nil {
		logging.Logf("lsp ", "llm completion error: %v", err)
		s.logLLMStats()
		return nil, false
	}
	s.incRecvCounters(len(text))
	s.logLLMStats()

	cleaned := s.postProcessCompletion(strings.TrimSpace(text), current[:p.Position.Character], current)
	if cleaned == "" {
		return nil, false
	}
	s.completionCachePut(key, cleaned)
	return s.makeCompletionItems(cleaned, inParams, current, p, docStr), true
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
	if s.chatSuffix == "" {
		return false
	}
	if strings.HasSuffix(t, s.chatSuffix) {
		if len(t) < len(s.chatSuffix)+1 {
			return false
		}
		prev := string(t[len(t)-len(s.chatSuffix)-1])
		for _, pf := range s.chatPrefixes {
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
	if manualInvoke && s.manualInvokeMinPrefix >= 0 {
		min = s.manualInvokeMinPrefix
	}
	return j-start >= min
}

// tryProviderNativeCompletion attempts provider-native completion and returns items when successful.
func (s *Server) tryProviderNativeCompletion(current string, p CompletionParams, above, below, funcCtx, docStr string, hasExtra bool, extraText string, inParams bool) ([]CompletionItem, bool) {
	cc, ok := s.llmClient.(llm.CodeCompleter)
	if !ok {
		return nil, false
	}
	before, after := s.docBeforeAfter(p.TextDocument.URI, p.Position)
	path := strings.TrimPrefix(p.TextDocument.URI, "file://")
	// Build provider-native prompt from template
	prompt := renderTemplate(s.promptNativeCompletion, map[string]string{
		"path":   path,
		"before": before,
	})
	lang := ""
	temp := 0.0
	if s.codingTemperature != nil {
		temp = *s.codingTemperature
	}
	prov := ""
	if s.llmClient != nil {
		prov = s.llmClient.Name()
	}
	logging.Logf("lsp ", "completion path=codex provider=%s uri=%s", prov, path)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
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
		if s.llmClient != nil {
			_ = stats.Update(ctx2, s.llmClient.Name(), s.llmClient.DefaultModel(), sentBytes, len(suggestions[0]))
		}
		s.logLLMStats()
		cleaned := strings.TrimSpace(suggestions[0])
		if cleaned != "" {
			cleaned = stripDuplicateAssignmentPrefix(current[:p.Position.Character], cleaned)
			if cleaned != "" {
				cleaned = stripDuplicateGeneralPrefix(current[:p.Position.Character], cleaned)
			}
			if cleaned != "" && hasDoubleOpenTrigger(current) {
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
	d := s.completionDebounce
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
	interval := s.throttleInterval
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
	// Vars for templates
	vars := map[string]string{
		"file":     p.TextDocument.URI,
		"function": funcCtx,
		"above":    above,
		"current":  current,
		"below":    below,
		"char":     fmt.Sprintf("%d", p.Position.Character),
	}
	sys := s.promptCompSysGeneral
	userTpl := s.promptCompUserGeneral
	if inParams {
		sys = s.promptCompSysParams
		userTpl = s.promptCompUserParams
	}
	if inlinePrompt && strings.TrimSpace(s.promptCompSysInline) != "" {
		sys = s.promptCompSysInline
	}
	user := renderTemplate(userTpl, vars)
	messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	if hasExtra && strings.TrimSpace(extraText) != "" {
		extra := renderTemplate(s.promptCompExtraHeader, map[string]string{"context": extraText})
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
	if cleaned != "" && hasDoubleOpenTrigger(currentLine) {
		if indent := leadingIndent(currentLine); indent != "" {
			cleaned = applyIndent(indent, cleaned)
		}
	}
	return cleaned
}
