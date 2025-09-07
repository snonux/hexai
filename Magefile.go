//go:build mage

// Hexai mage targets: build, dev, test, lint, install, etc.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var (
	Default                   = Build // Default target: build all binaries.
	coverageThreshold float64 = 85
	coveragePrinted           = make(chan struct{}, 1)
)

// Build builds the Hexai LSP and CLI binaries.
func Build() error {
	mg.Deps(BuildHexaiLSP, BuildHexaiCLI, BuildHexaiAction)
	printCoverage()
	return nil
}

// BuildHexaiLSP builds the LSP server binary.
func BuildHexaiLSP() error {
	printCoverage()
	return sh.RunV("go", "build", "-o", "hexai-lsp", "cmd/hexai-lsp/main.go")
}

// BuildHexaiCLI builds the CLI binary.
func BuildHexaiCLI() error {
	printCoverage()
	return sh.RunV("go", "build", "-o", "hexai", "cmd/hexai/main.go")
}

// BuildHexaiAction builds the hexai-action TUI binary.
func BuildHexaiAction() error {
    printCoverage()
    return sh.RunV("go", "build", "-o", "hexai-action", "cmd/hexai-action/main.go")
}

// Dev runs tests, vet, lint, then builds with race for both binaries.
func Dev() error {
	printCoverage()
	mg.Deps(Test, Vet, Lint)
	if err := sh.RunV("go", "build", "-race", "-o", "hexai-lsp", "cmd/hexai-lsp/main.go"); err != nil {
		return err
	}
	if err := sh.RunV("go", "build", "-race", "-o", "hexai", "cmd/hexai/main.go"); err != nil {
		return err
	}
    return sh.RunV("go", "build", "-race", "-o", "hexai-action", "cmd/hexai-action/main.go")
}

// Run launches the LSP server via go run (useful during development).
func Run() error {
	printCoverage()
	mg.Deps(Dev)
	return sh.RunV("go", "run", "cmd/hexai-lsp/main.go")
}

// RunCLI runs the CLI with a small test input.
func RunCLI() error {
	printCoverage()
	mg.Deps(Dev)
	cmd := "echo 'test' | go run cmd/hexai/main.go"
	return sh.RunV("bash", "-lc", cmd)
}

// Install copies built binaries to GOPATH/bin (defaults to ~/go/bin when GOPATH is unset).
func Install() error {
	printCoverage()
	mg.Deps(Build)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		gopath = filepath.Join(home, "go")
	}
	bin := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	if err := sh.RunV("cp", "-v", "./hexai-lsp", bin+"/"); err != nil {
		return err
	}
	if err := sh.RunV("cp", "-v", "./hexai", bin+"/"); err != nil {
		return err
	}
	return sh.RunV("cp", "-v", "./hexai-action", bin+"/")
}

// RunAction runs the hexai-action TUI via go run (reads stdin).
func RunAction() error {
    printCoverage()
    mg.Deps(Dev)
    return sh.RunV("go", "run", "cmd/hexai-action/main.go")
}

// printCoverage prints a warning if an existing coverage profile shows total < coverateThreshold.
func printCoverage() {
	// Ensure the top-level coverage profile is refreshed at least once per day.
	ensureDailyCoverage(24 * time.Hour)
	select {
	case coveragePrinted <- struct{}{}:
	default:
		// Coverage already printed
		return
	}

	profile := ""
	if _, err := os.Stat("docs/coverage.out"); err == nil {
		profile = "docs/coverage.out"
	}
	if profile == "" {
		fmt.Println("[coverage] No coverage profile found (run 'mage cover' or 'mage coverall').")
		return
	}
	pct, ok := totalCoveragePercent(profile)
	if !ok {
		fmt.Println("[coverage] Could not parse total coverage from", profile)
		return
	}
	if pct < coverageThreshold {
		fmt.Printf("[coverage] WARNING: total test coverage is %.1f%% (< %.1f%%)\n", pct, coverageThreshold)
	} else {
		fmt.Printf("[coverage] total test coverage: %.1f%% (>= %.1f%%)\n", pct, coverageThreshold)
	}
}

// ensureDailyCoverage regenerates the main coverage profile when it's missing
// or older than maxAge. It writes to docs/coverage.out via the Coverage target.
func ensureDailyCoverage(maxAge time.Duration) {
	const prof = "docs/coverage.out"
	st, err := os.Stat(prof)
	if err == nil {
		age := time.Since(st.ModTime())
		if age <= maxAge {
			return // fresh enough
		}
	}
	// Missing or stale; attempt to refresh. Do not hard-fail builds if coverage fails.
	if err := Coverage(); err != nil {
		fmt.Println("[coverage] refresh skipped due to error:", err)
	}
}

