//go:build mage

// Hexai mage targets: build, dev, test, lint, install, etc.
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "strconv"
    "regexp"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target: build both binaries.
var Default = Build

// Build builds the Hexai LSP and CLI binaries.
func Build() error {
    mg.Deps(BuildHexaiLSP, BuildHexaiCLI)
    warnIfLowCoverage(80.0)
    return nil
}

// BuildHexaiLSP builds the LSP server binary.
func BuildHexaiLSP() error {
	return sh.RunV("go", "build", "-o", "hexai-lsp", "cmd/hexai-lsp/main.go")
}

// BuildHexaiCLI builds the CLI binary.
func BuildHexaiCLI() error {
	return sh.RunV("go", "build", "-o", "hexai", "cmd/hexai/main.go")
}

// Dev runs tests, vet, lint, then builds with race for both binaries.
func Dev() error {
	mg.Deps(Test, Vet, Lint)
	if err := sh.RunV("go", "build", "-race", "-o", "hexai-lsp", "cmd/hexai-lsp/main.go"); err != nil {
		return err
	}
	return sh.RunV("go", "build", "-race", "-o", "hexai", "cmd/hexai/main.go")
}

// Run launches the LSP server via go run (useful during development).
func Run() error {
	mg.Deps(Dev)
	return sh.RunV("go", "run", "cmd/hexai-lsp/main.go")
}

// RunCLI runs the CLI with a small test input.
func RunCLI() error {
	mg.Deps(Dev)
	cmd := "echo 'test' | go run cmd/hexai/main.go"
	return sh.RunV("bash", "-lc", cmd)
}

// Install copies built binaries to GOPATH/bin (defaults to ~/go/bin when GOPATH is unset).
func Install() error {
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
    return sh.RunV("cp", "-v", "./hexai", bin+"/")
}

// warnIfLowCoverage prints a warning if an existing coverage profile shows total < threshold.
// It prefers docs/coverall.out, then docs/cover.out. It does not run tests.
func warnIfLowCoverage(threshold float64) {
    profile := ""
    if _, err := os.Stat("docs/coverall.out"); err == nil {
        profile = "docs/coverall.out"
    } else if _, err := os.Stat("docs/cover.out"); err == nil {
        profile = "docs/cover.out"
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
    if pct < threshold {
        fmt.Printf("WARNING: total test coverage is %.1f%% (< %.1f%%)\n", pct, threshold)
    } else {
        fmt.Printf("[coverage] total test coverage: %.1f%% (>= %.1f%%)\n", pct, threshold)
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

// Cover generates a unit test coverage report.
// - Writes coverage data to coverage.out
// - Prints function coverage summary to stdout
// - Writes HTML report to coverage.html
func Cover() error {
    // Ensure a clean slate
    const prof = "docs/cover.out"
    const html = "docs/cover.html"
    _ = os.Remove(prof)
    _ = os.Remove(html)
	if err := sh.RunV("go", "clean", "-testcache"); err != nil {
		return err
	}
    if err := sh.RunV("go", "test", "-covermode=atomic", "-coverprofile="+prof, "./..."); err != nil {
        return err
    }
    // Print function-by-function coverage summary
    if out, err := sh.Output("go", "tool", "cover", "-func="+prof); err == nil {
        fmt.Print(out)
        lines := strings.Split(strings.TrimSpace(out), "\n")
        for i := len(lines) - 1; i >= 0; i-- {
            if strings.HasPrefix(strings.TrimSpace(lines[i]), "total:") {
                fmt.Println("\nTotal coverage:", strings.TrimSpace(lines[i]))
                break
            }
        }
    } else {
        return err
    }
    // Generate an HTML report for browsers/editors
    if err := sh.RunV("go", "tool", "cover", "-html="+prof, "-o", html); err != nil {
        return err
    }
    fmt.Println("HTML coverage report written to "+html)
    return nil
}

// CoverAll generates a combined coverage profile across all packages (cross-package coverage).
// Instruments all packages during each test run using -coverpkg=./... so that
// coverage collected from one package's tests include code executed in others.
func CoverAll() error {
    const prof = "docs/coverall.out"
    const html = "docs/coverall.html"
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
    fmt.Println("HTML coverage report written to "+html+" (cross-package)")
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
