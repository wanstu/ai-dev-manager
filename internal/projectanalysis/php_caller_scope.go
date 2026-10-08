package projectanalysis

import (
	"bytes"
	"sort"
	"strings"
)

// phpFunctionBody denotes a named lexical function/method body. No PHP code
// is executed. Anonymous functions/closures stay an explicit limitation.
type phpFunctionBody struct {
	start, end int
	name, kind string
	line       int
}

func phpMatchingBrace(masked []byte, open int) int {
	if open < 0 || open >= len(masked) || masked[open] != '{' {
		return -1
	}
	depth := 1
	for i := open + 1; i < len(masked); i++ {
		switch masked[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// Locate the opening body brace after a named function's parameter list;
// declaration-only signatures and incomplete bodies are not attributed.
func phpFunctionOpenBrace(masked []byte, matchEnd int) int {
	depth := 1
	i := matchEnd
	for ; i < len(masked); i++ {
		switch masked[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				i++
				goto afterArguments
			}
		}
	}
	return -1
afterArguments:
	for ; i < len(masked); i++ {
		switch masked[i] {
		case ';', '}':
			return -1
		case '{':
			return i
		}
	}
	return -1
}

func phpNamedFunctionBodies(masked []byte, file phpCallerFile) []phpFunctionBody {
	matches := phpFunctionRE.FindAllSubmatchIndex(masked, -1)
	out := make([]phpFunctionBody, 0, len(matches))
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 {
			continue
		}
		open := phpFunctionOpenBrace(masked, m[1])
		close := phpMatchingBrace(masked, open)
		if close < 0 {
			continue
		}
		name := string(masked[m[2]:m[3]])
		kind := "function"
		for _, scope := range file.scopes {
			if scope.start < m[0] && m[0] < scope.end {
				name = file.withNamespace(scope.name) + "::" + name
				kind = "method"
				break
			}
		}
		if kind == "function" && file.namespace != "" && !file.ambiguousNamespace {
			name = file.withNamespace(name)
		}
		if file.ambiguousNamespace { // cannot attribute across namespace blocks
			continue
		}
		out = append(out, phpFunctionBody{start: open, end: close, name: name, kind: kind, line: bytes.Count(masked[:m[0]], []byte{'\n'}) + 1})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

func phpCallerAt(offset int, funcs []phpFunctionBody) (phpFunctionBody, bool) {
	var chosen phpFunctionBody
	found := false
	for _, f := range funcs {
		if f.start < offset && offset < f.end {
			if !found || f.start > chosen.start {
				chosen = f
				found = true
			}
		}
	}
	return chosen, found
}

// Short function names are intentionally not considered unique globally.
func phpCallerShortName(qualified string) string {
	if i := strings.LastIndex(qualified, "::"); i >= 0 {
		return qualified[i+2:]
	}
	if i := strings.LastIndex(qualified, "\\"); i >= 0 {
		return qualified[i+1:]
	}
	return qualified
}
