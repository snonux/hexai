package lsp

import "testing"

func TestPrefixStripping_Table(t *testing.T) {
	cases := []struct{ name, prefix, sugg, want string }{
		{"assign_walrus", "name := ", "name := compute()", "compute()"},
		{"assign_equals", "x = ", "x = y+1", "y+1"},
		{"general_db", "db.", "db.Query()", "Query()"},
		{"general_func", "func New ", "func New() *T", "() *T"},
	}
	for _, c := range cases {
		var got string
		if c.name == "assign_walrus" || c.name == "assign_equals" {
			got = stripDuplicateAssignmentPrefix(c.prefix, c.sugg)
		} else {
			got = stripDuplicateGeneralPrefix(c.prefix, c.sugg)
		}
		if got != c.want {
			t.Fatalf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
