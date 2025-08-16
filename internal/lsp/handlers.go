package lsp

import (
    "context"
    "encoding/json"
    "fmt"
    "hexai/internal"
    "hexai/internal/llm"
    "hexai/internal/logging"
    "os"
    "strings"
    "time"
)

func (s *Server) handle(req Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		s.handleInitialized()
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		s.handleExit()
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	default:
		if len(req.ID) != 0 {
			s.reply(req.ID, nil, &RespError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)})
		}
	}
}

func (s *Server) handleInitialize(req Request) {
    version := internal.Version
    if s.llmClient != nil {
        version = version + " [" + s.llmClient.Name() + ":" + s.llmClient.DefaultModel() + "]"
    }
    res := InitializeResult{
        Capabilities: ServerCapabilities{
            TextDocumentSync: 1, // 1 = TextDocumentSyncKindFull
            CompletionProvider: &CompletionOptions{
                ResolveProvider: false,
                // TODO: Make the trigger characters configurable
                TriggerCharacters: []string{".", ":", "/", "_"},
            },
        },
        ServerInfo: &ServerInfo{Name: "hexai", Version: version},
    }
    s.reply(req.ID, res, nil)
}

func (s *Server) handleInitialized() {
    logging.Logf("lsp ", "client initialized")
}

func (s *Server) handleShutdown(req Request) {
	s.reply(req.ID, nil, nil)
}

func (s *Server) handleExit() {
	s.exited = true
	os.Exit(0)
}

func (s *Server) handleDidOpen(req Request) {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.setDocument(p.TextDocument.URI, p.TextDocument.Text)
		s.markActivity()
	}
}

func (s *Server) handleDidChange(req Request) {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		if len(p.ContentChanges) > 0 {
			s.setDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
		}
		s.markActivity()
	}
}

func (s *Server) handleDidClose(req Request) {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.deleteDocument(p.TextDocument.URI)
		s.markActivity()
	}
}

