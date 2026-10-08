package projectanalysis

import (
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	jsFunctionRE = regexp.MustCompile("(?m)^[ \\t]*(?:export[ \\t]+(?:default[ \\t]+)?)?(?:async[ \\t]+)?function[ \\t]*\\*?[ \\t]+([A-Za-z_$][A-Za-z0-9_$]*)[ \\t]*\\(")
	jsClassRE    = regexp.MustCompile("(?m)^[ \\t]*(?:export[ \\t]+(?:default[ \\t]+)?)?(?:abstract[ \\t]+)?class[ \\t]+([A-Za-z_$][A-Za-z0-9_$]*)\\b")
	jsArrowRE    = regexp.MustCompile("(?m)^[ \\t]*(?:export[ \\t]+)?(?:const|let|var)[ \\t]+([A-Za-z_$][A-Za-z0-9_$]*)[ \\t]*=[ \\t]*(?:async[ \\t]+)?(?:\\([^\\r\\n]*?\\)|[A-Za-z_$][A-Za-z0-9_$]*)[ \\t]*=>")
	tsTypeRE     = regexp.MustCompile("(?m)^[ \\t]*(?:export[ \\t]+)?(?:declare[ \\t]+)?(interface|type)[ \\t]+([A-Za-z_$][A-Za-z0-9_$]*)\\b")
)

// maskSource preserves byte offsets and newlines while hiding comments and
// quoted contents. This is intentionally a bounded outline heuristic, not a
// JavaScript or PHP grammar parser.
func maskSource(data []byte, php bool) []byte {
	masked := append([]byte(nil), data...)
	erase := func(index int) {
		if masked[index] != '\n' && masked[index] != '\r' {
			masked[index] = ' '
		}
	}
	for i := 0; i < len(data); {
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
			j := i + 2
			for j < len(data) && data[j] != '\n' {
				j++
			}
			for k := i; k < j; k++ {
				erase(k)
			}
			i = j
			continue
		}
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
			j := i + 2
			for j+1 < len(data) && !(data[j] == '*' && data[j+1] == '/') {
				j++
			}
			if j+1 < len(data) {
				j += 2
			} else {
				j = len(data)
			}
			for k := i; k < j; k++ {
				erase(k)
			}
			i = j
			continue
		}
		if php && data[i] == '#' && (i+1 == len(data) || data[i+1] != '[') {
			j := i + 1
			for j < len(data) && data[j] != '\n' {
				j++
			}
			for k := i; k < j; k++ {
				erase(k)
			}
			i = j
			continue
		}
		if data[i] == '\'' || data[i] == '"' || (!php && data[i] == '`') {
			quote := data[i]
			j := i + 1
			for j < len(data) {
				if data[j] == '\\' && j+1 < len(data) {
					j += 2
					continue
				}
				if data[j] == quote {
					j++
					break
				}
				j++
			}
			for k := i; k < j; k++ {
				erase(k)
			}
			i = j
			continue
		}
		i++
	}
	return masked
}

type phpTypeScope struct {
	kind  string
	name  string
	start int
	end   int
}

func phpTypeScopes(masked []byte) []phpTypeScope {
	scopes := make([]phpTypeScope, 0)
	for _, match := range phpTypeRE.FindAllSubmatchIndex(masked, -1) {
		// Ignore forward declarations and incomplete headers.
		open := -1
		for j := match[1]; j < len(masked); j++ {
			if masked[j] == ';' || masked[j] == '}' {
				break
			}
			if masked[j] == '{' {
				open = j
				break
			}
		}
		if open < 0 {
			continue
		}
		depth := 1
		close := -1
		for j := open + 1; j < len(masked); j++ {
			switch masked[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					close = j
				}
			}
			if close >= 0 {
				break
			}
		}
		if close >= 0 {
			scopes = append(scopes, phpTypeScope{
				kind:  string(masked[match[2]:match[3]]),
				name:  string(masked[match[4]:match[5]]),
				start: open,
				end:   close,
			})
		}
	}
	return scopes
}

func analyzeJavaScript(path, rel, extension string, limit int) (fileOutline, bool) {
	language := "JavaScript"
	if extension == ".ts" || extension == ".tsx" {
		language = "TypeScript"
	}
	out := fileOutline{Path: rel, Language: language}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, true
	}
	parseIssue := false
	if len(data) > 1<<20 {
		data = data[:1<<20]
		parseIssue = true
	}
	masked := maskSource(data, false)
	// Precompute nesting once: large frontend files often have thousands of
	// declarations, so rescanning the prefix for each match is quadratic.
	depthAt := make([]int, len(masked)+1)
	for i, b := range masked {
		depthAt[i+1] = depthAt[i]
		switch b {
		case '{':
			depthAt[i+1]++
		case '}':
			if depthAt[i+1] > 0 {
				depthAt[i+1]--
			}
		}
	}
	appendMatches := func(re *regexp.Regexp, kind string, nameIndex int) {
		for _, match := range re.FindAllSubmatchIndex(masked, -1) {
			if len(out.Symbols) >= limit {
				return
			}
			// Only file-scope declarations; nested declarations need a
			// qualified scope model rather than pretending to be globals.
			if depthAt[match[0]] != 0 {
				continue
			}
			name := string(masked[match[nameIndex]:match[nameIndex+1]])
			symbolKind := kind
			if kind == "ts_type" {
				symbolKind = string(masked[match[2]:match[3]])
			}
			out.Symbols = append(out.Symbols, symbol{Kind: symbolKind, Name: name, Line: lineAt(masked, match[0])})
		}
	}
	appendMatches(jsFunctionRE, "function", 2)
	appendMatches(jsClassRE, "class", 2)
	appendMatches(jsArrowRE, "function", 2)
	if language == "TypeScript" {
		appendMatches(tsTypeRE, "ts_type", 4)
	}
	sort.SliceStable(out.Symbols, func(i, j int) bool {
		if out.Symbols[i].Line != out.Symbols[j].Line {
			return out.Symbols[i].Line < out.Symbols[j].Line
		}
		return strings.Compare(out.Symbols[i].Name, out.Symbols[j].Name) < 0
	})
	return out, parseIssue
}
