package projectanalysis

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxPHPCallSourceBytes = 1 << 20
const maxPHPCallsPerFile = 20000
const maxIndexedPHPCalls = 250000

// PHPCallRecord is a lexical call site, independent of the query target.
// OwnerKnown captures only a safely understood lexical receiver and must not
// be interpreted as proof of runtime dispatch.
type PHPCallRecord struct {
	Path                string `json:"path"`
	Line                int    `json:"line"`
	Column              int    `json:"column"`
	Name                string `json:"name"`
	CallKind            string `json:"call_kind"`
	Owner               string `json:"owner,omitempty"`
	OwnerKnown          bool   `json:"owner_known,omitempty"`
	CallerQualifiedName string `json:"caller_qualified_name,omitempty"`
	CallerKind          string `json:"caller_kind,omitempty"`
	CallerLine          int    `json:"caller_line,omitempty"`
	Context             string `json:"context,omitempty"`
}

var phpMethodCallRE = regexp.MustCompile(`(?i)(?:->|::)[ \t]*([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(`)
var phpFunctionCallRE = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(`)
var phpBeforeFunctionRE = regexp.MustCompile(`(?i)(?:\bfunction|\bfn|\bnew)[ \t]*$`)

func collectPHPCalls(path, rel string) ([]PHPCallRecord, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	complete := true
	if len(data) > maxPHPCallSourceBytes {
		data = data[:maxPHPCallSourceBytes]
		complete = false
	}
	masked := maskSource(data, true)
	caller := newPHPCallerFile(masked)
	functionBodies := phpNamedFunctionBodies(masked, caller)
	lineStarts := []int{0}
	for offset, b := range data {
		if b == '\n' {
			lineStarts = append(lineStarts, offset+1)
		}
	}
	calls := make([]PHPCallRecord, 0)
	appendCall := func(name, kind string, start, nameStart, matchEnd int, owner string, known bool) {
		if len(calls) >= maxPHPCallsPerFile {
			complete = false
			return
		}
		// Binary search precomputed line starts: avoids O(calls × bytes)
		// rescanning for large or minified PHP source files.
		lineIndex := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > start }) - 1
		lineStart := lineStarts[lineIndex]
		lineEnd := len(data)
		if lineIndex+1 < len(lineStarts) {
			lineEnd = lineStarts[lineIndex+1] - 1
		}
		call := PHPCallRecord{
			Path:   filepath.ToSlash(rel),
			Line:   lineIndex + 1,
			Column: nameStart + 1 - lineStart,
			Name:   name, CallKind: kind, Owner: owner, OwnerKnown: known,
			Context: phpReferenceContext(data, lineStart, lineEnd, start, matchEnd),
		}
		if scope, ok := phpCallerAt(start, functionBodies); ok {
			call.CallerQualifiedName = scope.name
			call.CallerKind = scope.kind
			call.CallerLine = scope.line
		}
		calls = append(calls, call)
	}
	for _, m := range phpMethodCallRE.FindAllSubmatchIndex(masked, -1) {
		start := m[0]
		if m[2] < 0 {
			continue
		}
		name := string(masked[m[2]:m[3]])
		receiver := readPHPReceiver(masked, start)
		owner, known := caller.qualifyReceiver(receiver, start, string(masked[start:start+2]) == "::")
		appendCall(name, "method", start, m[2], m[1], owner, known)
	}
	for _, m := range phpFunctionCallRE.FindAllSubmatchIndex(masked, -1) {
		start, nameStart := m[0], m[2]
		if nameStart < 0 {
			continue
		}
		// This is not a free function call if qualified by a PHP member
		// operator, or if the name is part of a declaration / constructor.
		prefixStart := max(0, start-40)
		before := strings.TrimRight(string(masked[prefixStart:start]), " \t")
		if strings.HasSuffix(before, "->") || strings.HasSuffix(before, "::") ||
			strings.HasSuffix(before, "$") || phpBeforeFunctionRE.MatchString(before) {
			continue
		}
		name := string(masked[m[2]:m[3]])
		switch strings.ToLower(name) {
		case "if", "for", "foreach", "while", "switch", "catch", "isset", "empty", "unset", "echo", "array", "list", "match", "include", "require", "include_once", "require_once", "function", "fn", "new":
			continue
		}
		appendCall(name, "function", start, nameStart, m[1], "", false)
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Line != calls[j].Line {
			return calls[i].Line < calls[j].Line
		}
		if calls[i].Column != calls[j].Column {
			return calls[i].Column < calls[j].Column
		}
		return calls[i].Name < calls[j].Name
	})
	return calls, complete, nil
}
