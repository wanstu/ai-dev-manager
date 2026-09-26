package projectanalysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const OverviewRelativePath = ".adm/project-overview.md"

type Options struct {
	MaxFiles   int
	MaxSymbols int
}

type Result struct {
	OverviewPath    string   `json:"overview_path"`
	Languages       []string `json:"languages"`
	FilesScanned    int      `json:"files_scanned"`
	GoFiles         int      `json:"go_files"`
	PHPFiles        int      `json:"php_files"`
	Symbols         int      `json:"symbols"`
	GoModule        string   `json:"go_module,omitempty"`
	ComposerPackage string   `json:"composer_package,omitempty"`
	Truncated       bool     `json:"truncated,omitempty"`
	ParseIssues     int      `json:"parse_issues,omitempty"`
	Markdown        string   `json:"-"`
}

type fileOutline struct {
	Path      string
	Language  string
	Namespace string
	Symbols   []symbol
}

type symbol struct {
	Kind string
	Name string
	Line int
}

var (
	phpNamespaceRE = regexp.MustCompile("(?m)^\\s*namespace\\s+([A-Za-z_\\\\][A-Za-z0-9_\\\\]*)\\s*;")
	phpTypeRE      = regexp.MustCompile("(?m)^\\s*(?:(?:abstract|final|readonly)\\s+)*(class|interface|trait|enum)\\s+([A-Za-z_][A-Za-z0-9_]*)")
	phpFunctionRE  = regexp.MustCompile("(?m)^\\s*(?:(?:public|protected|private|static|final|abstract|readonly)\\s+)*function\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\(")
)

func Analyze(root string, options Options) (Result, error) {
	if options.MaxFiles <= 0 {
		options.MaxFiles = 4000
	}
	if options.MaxSymbols <= 0 {
		options.MaxSymbols = 1200
	}
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, err
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("project root is not a directory")
	}

	result := Result{
		OverviewPath:    OverviewRelativePath,
		GoModule:        readGoModule(filepath.Join(root, "go.mod")),
		ComposerPackage: readComposerPackage(filepath.Join(root, "composer.json")),
	}
	keyFiles := rootKeyFiles(root)
	dirCounts := map[string]int{}
	var outlines []fileOutline
	remainingSymbols := options.MaxSymbols

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if result.FilesScanned >= options.MaxFiles {
			result.Truncated = true
			return fs.SkipAll
		}
		result.FilesScanned++

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go":
			result.GoFiles++
			dirCounts[summaryDir(rel)]++
			if remainingSymbols > 0 {
				outline, parseIssue := analyzeGo(path, rel, remainingSymbols)
				if parseIssue {
					result.ParseIssues++
				}
				remainingSymbols -= len(outline.Symbols)
				result.Symbols += len(outline.Symbols)
				if len(outline.Symbols) > 0 {
					outlines = append(outlines, outline)
				}
			} else {
				result.Truncated = true
			}
		case ".php":
			result.PHPFiles++
			dirCounts[summaryDir(rel)]++
			if remainingSymbols > 0 {
				outline, parseIssue := analyzePHP(path, rel, remainingSymbols)
				if parseIssue {
					result.ParseIssues++
				}
				remainingSymbols -= len(outline.Symbols)
				result.Symbols += len(outline.Symbols)
				if len(outline.Symbols) > 0 {
					outlines = append(outlines, outline)
				}
			} else {
				result.Truncated = true
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	if result.GoFiles > 0 || result.GoModule != "" {
		result.Languages = append(result.Languages, "Go")
	}
	if result.PHPFiles > 0 || result.ComposerPackage != "" {
		result.Languages = append(result.Languages, "PHP")
	}
	sort.Slice(outlines, func(i, j int) bool { return outlines[i].Path < outlines[j].Path })
	result.Markdown = render(result, keyFiles, dirCounts, outlines)
	return result, nil
}

func ignoredDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".adm", "vendor", "node_modules", "dist", "build", "coverage", ".idea", ".vscode", ".next", ".cache":
		return true
	default:
		return false
	}
}

func readGoModule(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func readComposerPackage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var payload struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.Name)
}

func rootKeyFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	known := map[string]bool{
		"go.mod": true, "go.sum": true, "go.work": true,
		"composer.json": true, "composer.lock": true,
		"package.json": true, "dockerfile": true, "makefile": true,
		"readme": true, "readme.md": true, "readme.txt": true,
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() && known[strings.ToLower(entry.Name())] {
			out = append(out, entry.Name())
		}
	}
	sort.Strings(out)
	return out
}

func summaryDir(rel string) string {
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return "."
	}
	parts := strings.Split(dir, "/")
	if len(parts) <= 3 {
		return dir
	}
	return strings.Join(parts[:3], "/") + "/..."
}