// totalCoveragePercent returns the parsed total percentage from a coverage profile using `go tool cover -func`.
func totalCoveragePercent(profile string) (float64, bool) {
	out, err := sh.Output("go", "tool", "cover", "-func="+profile)
	if err != nil {
		return 0, false
	}
	// Find a line like: "total:\t(statements)\t75.3%"
	re := regexp.MustCompile(`(?m)^total:\s*\(statements\)\s*([0-9]+\.[0-9]+|[0-9]+)%\s*$`)
	m := re.FindStringSubmatch(out)
	if len(m) != 2 {
		return 0, false
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Test runs the test suite.
func Test() error {
	if err := sh.RunV("go", "clean", "-testcache"); err != nil {
		return err
	}
	return sh.RunV("go", "test", "-v", "./...")
}

// Coverage generates a combined coverage profile across all packages (cross-package coverage).
// Instruments all packages during each test run using -coverpkg=./... so that
// coverage collected from one package's tests include code executed in others.
func Coverage() error {
	const prof = "docs/coverage.out"
	const html = "docs/coverage.html"
	_ = os.Remove(prof)
	_ = os.Remove(html)
	if err := sh.RunV("go", "clean", "-testcache"); err != nil {
		return err
	}
	if err := sh.RunV("go", "test", "-covermode=atomic", "-coverpkg=./...", "-coverprofile="+prof, "./..."); err != nil {
		return err
	}
	if out, err := sh.Output("go", "tool", "cover", "-func="+prof); err == nil {
		fmt.Print(out)
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "total:") {
				fmt.Println("\nTotal coverage (cross-package):", strings.TrimSpace(lines[i]))
				break
			}
		}
	} else {
		return err
	}
	if err := sh.RunV("go", "tool", "cover", "-html="+prof, "-o", html); err != nil {
		return err
	}
	fmt.Println("HTML coverage report written to " + html + " (cross-package)")
	return nil
}

// Vet runs go vet.
func Vet() error {
	return sh.RunV("go", "vet", "./...")
}

// Lint runs golangci-lint.
func Lint() error {
	return sh.RunV("golangci-lint", "run")
}

// DevInstall installs helpful developer tools.
func DevInstall() error {
	if err := sh.RunV("go", "install", "golang.org/x/tools/gopls@latest"); err != nil {
		return err
	}
	return sh.RunV("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
}

// CoverCheck enforces minimum per-package coverage.
// Exceptions: any package whose import path contains "/cmd/" and any substring
// provided via HEXAI_COVER_EXCEPT (comma-separated).
func CoverCheck() error {
	except := []string{"/cmd/"}
	if v := strings.TrimSpace(os.Getenv("HEXAI_COVER_EXCEPT")); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				except = append(except, s)
			}
		}
	}
	list, err := sh.Output("go", "list", "./...")
	if err != nil {
		return err
	}
	pkgs := strings.Split(strings.TrimSpace(list), "\n")
	mod := modulePathGuess()
	_ = os.MkdirAll("docs/coverage", 0o755)
	type res struct {
		pkg   string
		total float64
	}
	var all, bad []res
	for _, pkg := range pkgs {
		if pkg == "" {
			continue
		}
		skip := false
		for _, ex := range except {
			if strings.Contains(pkg, ex) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		safe := strings.ReplaceAll(strings.TrimPrefix(pkg, mod), "/", "_")
		if safe == pkg {
			if i := strings.LastIndex(pkg, "/"); i >= 0 {
				safe = pkg[i+1:]
			}
		}
		prof := filepath.Join("docs", "coverage", safe+".out")
		// Per-package run; ignore errors to allow packages without tests
		_, _ = sh.Output("go", "test", "-covermode=count", "-coverprofile", prof, pkg)
		// Read total
		total, ok := totalCoveragePercent(prof)
		if !ok {
			total = 0
		}
		all = append(all, res{pkg, total})
		if total < coverageThreshold {
			bad = append(bad, res{pkg, total})
		}
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Printf("Per-package coverage (threshold %.1f%%)\n", coverageThreshold)
	for _, r := range all {
		fmt.Printf("- %s: %.1f%%\n", r.pkg, r.total)
	}
	if len(bad) > 0 {
		fmt.Println("\nPackages below threshold:")
		for _, r := range bad {
			fmt.Printf("- %s: %.1f%%\n", r.pkg, r.total)
		}
		return fmt.Errorf("coverage check failed (%d package(s) < %.1f%%)", len(bad), coverageThreshold)
	}
	fmt.Println("All packages meet coverage threshold.")
	return nil
}

func modulePathGuess() string {
	if out, err := sh.Output("go", "list", "-m"); err == nil {
		return strings.TrimSpace(out)
	}
	return ""
}
