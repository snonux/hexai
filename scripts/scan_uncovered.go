// Command scan_uncovered analyzes Go source to find functions without tests
// and recommends whether to add tests based on export status and basic complexity.
package main

import (
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "os"
    "path/filepath"
    "sort"
    "strings"
)

type FuncInfo struct {
    File    string
    Package string
    Name    string
    Recv    string // receiver type for methods
    Exported bool
    Complexity int // naive: number of statements in body
    HasControl bool // has if/for/switch/select
}

func main() {
    root := "."
    dirs := map[string][]string{}
    _ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
        if err != nil { return nil }
        // Skip vendor, tmp, .git, node_modules, build caches
        base := filepath.Base(path)
        if d.IsDir() {
            if (path != "." && strings.HasPrefix(base, ".")) || base == "vendor" || base == "tmp" || base == "node_modules" || base == ".gocache" || base == ".gomodcache" {
                return filepath.SkipDir
            }
            return nil
        }
        if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
            dir := filepath.Dir(path)
            dirs[dir] = append(dirs[dir], path)
        }
        return nil
    })

    fset := token.NewFileSet()
    type missing struct{
        pkg string
        dir string
        items []FuncInfo
    }
    var all []missing

    for dir, files := range dirs {
        // parse package name from any file
        pkg := ""
        var funcs []FuncInfo
        for _, file := range files {
            f, err := parser.ParseFile(fset, file, nil, 0)
            if err != nil { continue }
            if pkg == "" { pkg = f.Name.Name }
            for _, d := range f.Decls {
                fd, ok := d.(*ast.FuncDecl)
                if !ok || fd.Name == nil { continue }
                // Skip init/test helpers in non-test files? keep all funcs
                info := FuncInfo{File: file, Package: pkg, Name: fd.Name.Name, Exported: ast.IsExported(fd.Name.Name)}
                if fd.Recv != nil && len(fd.Recv.List) > 0 {
                    info.Recv = typeString(fd.Recv.List[0].Type)
                }
                if fd.Body != nil {
                    info.Complexity = len(fd.Body.List)
                    ast.Inspect(fd.Body, func(n ast.Node) bool {
                        switch n.(type) {
                        case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt:
                            info.HasControl = true
                        }
                        return true
                    })
                }
                funcs = append(funcs, info)
            }
        }
        if len(funcs) == 0 { continue }
        // Gather test identifiers and test names in this dir
        testIdents := make(map[string]struct{})
        testNames := make(map[string]struct{})
        _ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
            if err != nil { return nil }
            if d.IsDir() { return nil }
            if !strings.HasSuffix(p, "_test.go") { return nil }
            tf, err := parser.ParseFile(fset, p, nil, 0)
            if err != nil { return nil }
            for _, d := range tf.Decls {
                if fd, ok := d.(*ast.FuncDecl); ok && fd.Name != nil {
                    if strings.HasPrefix(fd.Name.Name, "Test") { testNames[fd.Name.Name] = struct{}{} }
                }
            }
            ast.Inspect(tf, func(n ast.Node) bool {
                switch x := n.(type) {
                case *ast.Ident:
                    testIdents[x.Name] = struct{}{}
                case *ast.SelectorExpr:
                    testIdents[x.Sel.Name] = struct{}{}
                }
                return true
            })
            return nil
        })

        // Filter funcs missing coverage signal
        var miss []FuncInfo
        for _, fn := range funcs {
            // Heuristic: if function name appears in tests, assume covered
            if _, ok := testIdents[fn.Name]; ok { continue }
            // Methods: also consider receiver type name for TestType_Method style
            if fn.Recv != "" {
                // e.g., TestType_Method often contains Method name as Ident; already covered by above
            }
            // Skip generated or trivial files? we'll decide in recommendation
            miss = append(miss, fn)
        }
        if len(miss) == 0 { continue }
        sort.Slice(miss, func(i, j int) bool {
            if miss[i].File == miss[j].File {
                return miss[i].Name < miss[j].Name
            }
            return miss[i].File < miss[j].File
        })
        all = append(all, missing{pkg: pkg, dir: dir, items: miss})
    }

    // Write markdown report to stdout
    fmt.Println("# Unit tests to consider adding")
    fmt.Println()
    fmt.Println("This report lists functions that do not appear to be referenced in test files in the same package directory. Recommendations are heuristic.")
    fmt.Println()
    for _, m := range all {
        fmt.Printf("## %s (%s)\n\n", m.pkg, m.dir)
        for _, fn := range m.items {
            rec := recommend(fn)
            var sig string
            if fn.Recv != "" { sig = fmt.Sprintf("method (%s).%s", fn.Recv, fn.Name) } else { sig = "func " + fn.Name }
            fmt.Printf("- %s — %s\n", relPath(fn.File), sig)
            fmt.Printf("  - exported: %t  complexity: %d  has-control: %t\n", fn.Exported, fn.Complexity, fn.HasControl)
            fmt.Printf("  - recommendation: %s\n", rec)
        }
        fmt.Println()
    }
}

func typeString(t ast.Expr) string {
    switch x := t.(type) {
    case *ast.Ident:
        return x.Name
    case *ast.StarExpr:
        return typeString(x.X)
    case *ast.IndexExpr:
        return typeString(x.X)
    case *ast.IndexListExpr:
        return typeString(x.X)
    case *ast.SelectorExpr:
        return x.Sel.Name
    default:
        return ""
    }
}

func recommend(fn FuncInfo) string {
    // Strongly recommend for exported functions and methods on exported types
    if fn.Exported { return "add test (exported)" }
    if fn.Recv != "" && isExportedIdent(fn.Recv) { return "add test (exported receiver)" }
    // Recommend for functions with control flow or non-trivial body
    if fn.HasControl || fn.Complexity >= 3 { return "add test (non-trivial logic)" }
    // Otherwise, optional
    return "optional (helper/trivial)"
}

func isExportedIdent(name string) bool {
    if name == "" { return false }
    r := []rune(name)
    return r[0] >= 'A' && r[0] <= 'Z'
}

func relPath(p string) string {
    if rp, err := filepath.Rel(".", p); err == nil { return rp }
    return p
}
