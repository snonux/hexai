package lsp

import "testing"

func TestStripCodeFences_Table(t *testing.T) {
    cases := []struct{ name, in, want string }{
        {"no_fence", "return x", "return x"},
        {"plain_fence", "```\nA\nB\n```", "A\nB"},
        {"lang_fence", "```go\nfmt.Println()\n```", "fmt.Println()"},
        {"spaces", " \n```python\nprint('x')\n```\n ", "print('x')"},
    }
    for _, c := range cases {
        if got := stripCodeFences(c.in); got != c.want {
            t.Fatalf("%s: got %q want %q", c.name, got, c.want)
        }
    }
}

func TestStripInlineCodeSpan_Table(t *testing.T) {
    cases := []struct{ name, in, want string }{
        {"no_ticks", "text", "text"},
        {"single_span", "Use `foo()` here", "foo()"},
        {"multiple", "`a` + `b`", "a"},
        {"unmatched", "`missing end", "`missing end"},
    }
    for _, c := range cases {
        if got := stripInlineCodeSpan(c.in); got != c.want {
            t.Fatalf("%s: got %q want %q", c.name, got, c.want)
        }
    }
}

