package projectanalysis

import (
	"regexp"
	"strings"
)

// Limit these hints to unambiguous single-name PHP declarations. Union,
// intersection and complex parameter types intentionally remain unknown.
var phpTypedParameterRE = regexp.MustCompile(`(?:^|,)[ \t\r\n]*(\??\\?[A-Za-z_][A-Za-z0-9_\\]*)[ \t\r\n]+&?[ \t\r\n]*(\$[A-Za-z_][A-Za-z0-9_]*)\b`)
var phpExtendsRE = regexp.MustCompile(`(?i)\bclass[ \t\r\n]+([A-Za-z_][A-Za-z0-9_]*)[ \t\r\n]+extends[ \t\r\n]+(\\?[A-Za-z_][A-Za-z0-9_\\]*)\b[^{]*\{`)
var phpBuiltinType = map[string]bool{
	"int": true, "integer": true, "bool": true, "boolean": true, "float": true, "double": true,
	"string": true, "array": true, "iterable": true, "mixed": true, "object": true,
	"callable": true, "void": true, "null": true, "true": true, "false": true,
	"never": true, "self": true, "static": true, "parent": true,
}

// phpParameterTypeHints records advisory type declarations, not proof of an
// object's runtime class. A later variable assignment may change the value.
func phpParameterTypeHints(masked []byte, argsStart int, file phpCallerFile) map[string]string {
	out := map[string]string{}
	if argsStart < 0 || argsStart > len(masked) {
		return out
	}
	depth := 1
	close := -1
	for i := argsStart; i < len(masked); i++ {
		switch masked[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				close = i
			}
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 || close-argsStart > 4096 {
		return out
	}
	params := masked[argsStart:close]
	for _, m := range phpTypedParameterRE.FindAllSubmatch(params, -1) {
		typ := strings.TrimPrefix(string(m[1]), "?")
		if phpBuiltinType[strings.ToLower(typ)] {
			continue
		}
		owner, ok := file.qualifyReceiver(typ, argsStart, true)
		if ok {
			variable := string(m[2])
			out[variable] = owner
		}
	}
	return out
}

// phpDirectParentByScope extracts only explicit, simple class extends clauses.
// No trait, interface, or multi-level inheritance inference is performed here.
func phpDirectParentByScope(masked []byte, file phpCallerFile) map[int]string {
	out := map[int]string{}
	for _, m := range phpExtendsRE.FindAllSubmatchIndex(masked, -1) {
		if m[1] <= 0 {
			continue
		}
		open := m[1] - 1
		klass := ""
		for _, scope := range file.scopes {
			if scope.kind == "class" && scope.start == open {
				klass = scope.name
				break
			}
		}
		if klass == "" || string(masked[m[2]:m[3]]) != klass {
			continue
		}
		raw := string(masked[m[4]:m[5]])
		parent, known := file.qualifyReceiver(raw, open, true)
		if known && parent != "" && !strings.EqualFold(parent, file.withNamespace(klass)) {
			out[open] = parent
		}
	}
	return out
}
