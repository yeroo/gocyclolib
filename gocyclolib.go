package gocyclolib

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sort"
)

var (
	skipGodepsGlobal = false
	skipVendorGlobal = false
	statsGlobal []stat = nil
)

func getStats(paths []string, skipGodeps bool, skipVendor bool) []stat {
	if statsGlobal == nil || skipGodepsGlobal != skipGodeps || skipVendorGlobal != skipVendor {
		skipGodepsGlobal = skipGodeps
		skipVendorGlobal = skipVendor
		statsGlobal = analyze(paths)
	}
	sort.Sort(byComplexity(statsGlobal))
	return statsGlobal
}

func Average(paths []string, skipGodeps bool, skipVendor bool) float64 {
	stats := getStats(paths, skipGodeps, skipVendor)
	return average(stats)
}
func GetStats(paths []string, skipGodeps bool, skipVendor bool) {
	return getStats(paths, skipGodeps, skipVendor)
}

func analyze(paths []string) []stat {
	var stats []stat
	for _, path := range paths {
		if isDir(path) {
			stats = analyzeDir(path, stats)
		} else {
			stats = analyzeFile(path, stats)
		}
	}
	return stats
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

func analyzeFile(fname string, stats []stat) []stat {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fname, nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	return buildStats(f, fset, stats)
}

func analyzeDir(dirname string, stats []stat) []stat {
	filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && isAnalyzeTargetGodeps(dirname, path) && isAnalyzeTargetVendor(dirname, path) {
			stats = analyzeFile(path, stats)
		}
		return err
	})
	return stats
}

func isAnalyzeTargetGodeps(dirname, path string) bool {
	prefix := strings.Join([]string{dirname, "Godeps"}, string(os.PathSeparator))
	if dirname == "." {
		prefix = "Godeps"
	}
	if strings.HasPrefix(path, prefix) && *skipGodepsGlobal {
		return false
	}
	return strings.HasSuffix(path, ".go")
}

func isAnalyzeTargetVendor(dirname, path string) bool {
	prefix := strings.Join([]string{dirname, "vendor"}, string(os.PathSeparator))
	if dirname == "." {
		prefix = "vendor"
	}
	if strings.HasPrefix(path, prefix) && *skipVendorGlobal {
		return false
	}
	return strings.HasSuffix(path, ".go")
}
//func writeStats(w io.Writer, sortedStats []stat) int {
//	for i, stat := range sortedStats {
//		if i == *top {
//			return i
//		}
//		if stat.Complexity <= *over {
//			return i
//		}
//		fmt.Fprintln(w, stat)
//	}
//	return len(sortedStats)
//}
//
//func showAverage(stats []stat, showLabel bool) {
//	if showLabel {
//		fmt.Printf("Average: %.3g\n", average(stats))
//	} else {
//		fmt.Printf("%.3g\n", average(stats))
//	}
//
//}

func average(stats []stat) float64 {
	total := 0
	for _, s := range stats {
		total += s.Complexity
	}
	return float64(total) / float64(len(stats))
}

type stat struct {
	PkgName    string
	FuncName   string
	Complexity int
	Pos        token.Position
}

func (s stat) String() string {
	return fmt.Sprintf("%d %s %s %s", s.Complexity, s.PkgName, s.FuncName, s.Pos)
}

type byComplexity []stat

func (s byComplexity) Len() int {
	return len(s)
}
func (s byComplexity) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byComplexity) Less(i, j int) bool {
	return s[i].Complexity >= s[j].Complexity
}

func buildStats(f *ast.File, fset *token.FileSet, stats []stat) []stat {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			stats = append(stats, stat{
				PkgName:    f.Name.Name,
				FuncName:   funcName(fn),
				Complexity: complexity(fn),
				Pos:        fset.Position(fn.Pos()),
			})
		}
	}
	return stats
}

// funcName returns the name representation of a function or method:
// "(Type).Name" for methods or simply "Name" for functions.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv != nil {
		if fn.Recv.NumFields() > 0 {
			typ := fn.Recv.List[0].Type
			return fmt.Sprintf("(%s).%s", recvString(typ), fn.Name)
		}
	}
	return fn.Name.Name
}

// recvString returns a string representation of recv of the
// form "T", "*T", or "BADRECV" (if not a proper receiver type).
func recvString(recv ast.Expr) string {
	switch t := recv.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + recvString(t.X)
	}
	return "BADRECV"
}

// complexity calculates the cyclomatic complexity of a function.
func complexity(fn *ast.FuncDecl) int {
	v := complexityVisitor{}
	ast.Walk(&v, fn)
	return v.Complexity
}

type complexityVisitor struct {
	// Complexity is the cyclomatic complexity
	Complexity int
}

// Visit implements the ast.Visitor interface.
func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.FuncDecl, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
		v.Complexity++
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			v.Complexity++
		}
	}
	return v
}