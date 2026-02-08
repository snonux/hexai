package tmuxedit

import (
	"fmt"
	"strings"
	"testing"
)

func TestCursorAgent_ExtractPrompt(t *testing.T) {
	agent := newCursorAgent()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "box with arrow",
			content: "Cursor Agent\n │ → fix the bug                    INSERT │",
			want:    "fix the bug",
		},
		{
			name:    "box without arrow",
			content: "Cursor Agent\n │ fix the bug              │",
			want:    "fix the bug",
		},
		{
			name:    "strips follow-up placeholder",
			content: "Cursor\n │ → Add a follow-up            │",
			want:    "",
		},
		{
			name:    "multi-line prompt",
			content: " │ → first line of prompt                    │\n │   second line here                        │\n │   third line end                          │",
			want:    "first line of prompt\nsecond line here\nthird line end",
		},
		{
			name:    "multi-line with noise",
			content: " │ → fix the bug INSERT                     │\n │   also refactor tests                     │",
			want:    "fix the bug\nalso refactor tests",
		},
		{
			name: "multi-box takes last box only",
			content: " ┌──────────────┐\n" +
				" │ $ git push    │\n" +
				" └──────────────┘\n" +
				" ┌──────────────┐\n" +
				" │ Run command?  │\n" +
				" │ → Yes (enter) │\n" +
				" │   No (esc)    │\n" +
				" └──────────────┘\n" +
				" ┌──────────────┐\n" +
				" │ → hello world │\n" +
				" └──────────────┘\n",
			want: "hello world",
		},
		{
			name: "multi-box multi-line prompt",
			content: " ┌──────────────┐\n" +
				" │ $ git push    │\n" +
				" └──────────────┘\n" +
				" ┌──────────────┐\n" +
				" │ → first line  │\n" +
				" │   second line │\n" +
				" │   third line  │\n" +
				" └──────────────┘\n",
			want: "first line\nsecond line\nthird line",
		},
		{
			name:    "no match",
			content: "no prompt here",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agent.ExtractPrompt(tt.content)
			if got != tt.want {
				t.Errorf("ExtractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCursorAgent_ClearInput(t *testing.T) {
	noSleep(t)
	var calls []string
	oldSend := sendKeys
	oldRepeat := sendRepeatedKey
	defer func() {
		sendKeys = oldSend
		sendRepeatedKey = oldRepeat
	}()
	sendKeys = func(paneID string, keys ...string) error {
		calls = append(calls, fmt.Sprintf("send:%s:%s", paneID, strings.Join(keys, ",")))
		return nil
	}
	sendRepeatedKey = func(paneID, key string, count int) error {
		calls = append(calls, fmt.Sprintf("repeat:%s:%s*%d", paneID, key, count))
		return nil
	}

	agent := newCursorAgent()
	err := agent.ClearInput("%5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "End BSpace*200" should send End normally, then BSpace 200 times via -N
	want := []string{
		"send:%5:End",
		"repeat:%5:BSpace*200",
	}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestCursorAgent_Detect(t *testing.T) {
	agent := newCursorAgent()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"box with arrow", "│ → type here │", true},
		{"commands footer", "/ commands · @ files", true},
		{"no match", "some text", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agent.Detect(tt.content); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}
