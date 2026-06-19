// ExecuteCommand handler to support post-edit navigation (jump to generated test).

package lsp

import (
	"encoding/json"
)

func (s *Server) handleExecuteCommand(req Request) {
	var p ExecuteCommandParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.reply(req.ID, nil, nil)
		return
	}
	switch p.Command {
	case "hexai.showDocument":
		if len(p.Arguments) >= 2 {
			uri, _ := p.Arguments[0].(string)
			var r Range
			// Convert second arg to Range via re-marshal to be robust across clients
			if b, err := json.Marshal(p.Arguments[1]); err == nil {
				_ = json.Unmarshal(b, &r)
			}
			if uri != "" {
				s.clientShowDocument(uri, &r)
			}
		}
		s.reply(req.ID, nil, nil)
		return
	default:
		// Unknown command; no-op
		s.reply(req.ID, nil, nil)
		return
	}
}
