package hexaiaction

import (
    "bytes"
    "regexp"
    "testing"

    "github.com/charmbracelet/bubbles/list"
)

func stripANSI(s string) string {
    re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    return re.ReplaceAllString(s, "")
}

func TestOneLineDelegate_Render(t *testing.T) {
    items := []list.Item{item{title: "Rewrite selection", kind: ActionRewrite, hotkey: 'r'}}
    m := list.New(items, oneLineDelegate{}, 0, 0)
    m.Select(0)
    var b bytes.Buffer
    oneLineDelegate{}.Render(&b, m, 0, items[0])
    out := stripANSI(b.String())
    if !regexp.MustCompile(`> \w`).MatchString(out) {
        t.Fatalf("expected cursor prefix in %q", out)
    }
    if !regexp.MustCompile(`Rewrite selection`).MatchString(out) {
        t.Fatalf("expected title in %q", out)
    }
    if !regexp.MustCompile(`\(r\)`).MatchString(out) {
        t.Fatalf("expected hotkey in %q", out)
    }
}
