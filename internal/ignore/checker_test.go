package ignore

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeGitignore creates a .gitignore in dir with the given lines.
func writeGitignore(t *testing.T, dir string, lines ...string) {
	t.Helper()
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}

func TestSimpleWildcard(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "app.log")); !ign {
		t.Error("expected app.log to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "debug.log")); !ign {
		t.Error("expected debug.log to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "app.go")); ign {
		t.Error("app.go should not be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "log.txt")); ign {
		t.Error("log.txt should not be ignored")
	}
}

func TestDirectoryPattern(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "build/")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "build", "output.js")); !ign {
		t.Error("expected build/output.js to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "rebuild", "x")); ign {
		t.Error("rebuild/x should not be ignored")
	}
}

func TestDoubleStarPattern(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "**/temp")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "a", "b", "temp")); !ign {
		t.Error("expected a/b/temp to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "temp")); !ign {
		t.Error("expected temp to be ignored")
	}
}

func TestNegation(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log", "!important.log")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "debug.log")); !ign {
		t.Error("expected debug.log to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "important.log")); ign {
		t.Error("important.log should not be ignored (negated)")
	}
}

func TestComments(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "# comment", "*.tmp")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "x.tmp")); !ign {
		t.Error("expected x.tmp to be ignored")
	}
	// A file literally named "# comment" should not be ignored
	if ign, _ := c.IsIgnored(filepath.Join(dir, "# comment")); ign {
		t.Error("file named '# comment' should not be ignored")
	}
}

func TestExtensionGroups(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.out", "*.html")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "coverage.out")); !ign {
		t.Error("expected coverage.out to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "main.go")); ign {
		t.Error("main.go should not be ignored")
	}
}

func TestNestedDirs(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "vendor/**")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "vendor", "lib", "x.go")); !ign {
		t.Error("expected vendor/lib/x.go to be ignored")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "myvendor", "x")); ign {
		t.Error("myvendor/x should not be ignored")
	}
}

func TestExtraPatternsOnly(t *testing.T) {
	// No gitignore, only extra patterns
	c := New("", false, []string{"*.min.js", "dist/**"})

	if ign, reason := c.IsIgnored("/project/app.min.js"); !ign {
		t.Error("expected app.min.js to be ignored")
	} else if reason != "matched extra ignore pattern" {
		t.Errorf("unexpected reason: %s", reason)
	}
	if ign, _ := c.IsIgnored("/project/dist/bundle.js"); !ign {
		t.Error("expected dist/bundle.js to be ignored")
	}
	if ign, _ := c.IsIgnored("/project/app.js"); ign {
		t.Error("app.js should not be ignored")
	}
}

func TestCombinedGitignoreAndExtra(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log")
	c := New(dir, true, []string{"*.min.js"})

	// gitignore match
	if ign, reason := c.IsIgnored(filepath.Join(dir, "app.log")); !ign {
		t.Error("expected app.log to be ignored")
	} else if reason != "matched .gitignore pattern" {
		t.Errorf("unexpected reason: %s", reason)
	}
	// extra pattern match
	if ign, reason := c.IsIgnored(filepath.Join(dir, "app.min.js")); !ign {
		t.Error("expected app.min.js to be ignored")
	} else if reason != "matched extra ignore pattern" {
		t.Errorf("unexpected reason: %s", reason)
	}
	// neither match
	if ign, _ := c.IsIgnored(filepath.Join(dir, "main.go")); ign {
		t.Error("main.go should not be ignored")
	}
}

func TestNilChecker(t *testing.T) {
	var c *Checker
	if ign, _ := c.IsIgnored("/some/file.go"); ign {
		t.Error("nil checker should never ignore")
	}
}

func TestEmptyChecker(t *testing.T) {
	c := New("", false, nil)
	if ign, _ := c.IsIgnored("/some/file.go"); ign {
		t.Error("empty checker should never ignore")
	}
}

func TestUpdatePatterns(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log")
	c := New(dir, true, nil)

	if ign, _ := c.IsIgnored(filepath.Join(dir, "app.log")); !ign {
		t.Error("expected app.log ignored initially")
	}

	// Update: disable gitignore, add extra pattern
	c.Update(false, []string{"*.tmp"})

	if ign, _ := c.IsIgnored(filepath.Join(dir, "app.log")); ign {
		t.Error("app.log should not be ignored after disabling gitignore")
	}
	if ign, _ := c.IsIgnored(filepath.Join(dir, "x.tmp")); !ign {
		t.Error("expected x.tmp ignored after update")
	}
}

func TestThreadSafety(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log")
	c := New(dir, true, nil)

	var wg sync.WaitGroup
	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.IsIgnored(filepath.Join(dir, "app.log"))
			c.IsIgnored(filepath.Join(dir, "main.go"))
		}()
	}
	// Concurrent updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Update(true, []string{"*.tmp"})
		}()
	}
	wg.Wait()
}

func TestNoGitRoot(t *testing.T) {
	// gitRoot empty but gitignore enabled — should not crash, gitignore has no effect
	c := New("", true, []string{"*.bak"})

	if ign, _ := c.IsIgnored("/any/file.go"); ign {
		t.Error("should not ignore .go files")
	}
	if ign, _ := c.IsIgnored("/any/file.bak"); !ign {
		t.Error("extra patterns should still work without git root")
	}
}

func TestPathOutsideGitRoot(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir, "*.log")
	c := New(dir, true, nil)

	// Path outside the git root — relPath returns absolute, gitignore won't match
	if ign, _ := c.IsIgnored("/completely/elsewhere/app.log"); ign {
		t.Error("files outside git root should not be matched by gitignore")
	}
}

func TestMixedRealGitignore(t *testing.T) {
	dir := t.TempDir()
	writeGitignore(t, dir,
		"# Build outputs",
		"bin/",
		"*.exe",
		"*.dll",
		"",
		"# Dependencies",
		"vendor/**",
		"",
		"# IDE",
		".idea/",
		".vscode/",
	)
	c := New(dir, true, nil)

	ignored := []string{
		filepath.Join(dir, "bin", "app"),
		filepath.Join(dir, "main.exe"),
		filepath.Join(dir, "vendor", "lib", "x.go"),
		filepath.Join(dir, ".idea", "workspace.xml"),
	}
	for _, p := range ignored {
		if ign, _ := c.IsIgnored(p); !ign {
			t.Errorf("expected %s to be ignored", p)
		}
	}

	allowed := []string{
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "internal", "app.go"),
		filepath.Join(dir, "README.md"),
	}
	for _, p := range allowed {
		if ign, _ := c.IsIgnored(p); ign {
			t.Errorf("%s should not be ignored", p)
		}
	}
}
