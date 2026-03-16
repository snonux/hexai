// Summary: Completion handlers split from handlers.go to reduce file size and isolate feature logic.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

type completionJobResult struct {
	items []CompletionItem
	ok    bool
}

func (s *Server) handleCompletion(req Request) {
	if s.completionDisabled() {
		s.reply(req.ID, CompletionList{IsIncomplete: false, Items: nil}, nil)
		return
	}
	var p CompletionParams
	var docStr string
	if err := json.Unmarshal(req.Params, &p); err == nil {
		// Skip completion for gitignored / extra-pattern-ignored files
		if ignored, reason := s.isFileIgnored(p.TextDocument.URI); ignored {
			logging.Logf("lsp ", "completion skipped: file ignored (%s) uri=%s", reason, p.TextDocument.URI)
			if s.ignoreLSPNotifyEnabled() {
				s.reply(req.ID, CompletionList{IsIncomplete: false, Items: []CompletionItem{
					{Label: "[hexai] file ignored", Detail: reason},
				}}, nil)
			} else {
				s.reply(req.ID, CompletionList{IsIncomplete: false, Items: nil}, nil)
			}
			return
		}
		// Log trigger information for every completion request from client
		tk, tch := extractTriggerInfo(p)
		logging.Logf("lsp ", "completion trigger kind=%d char=%q uri=%s line=%d char=%d",
			tk, tch, p.TextDocument.URI, p.Position.Line, p.Position.Character)
		above, current, below, funcCtx := s.lineContext(p.TextDocument.URI, p.Position)
		docStr = s.buildDocString(p, above, current, below, funcCtx)
		if s.logContext {
			s.logCompletionContext(p, above, current, below, funcCtx)
		}
		if s.currentLLMClient() != nil {
			newFunc := s.isDefiningNewFunction(p.TextDocument.URI, p.Position)
			extra, has := s.buildAdditionalContext(newFunc, p.TextDocument.URI, p.Position)
			items, ok, incomplete := s.tryLLMCompletion(p, above, current, below, funcCtx, docStr, has, extra)
			if ok {
				s.reply(req.ID, CompletionList{IsIncomplete: incomplete, Items: items}, nil)
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

func (s *Server) tryLLMCompletion(p CompletionParams, above, current, below, funcCtx, docStr string, hasExtra bool, extraText string) ([]CompletionItem, bool, bool) {
	ctx, cancel := s.requestTimeoutContext(12 * time.Second)
	var cancelOnce sync.Once
	end := func() { cancelOnce.Do(cancel) }

	plan, items, handled := s.prepareCompletionPlan(p, above, current, below, funcCtx, docStr, hasExtra, extraText)
	if handled {
		end()
		return items, true, false
	}
	specs := s.buildRequestSpecs(surfaceCompletion)
	if len(specs) == 0 {
		end()
		return nil, false, false
	}
	results, started, ok := s.startCompletionJobs(ctx, plan, specs)
	if !ok || started == 0 {
		end()
		return nil, false, false
	}

	if started == 1 {
		items, ok := readSingleCompletionResult(results)
		if !ok {
			end()
			return nil, false, false
		}
		end()
		return items, true, false
	}

	if s.completionWaitAll() {
		combined := collectCompletionResults(results)
		end()
		if len(combined) == 0 {
			return nil, false, false
		}
		return combined, true, false
	}

	firstItems, ok := s.firstCompletionAndStore(results, plan.cacheKey, end)
	if !ok {
		end()
		return nil, false, false
	}
	return firstItems, true, true
}

func (s *Server) startCompletionJobs(ctx context.Context, plan completionPlan, specs []requestSpec) (<-chan completionJobResult, int, bool) {
	results := make(chan completionJobResult, len(specs))
	s.waitForDebounce(ctx)
	if !s.waitForThrottle(ctx) {
		close(results)
		return results, 0, false
	}
	var wg sync.WaitGroup
	started := 0
	for _, spec := range specs {
		spec := spec
		client := s.clientFor(spec)
		if client == nil {
			continue
		}
		started++
		wg.Add(1)
		go func(spec requestSpec, client llm.Client) {
			defer wg.Done()
			items, ok := s.runCompletionForSpec(ctx, plan, spec, client)
			results <- completionJobResult{items: items, ok: ok}
		}(spec, client)
	}
	if started == 0 {
		close(results)
		return results, 0, true
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results, started, true
}

func readSingleCompletionResult(results <-chan completionJobResult) ([]CompletionItem, bool) {
	res, ok := <-results
	if !ok || !res.ok || len(res.items) == 0 {
		return nil, false
	}
	return res.items, true
}

func collectCompletionResults(results <-chan completionJobResult) []CompletionItem {
	combined := make([]CompletionItem, 0)
	for res := range results {
		if !res.ok || len(res.items) == 0 {
			continue
		}
		combined = append(combined, res.items...)
	}
	return combined
}

func (s *Server) firstCompletionAndStore(results <-chan completionJobResult, cacheKey string, end func()) ([]CompletionItem, bool) {
	firstCh := make(chan []CompletionItem, 1)
	go s.collectFirstCompletion(results, cacheKey, firstCh, end)
	firstItems, ok := <-firstCh
	if !ok || len(firstItems) == 0 {
		return nil, false
	}
	return firstItems, true
}

func (s *Server) collectFirstCompletion(results <-chan completionJobResult, cacheKey string, firstCh chan<- []CompletionItem, end func()) {
	defer end()
	combined := make([]CompletionItem, 0)
	firstSent := false
	for res := range results {
		if !res.ok || len(res.items) == 0 {
			continue
		}
		combined = append(combined, res.items...)
		if !firstSent {
			first := make([]CompletionItem, len(res.items))
			copy(first, res.items)
			firstCh <- first
			firstSent = true
		}
	}
	if firstSent {
		s.storePendingCompletion(cacheKey, combined)
	}
	close(firstCh)
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
	openStr, _, openChar, closeChar := s.inlineMarkers()
	plan.inlinePrompt = lineHasInlinePrompt(current, openStr, openChar, closeChar)
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
	if pending := s.takePendingCompletion(plan.cacheKey); len(pending) > 0 {
		return plan, pending, true
	}
	if isBareDoubleOpen(current, openStr, openChar, closeChar) || isBareDoubleOpen(below, openStr, openChar, closeChar) {
		logging.Logf("lsp ", "%scompletion skip=empty-double-semicolon line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return plan, []CompletionItem{}, true
	}
	if !plan.inParams && !s.prefixHeuristicAllows(plan.inlinePrompt, current, p, plan.manualInvoke) {
		logging.Logf("lsp ", "%scompletion skip=short-prefix line=%d char=%d current=%q%s", logging.AnsiYellow, p.Position.Line, p.Position.Character, trimLen(current), logging.AnsiBase)
		return plan, []CompletionItem{}, true
	}
	return plan, nil, false
}

func (s *Server) runCompletionForSpec(ctx context.Context, plan completionPlan, spec requestSpec, client llm.Client) ([]CompletionItem, bool) {
	sortPrefix := fmt.Sprintf("%04d", spec.index)
	modelKey := spec.effectiveModel(client.DefaultModel())
	providerKey := spec.provider
	if providerKey == "" {
		providerKey = canonicalProvider(client.Name())
	}
	cacheKey := plan.cacheKey + "|" + providerKey + ":" + modelKey
	if cached, ok := s.completionCacheGet(cacheKey); ok && strings.TrimSpace(cached) != "" {
		logging.Logf("lsp ", "completion cache hit uri=%s line=%d char=%d preview=%s%s%s",
			plan.params.TextDocument.URI, plan.params.Position.Line, plan.params.Position.Character,
			logging.AnsiGreen, logging.PreviewForLog(cached), logging.AnsiBase)
		detail := fmt.Sprintf("Hexai %s:%s", client.Name(), modelKey)
		items := s.makeCompletionItems(cached, plan.inParams, plan.current, plan.params, plan.docStr, detail, sortPrefix)
		return items, true
	}
	if items, ok := s.tryProviderNativeCompletion(ctx, plan, spec, client, sortPrefix); ok {
		return items, true
	}
	return s.executeChatCompletion(ctx, plan, spec, client, sortPrefix)
}

func (s *Server) executeChatCompletion(ctx context.Context, plan completionPlan, spec requestSpec, client llm.Client, sortPrefix string) ([]CompletionItem, bool) {
	messages := s.buildCompletionMessages(plan.inlinePrompt, plan.hasExtra, plan.extraText, plan.inParams, plan.params, plan.above, plan.current, plan.below, plan.funcCtx)
	sentSize := 0
	for _, m := range messages {
		sentSize += len(m.Content)
	}
	s.incSentCounters(sentSize)
	text, err := client.Chat(ctx, messages, spec.options...)
	if err != nil {
		logging.Logf("lsp ", "llm completion error: %v", err)
		s.logLLMStats("")
		return nil, false
	}
	s.incRecvCounters(len(text))
	modelUsed := spec.effectiveModel(client.DefaultModel())
	_ = stats.Update(ctx, client.Name(), modelUsed, sentSize, len(text))
	s.logLLMStats(modelUsed)
	trimmed := strings.TrimSpace(text)
	cleaned := s.postProcessCompletion(trimmed, plan.current[:plan.params.Position.Character], plan.current)
	if cleaned == "" {
		return nil, false
	}
	detail := fmt.Sprintf("Hexai %s:%s", client.Name(), modelUsed)
	providerKey := spec.provider
	if providerKey == "" {
		providerKey = canonicalProvider(client.Name())
	}
	cacheKey := plan.cacheKey + "|" + providerKey + ":" + modelUsed
	s.completionCachePut(cacheKey, cleaned)
	items := s.makeCompletionItems(cleaned, plan.inParams, plan.current, plan.params, plan.docStr, detail, sortPrefix)
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
func (s *Server) tryProviderNativeCompletion(ctx context.Context, plan completionPlan, spec requestSpec, client llm.Client, sortPrefix string) ([]CompletionItem, bool) {
	cc, ok := client.(llm.CodeCompleter)
	if !ok {
		return nil, false
	}
	current := plan.current
	p := plan.params
	before, after := s.docBeforeAfter(p.TextDocument.URI, p.Position)
	path := strings.TrimPrefix(p.TextDocument.URI, "file://")
	cfg := s.currentConfig()
	openStr, _, openChar, closeChar := s.inlineMarkers()
	prompt := renderTemplate(cfg.PromptNativeCompletion, map[string]string{
		"path":   path,
		"before": before,
	})
	provider := spec.provider
	if provider == "" {
		provider = canonicalProvider(cfg.Provider)
	}
	logging.Logf("lsp ", "completion path=codex provider=%s uri=%s", provider, path)
	ctx2, cancel2 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel2()
	sentBytes := len(prompt) + len(after)
	modelUsed := spec.effectiveModel(client.DefaultModel())
	tempVal := 0.0
	if val, ok := chooseSurfaceTemperature(cfg, spec.entry, provider, modelUsed); ok {
		tempVal = val
	}
	suggestions, err := cc.CodeCompletion(ctx2, prompt, after, 1, "", tempVal)
	if err != nil || len(suggestions) == 0 {
		if err != nil {
			logging.Logf("lsp ", "completion path=codex error=%v (falling back)", err)
		}
		return nil, false
	}
	s.incSentCounters(sentBytes)
	s.incRecvCounters(len(suggestions[0]))
	_ = stats.Update(ctx2, client.Name(), modelUsed, sentBytes, len(suggestions[0]))
	s.logLLMStats(modelUsed)
	cleaned := strings.TrimSpace(suggestions[0])
	if cleaned == "" {
		return nil, false
	}
	cleaned = stripDuplicateAssignmentPrefix(current[:p.Position.Character], cleaned)
	if cleaned == "" {
		return nil, false
	}
	cleaned = stripDuplicateGeneralPrefix(current[:p.Position.Character], cleaned)
	if cleaned == "" {
		return nil, false
	}
	if strings.TrimSpace(cleaned) != "" && hasDoubleOpenTrigger(current, openStr, openChar, closeChar) {
		indent := leadingIndent(current)
		if indent != "" {
			cleaned = applyIndent(indent, cleaned)
		}
	}
	if strings.TrimSpace(cleaned) == "" {
		return nil, false
	}
	detail := fmt.Sprintf("Hexai %s:%s", client.Name(), modelUsed)
	providerKey := provider
	if providerKey == "" {
		providerKey = canonicalProvider(client.Name())
	}
	cacheKey := plan.cacheKey + "|" + providerKey + ":" + modelUsed
	s.completionCachePut(cacheKey, cleaned)
	items := s.makeCompletionItems(cleaned, plan.inParams, current, p, plan.docStr, detail, sortPrefix)
	return items, true
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
	openStr, _, openChar, closeChar := s.inlineMarkers()
	if cleaned != "" && hasDoubleOpenTrigger(currentLine, openStr, openChar, closeChar) {
		if indent := leadingIndent(currentLine); indent != "" {
			cleaned = applyIndent(indent, cleaned)
		}
	}
	return cleaned
}
