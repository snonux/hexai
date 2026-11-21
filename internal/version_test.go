package internal

import (
	"regexp"
	"testing"
)

// semverRe accepts plain x.y.z versions without leading zeros, which is the
// format AGENTS.md prescribes for releases and what tags are cut from.
var semverRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func TestVersionIsSemver(t *testing.T) {
	if !semverRe.MatchString(Version) {
		t.Fatalf("Version = %q, want a plain x.y.z semantic version", Version)
	}
}

// Negative case: the pattern must reject the malformed shapes it guards against.
func TestSemverPatternRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "1.2", "v1.2.3", "1.2.3-rc1", "01.2.3", "1.2.3.4", " 1.2.3"} {
		if semverRe.MatchString(bad) {
			t.Errorf("pattern accepted malformed version %q", bad)
		}
	}
}