func analyzeGo(path, rel string, limit int) (fileOutline, bool) {
	out := fileOutline{Path: rel, Language: "Go"}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if file == nil {
		return out, err != nil
	}
	if file.Name != nil {
		out.Namespace = file.Name.Name
	}
	for _, decl := range file.Decls {
		if len(out.Symbols) >= limit {
			break
		}
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				out.Symbols = append(out.Symbols, symbol{Kind: "type", Name: ts.Name.Name, Line: fset.Position(ts.Pos()).Line})
				if len(out.Symbols) >= limit {
					break
				}
			}
		case *ast.FuncDecl:
			kind, name := "func", d.Name.Name
			if receiver := receiverName(d); receiver != "" {
				kind, name = "method", receiver+"."+name
			}
			out.Symbols = append(out.Symbols, symbol{Kind: kind, Name: name, Line: fset.Position(d.Pos()).Line})
		}
	}
	return out, err != nil
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	var render func(ast.Expr) string
	render = func(expr ast.Expr) string {
		switch e := expr.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.StarExpr:
			return render(e.X)
		case *ast.IndexExpr:
			return render(e.X)
		case *ast.IndexListExpr:
			return render(e.X)
		default:
			return ""
		}
	}
	return render(fn.Recv.List[0].Type)
}

func analyzePHP(path, rel string, limit int) (fileOutline, bool) {
	out := fileOutline{Path: rel, Language: "PHP"}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, true
	}
	parseIssue := false
	if len(data) > 1<<20 {
		data = data[:1<<20]
		parseIssue = true
	}
	if match := phpNamespaceRE.FindSubmatch(data); len(match) > 1 {
		out.Namespace = string(match[1])
	}
	for _, match := range phpTypeRE.FindAllSubmatchIndex(data, -1) {
		if len(out.Symbols) >= limit {
			break
		}
		out.Symbols = append(out.Symbols, symbol{
			Kind: string(data[match[2]:match[3]]),
			Name: string(data[match[4]:match[5]]),
			Line: lineAt(data, match[0]),
		})
	}
	for _, match := range phpFunctionRE.FindAllSubmatchIndex(data, -1) {
		if len(out.Symbols) >= limit {
			break
		}
		out.Symbols = append(out.Symbols, symbol{
			Kind: "function",
			Name: string(data[match[2]:match[3]]),
			Line: lineAt(data, match[0]),
		})
	}
	sort.SliceStable(out.Symbols, func(i, j int) bool {
		if out.Symbols[i].Line == out.Symbols[j].Line {
			return out.Symbols[i].Name < out.Symbols[j].Name
		}
		return out.Symbols[i].Line < out.Symbols[j].Line
	})
	return out, parseIssue
}

func lineAt(data []byte, offset int) int {
	if offset <= 0 {
		return 1
	}
	return bytes.Count(data[:offset], []byte{'\n'}) + 1
}

func render(result Result, keyFiles []string, dirCounts map[string]int, outlines []fileOutline) string {
	var out strings.Builder
	out.WriteString("# ADM Project Overview\n\n")
	out.WriteString("> Generated by ADM static project analysis. Project code was not executed.\n\n")
	out.WriteString("## Project facts\n\n")
	languages := "Unknown"
	if len(result.Languages) > 0 {
		languages = strings.Join(result.Languages, ", ")
	}
	fmt.Fprintf(&out, "- Root: .\n- Detected languages: %s\n- Files scanned: %d\n- Go files: %d\n- PHP files: %d\n- Symbols indexed: %d\n", languages, result.FilesScanned, result.GoFiles, result.PHPFiles, result.Symbols)
	if result.GoModule != "" {
		fmt.Fprintf(&out, "- Go module: %s\n", result.GoModule)
	}
	if result.ComposerPackage != "" {
		fmt.Fprintf(&out, "- Composer package: %s\n", result.ComposerPackage)
	}
	if result.ParseIssues > 0 {
		fmt.Fprintf(&out, "- Parse/read issues: %d\n", result.ParseIssues)
	}
	if result.Truncated {
		out.WriteString("- Analysis bounded: yes\n")
	}

	out.WriteString("\n## Key files\n\n")
	if len(keyFiles) == 0 {
		out.WriteString("- None detected at project root.\n")
	} else {
		for _, name := range keyFiles {
			fmt.Fprintf(&out, "- %s\n", name)
		}
	}

	out.WriteString("\n## Directory outline\n\n")
	dirs := make([]string, 0, len(dirCounts))
	for dir := range dirCounts {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		out.WriteString("- No Go/PHP source directories detected.\n")
	} else {
		for _, dir := range dirs {
			fmt.Fprintf(&out, "- %s — %d source files\n", dir, dirCounts[dir])
		}
	}

	out.WriteString("\n## Source outline\n\n")
	if len(outlines) == 0 {
		out.WriteString("No indexed Go/PHP declarations.\n")
	} else {
		for _, file := range outlines {
			fmt.Fprintf(&out, "### %s\n\n", file.Path)
			if file.Namespace != "" {
				label := "Package"
				if file.Language == "PHP" {
					label = "Namespace"
				}
				fmt.Fprintf(&out, "%s: %s\n\n", label, file.Namespace)
			}
			for _, item := range file.Symbols {
				fmt.Fprintf(&out, "- L%d · %s · %s\n", item.Line, item.Kind, item.Name)
			}
			out.WriteString("\n")
		}
	}

	out.WriteString("## Analysis boundaries\n\n")
	out.WriteString("- Static only: ADM does not execute project code during analysis.\n")
	out.WriteString("- Common dependency/generated directories are skipped: .git, .adm, vendor, node_modules, dist, build and similar.\n")
	out.WriteString("- Go declarations use the Go parser; PHP outline extraction is heuristic.\n")
	return out.String()
}
