package askcli

import (
	"fmt"
	"strings"
)

// completionContextSpec is the single source of truth for a Fish completion
// context condition (the __ask_in_*_context functions). Both the Go evaluator
// (used by the fish*CompletionContext helpers and their unit tests) and the
// Fish function emitter consume the same spec, so the generated shell condition
// functions cannot diverge from the tested Go logic. A spec is an AND of
// top-level matchers; an anyOf matcher expresses an OR of AND-groups.
type completionContextSpec struct {
	name         string
	needsCurrent bool
	matches      []ctxMatcher
}

// ctxMatcher is a condition a generated Fish context function checks.
// emitFishGuard writes Fish lines that return 1 (fail) when the matcher fails
// and fall through (no return) when it succeeds, assuming `positional` (and
// `current` when the spec sets needsCurrent) is already set in the function.
type ctxMatcher interface {
	matchGo(positional []string, current string, names []string) bool
	emitFishGuard(b *strings.Builder, names []string)
}

// groupMatcher is a ctxMatcher that can also render as one or more Fish
// condition lines (true on success) for use inside anyOf group `if` blocks.
type groupMatcher interface {
	ctxMatcher
	fishConditionLines(names []string) []string
}

// evalCompletionContext evaluates the spec against positionals (scope prefixes
// already stripped) and the current token. names is the set referenced by a
// cmdIn matcher that uses the singleSelector set (commandRegistry.singleSelectorNames()).
func evalCompletionContext(spec completionContextSpec, positional []string, current string, names []string) bool {
	for _, m := range spec.matches {
		if !m.matchGo(positional, current, names) {
			return false
		}
	}
	return true
}

// The three completion-context specs below are the single source of truth shared
// by the fish*CompletionContext Go helpers (and their tests) and the generated
// Fish __ask_in_*_context functions. The uuid context offers a task selector
// for single-selector commands; the dep-uuid context offers selectors for the
// dep add/rm/list subcommands; the add-dep-modifier context offers the
// depends:/depends:<alias> dependency modifier while adding a task.
var (
	uuidContextSpec = completionContextSpec{
		name: "__ask_in_uuid_context",
		matches: []ctxMatcher{
			lenIs{n: 1},
			cmdIn{idx: 0, selectorSet: true},
		},
	}

	depUUIDContextSpec = completionContextSpec{
		name: "__ask_in_dep_uuid_context",
		matches: []ctxMatcher{
			cmdEq{idx: 0, val: "dep"},
			// count >= 2 guard so $positional[2] access in the anyOf groups is bounded.
			lenBetween{lo: 2, hi: -1},
			anyOf{groups: [][]groupMatcher{
				{cmdIn{idx: 1, vals: []string{"add", "rm"}}, lenBetween{lo: 2, hi: 3}},
				{cmdEq{idx: 1, val: "list"}, lenIs{n: 2}},
			}},
		},
	}

	addDepModifierContextSpec = completionContextSpec{
		name:         "__ask_in_add_dep_modifier_context",
		needsCurrent: true,
		matches: []ctxMatcher{
			cmdEq{idx: 0, val: "add"},
			anyOf{groups: [][]groupMatcher{
				{currentIs{val: "depends"}},
				{currentPrefix{prefix: "depends:"}},
			}},
		},
	}
)

// completionContextSpecs is the ordered list of context specs emitted into the
// Fish completion script.
var completionContextSpecs = []completionContextSpec{
	uuidContextSpec, depUUIDContextSpec, addDepModifierContextSpec,
}

// --- simple matchers ---

// cmdEq matches when positional[idx] equals val.
type cmdEq struct {
	idx int
	val string
}

func (m cmdEq) matchGo(p []string, _ string, _ []string) bool {
	return m.idx < len(p) && p[m.idx] == m.val
}

func (m cmdEq) emitFishGuard(b *strings.Builder, _ []string) {
	fmt.Fprintf(b, "    if test (count $positional) -le %d\n        return 1\n    end\n", m.idx)
	fmt.Fprintf(b, "    if test $positional[%d] != %s\n        return 1\n    end\n", m.idx+1, fishSingleQuote(m.val))
}

