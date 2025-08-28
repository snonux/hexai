//go:build mage

// Hexai mage targets: build, dev, test, lint, install, etc.
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/magefile/mage/mg"
    "github.com/magefile/mage/sh"
)

// Default target: build both binaries.
var Default = Build

// Build builds the Hexai LSP and CLI binaries.
func Build() error {
    mg.Deps(BuildHexaiLSP, BuildHexaiCLI)
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

// Test runs the test suite.
func Test() error {
    if err := sh.RunV("go", "clean", "-testcache"); err != nil {
        return err
    }
    return sh.RunV("go", "test", "-v", "./...")
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