func (s *Server) handleCompletion(req Request) {
	var p CompletionParams
	var docStr string
    if err := json.Unmarshal(req.Params, &p); err == nil {
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

func (s *Server) reply(id json.RawMessage, result any, err *RespError) {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	s.writeMessage(resp)
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

    inParams := inParamList(current, p.Position.Character)
    sysPrompt, userPrompt := buildPrompts(inParams, p, above, current, below, funcCtx)
    messages := []llm.Message{
        {Role: "system", Content: sysPrompt},
        {Role: "user", Content: userPrompt},
    }
    if hasExtra && extraText != "" {
        messages = append(messages, llm.Message{Role: "user", Content: "Additional context:\n" + extraText})
    }

    text, err := s.llmClient.Chat(ctx, messages, llm.WithMaxTokens(s.maxTokens), llm.WithTemperature(0.2))
    if err != nil {
        logging.Logf("lsp ", "llm completion error: %v", err)
        return nil, false
    }
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return nil, false
	}

    te, filter := computeTextEditAndFilter(cleaned, inParams, current, p)
    rm := s.collectPromptRemovalEdits(p.TextDocument.URI)
    label := labelForCompletion(cleaned, filter)
    // Detail shows provider/model for visibility in client UI
    detail := "Hexai LLM completion"
    if s.llmClient != nil {
        detail = "Hexai " + s.llmClient.Name() + ":" + s.llmClient.DefaultModel()
    }
    items := []CompletionItem{{
        Label:            label,
        Kind:             1,
        Detail:           detail,
        InsertTextFormat: 1,
        FilterText:       strings.TrimLeft(filter, " \t"),
        TextEdit:         te,
        AdditionalTextEdits: rm,
        SortText:         "0000",
        Documentation:    docStr,
    }}
    return items, true
}

// collectPromptRemovalEdits returns edits to remove all inline prompt markers.
// Supported form (inclusive):
// - ";...;"   (optional single space after trailing ';')
// Multiple markers per line are supported.
func (s *Server) collectPromptRemovalEdits(uri string) []TextEdit {
    d := s.getDocument(uri)
    if d == nil || len(d.lines) == 0 {
        return nil
    }
    var edits []TextEdit
    for i, line := range d.lines {
        // Scan for ;...; markers
        startSemi := 0
        for startSemi < len(line) {
            j := strings.Index(line[startSemi:], ";")
            if j < 0 { break }
            j += startSemi
            k := strings.Index(line[j+1:], ";")
            if k < 0 { break }
            endChar := j + 1 + k + 1 // include trailing ';'
            if endChar < len(line) && line[endChar] == ' ' { endChar++ }
            edits = append(edits, TextEdit{Range: Range{Start: Position{Line: i, Character: j}, End: Position{Line: i, Character: endChar}}, NewText: ""})
            startSemi = endChar
        }
    }
    return edits
}

func inParamList(current string, cursor int) bool {
	if !strings.Contains(current, "func ") {
		return false
	}
	open := strings.Index(current, "(")
	close := strings.Index(current, ")")
	return open >= 0 && cursor > open && (close == -1 || cursor <= close)
}

func buildPrompts(inParams bool, p CompletionParams, above, current, below, funcCtx string) (string, string) {
	if inParams {
		sys := "You are a terse Go code completion engine for function signatures. Return only the parameter list contents (without parentheses), no braces, no prose. Prefer idiomatic names and types."
		user := fmt.Sprintf("Cursor is inside the function parameter list. Suggest only the parameter list (no parentheses).\nFunction line: %s\nCurrent line (cursor at %d): %s", funcCtx, p.Position.Character, current)
		return sys, user
	}
	sys := "You are a terse code completion engine. Return only the code to insert, no surrounding prose or backticks."
	user := fmt.Sprintf("Provide the next likely code to insert at the cursor.\nFile: %s\nFunction/context: %s\nAbove line: %s\nCurrent line (cursor at character %d): %s\nBelow line: %s\nOnly return the completion snippet.", p.TextDocument.URI, funcCtx, above, p.Position.Character, current, below)
	return sys, user
}

func computeTextEditAndFilter(cleaned string, inParams bool, current string, p CompletionParams) (*TextEdit, string) {
	if inParams {
		open := strings.Index(current, "(")
		close := strings.Index(current, ")")
		if open >= 0 {
			left := open + 1
			right := len(current)
			if close >= 0 && close >= left {
				right = close
			}
			if p.Position.Character < right {
				right = p.Position.Character
			}
			te := &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: left}, End: Position{Line: p.Position.Line, Character: right}}, NewText: cleaned}
			var filter string
			if left >= 0 && right >= left && right <= len(current) {
				filter = strings.TrimLeft(current[left:right], " \t")
			}
			return te, filter
		}
	}
	startChar := computeWordStart(current, p.Position.Character)
	te := &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: startChar}, End: Position{Line: p.Position.Line, Character: p.Position.Character}}, NewText: cleaned}
	filter := strings.TrimLeft(current[startChar:p.Position.Character], " \t")
	return te, filter
}

func computeWordStart(current string, at int) int {
	if at > len(current) {
		at = len(current)
	}
	for at > 0 {
		ch := current[at-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			at--
			continue
		}
		break
	}
	return at
}

func labelForCompletion(cleaned, filter string) string {
	label := trimLen(firstLine(cleaned))
	if filter != "" && !strings.HasPrefix(strings.ToLower(label), strings.ToLower(filter)) {
		return filter
	}
	return label
}

func (s *Server) fallbackCompletionItems(docStr string) []CompletionItem {
	return []CompletionItem{{
		Label:         "hexai-complete",
		Kind:          1,
		Detail:        "dummy completion",
		InsertText:    "hexai",
		SortText:      "9999",
		Documentation: docStr,
	}}
}