func (m cmdEq) fishConditionLines(_ []string) []string {
	return []string{
		fmt.Sprintf("test (count $positional) -gt %d", m.idx),
		fmt.Sprintf("test $positional[%d] = %s", m.idx+1, fishSingleQuote(m.val)),
	}
}

// cmdIn matches when positional[idx] is a member of vals, or of the
// singleSelector set when selectorSet is true.
type cmdIn struct {
	idx         int
	vals        []string
	selectorSet bool
}

// cmdInValues resolves the value set a cmdIn matches against: the registry's
// singleSelector names when selectorSet is true, otherwise the literal vals.
func cmdInValues(m cmdIn, names []string) []string {
	if m.selectorSet {
		return names
	}
	return m.vals
}

func (m cmdIn) matchGo(p []string, _ string, names []string) bool {
	if m.idx >= len(p) {
		return false
	}
	for _, v := range cmdInValues(m, names) {
		if p[m.idx] == v {
			return true
		}
	}
	return false
}

func (m cmdIn) emitFishGuard(b *strings.Builder, names []string) {
	fmt.Fprintf(b, "    if test (count $positional) -le %d\n        return 1\n    end\n", m.idx)
	fmt.Fprintf(b, "    if not contains -- $positional[%d] %s\n        return 1\n    end\n",
		m.idx+1, strings.Join(cmdInValues(m, names), " "))
}

func (m cmdIn) fishConditionLines(names []string) []string {
	return []string{
		fmt.Sprintf("test (count $positional) -gt %d", m.idx),
		fmt.Sprintf("contains -- $positional[%d] %s", m.idx+1, strings.Join(cmdInValues(m, names), " ")),
	}
}

// lenIs matches when len(positional) == n.
type lenIs struct{ n int }

func (m lenIs) matchGo(p []string, _ string, _ []string) bool { return len(p) == m.n }

func (m lenIs) emitFishGuard(b *strings.Builder, _ []string) {
	fmt.Fprintf(b, "    if test (count $positional) -ne %d\n        return 1\n    end\n", m.n)
}

func (m lenIs) fishConditionLines(_ []string) []string {
	return []string{fmt.Sprintf("test (count $positional) -eq %d", m.n)}
}

// lenBetween matches when lo <= len(positional) <= hi. hi < 0 means no upper
// bound (used as a safe count guard so later index access is bounded).
type lenBetween struct{ lo, hi int }

func (m lenBetween) matchGo(p []string, _ string, _ []string) bool {
	if len(p) < m.lo {
		return false
	}
	return m.hi < 0 || len(p) <= m.hi
}

func (m lenBetween) emitFishGuard(b *strings.Builder, _ []string) {
	fmt.Fprintf(b, "    if test (count $positional) -lt %d\n        return 1\n    end\n", m.lo)
	if m.hi >= 0 {
		fmt.Fprintf(b, "    if test (count $positional) -gt %d\n        return 1\n    end\n", m.hi)
	}
}

func (m lenBetween) fishConditionLines(_ []string) []string {
	lines := []string{fmt.Sprintf("test (count $positional) -ge %d", m.lo)}
	if m.hi >= 0 {
		lines = append(lines, fmt.Sprintf("test (count $positional) -le %d", m.hi))
	}
	return lines
}

// currentIs matches when current equals val.
type currentIs struct{ val string }

func (m currentIs) matchGo(_ []string, current string, _ []string) bool { return current == m.val }

func (m currentIs) emitFishGuard(b *strings.Builder, _ []string) {
	fmt.Fprintf(b, "    if test \"$current\" != %s\n        return 1\n    end\n", fishSingleQuote(m.val))
}

func (m currentIs) fishConditionLines(_ []string) []string {
	return []string{fmt.Sprintf("test \"$current\" = %s", fishSingleQuote(m.val))}
}

// currentPrefix matches when current has the given literal prefix.
type currentPrefix struct{ prefix string }

func (m currentPrefix) matchGo(_ []string, current string, _ []string) bool {
	return strings.HasPrefix(current, m.prefix)
}

func (m currentPrefix) emitFishGuard(b *strings.Builder, _ []string) {
	fmt.Fprintf(b, "    if not string match -qr %s -- $current\n        return 1\n    end\n",
		fishSingleQuote("^"+fishRegexEscape(m.prefix)))
}

func (m currentPrefix) fishConditionLines(_ []string) []string {
	return []string{fmt.Sprintf("string match -qr %s -- $current", fishSingleQuote("^"+fishRegexEscape(m.prefix)))}
}

// --- anyOf matcher (OR of AND-groups); only used as a top-level matcher ---

type anyOf struct{ groups [][]groupMatcher }

func (m anyOf) matchGo(p []string, current string, names []string) bool {
	for _, g := range m.groups {
		if groupMatches(g, p, current, names) {
			return true
		}
	}
	return false
}

func groupMatches(g []groupMatcher, p []string, current string, names []string) bool {
	for _, t := range g {
		if !t.matchGo(p, current, names) {
			return false
		}
	}
	return true
}

// emitFishGuard emits the proven flag pattern: try each group as an `and`-chain
// `if`; set a flag on the first group that fully matches; return 1 if no group
// matched. This avoids fish `or` precedence pitfalls (see anyOf fish tests).
func (m anyOf) emitFishGuard(b *strings.Builder, names []string) {
	b.WriteString("    set -l __ask_ctx_matched 0\n")
	for _, g := range m.groups {
		b.WriteString("    if test $__ask_ctx_matched -eq 0\n")
		writeAnyOfGroupIf(b, g, names)
		b.WriteString("    end\n")
	}
	b.WriteString("    if test $__ask_ctx_matched -eq 0\n        return 1\n    end\n")
}

// writeAnyOfGroupIf writes the inner `if <conditions>; set __ask_ctx_matched 1;
// end` for one OR-group, with `and` continuations across the group's conditions.
func writeAnyOfGroupIf(b *strings.Builder, g []groupMatcher, names []string) {
	var lines []string
	for _, t := range g {
		lines = append(lines, t.fishConditionLines(names)...)
	}
	if len(lines) == 0 {
		b.WriteString("            set __ask_ctx_matched 1\n")
		return
	}
	fmt.Fprintf(b, "        if %s\n", lines[0])
	for _, l := range lines[1:] {
		fmt.Fprintf(b, "            and %s\n", l)
	}
	b.WriteString("            set __ask_ctx_matched 1\n        end\n")
}

// --- emission helpers ---

// writeFishContextFunctionFromSpec emits the Fish condition function for spec.
// names is the singleSelector set referenced by cmdIn matchers.
func writeFishContextFunctionFromSpec(b *strings.Builder, spec completionContextSpec, names []string) {
	fmt.Fprintf(b, "function %s\n", spec.name)
	b.WriteString("    set -l positional (__ask_command_positionals)\n")
	if spec.needsCurrent {
		// Mirror the Go helper's strings.TrimSpace(current) so both sides agree
		// on surrounding whitespace. commandline -ct is already a token, so the
		// trim is a harmless no-op in practice but keeps the two implementations
		// from diverging on edge inputs.
		b.WriteString("    set -l current (string trim -- (commandline -ct))\n")
	}
	for _, m := range spec.matches {
		m.emitFishGuard(b, names)
	}
	b.WriteString("    return 0\nend\n\n")
}

// fishSingleQuote wraps value in single quotes for Fish, escaping embedded
// single quotes via the end-quote/\'/reopen-quote sequence.
func fishSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// fishRegexEscape escapes regex metacharacters in a literal prefix so that the
// generated `string match -qr '^<prefix>'` treats the prefix as a literal.
func fishRegexEscape(s string) string {
	const metachars = `\.[]{}()*+?^$|`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(metachars, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
