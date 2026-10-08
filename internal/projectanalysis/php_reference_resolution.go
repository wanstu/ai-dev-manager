package projectanalysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type PHPReferenceQuery struct {
	Name       string
	Path       string
	Kind       string
	MaxResults int
}

type phpReferenceResolver struct {
	targetClass string
	targetFQN   string
	// A different receiver can be excluded only when its own implementation
	// of this method is known. It might otherwise inherit the target method.
	directOwners map[string]bool
}

func newPHPReferenceResolver(symbolsJSONL []byte, query PHPReferenceQuery) (phpReferenceResolver, error) {
	if query.Kind != "method" && !strings.Contains(query.Name, "::") {
		return phpReferenceResolver{}, nil
	}
	pos := strings.LastIndex(query.Name, "::")
	if pos < 0 {
		return phpReferenceResolver{}, nil
	}
	ownerName := strings.TrimPrefix(strings.TrimSpace(query.Name[:pos]), "\\")
	method := strings.TrimSpace(query.Name[pos+2:])
	if ownerName == "" || method == "" {
		return phpReferenceResolver{}, nil
	}
	var matches []SymbolRecord
	directOwners := make(map[string]bool)
	scan := bufio.NewScanner(strings.NewReader(string(symbolsJSONL)))
	scan.Buffer(make([]byte, 64*1024), 1<<20)
	for scan.Scan() {
		var s SymbolRecord
		if err := json.Unmarshal(scan.Bytes(), &s); err != nil {
			return phpReferenceResolver{}, fmt.Errorf("invalid indexed symbol: %w", err)
		}
		if s.Language != "PHP" || s.Kind != "method" {
			continue
		}
		split := strings.LastIndex(s.QualifiedName, "::")
		if split < 0 || !strings.EqualFold(s.QualifiedName[split+2:], method) {
			continue
		}
		class := s.QualifiedName[:split]
		directOwners[strings.ToLower(class)] = true
		if query.Path != "" && !strings.EqualFold(filepath.ToSlash(query.Path), filepath.ToSlash(s.Path)) {
			continue
		}
		short := class
		if i := strings.LastIndex(short, "\\"); i >= 0 {
			short = short[i+1:]
		}
		if strings.EqualFold(ownerName, class) || strings.EqualFold(ownerName, short) {
			matches = append(matches, s)
		}
	}
	if err := scan.Err(); err != nil {
		return phpReferenceResolver{}, err
	}
	if len(matches) != 1 {
		return phpReferenceResolver{}, nil
	}
	split := strings.LastIndex(matches[0].QualifiedName, "::")
	return phpReferenceResolver{targetClass: matches[0].QualifiedName[:split], targetFQN: matches[0].QualifiedName, directOwners: directOwners}, nil
}

// classifyIndexedCall preserves why the index did or did not connect a
// lexical call to this definition. Inheritance and parameter types are
// evidence for an AI investigator, not a claim about runtime dispatch.
func (r phpReferenceResolver) classifyIndexedCall(call PHPCallRecord) (kind, reason string, include bool) {
	if call.CallKind != "method" {
		return "candidate_call", "function_name_only", true
	}
	if r.targetFQN == "" {
		return "candidate_call", "target_definition_unresolved", true
	}
	if call.OwnerKnown {
		if strings.EqualFold(call.Owner, r.targetClass) {
			return "resolved_call", "lexical_owner_matches_definition", true
		}
		if r.directOwners[strings.ToLower(call.Owner)] {
			return "", "another_class_defines_method", false
		}
		if call.OwnerParent != "" && strings.EqualFold(call.OwnerParent, r.targetClass) {
			return "inherited_candidate", "direct_extends_clause", true
		}
		return "candidate_call", "receiver_hierarchy_unverified", true
	}
	if call.TypeHint != "" {
		if strings.EqualFold(call.TypeHint, r.targetClass) {
			return "candidate_call", "parameter_type_matches_definition", true
		}
		return "candidate_call", "parameter_type_other_or_subtype", true
	}
	return "candidate_call", "dynamic_receiver", true
}

var phpClassImportRE = regexp.MustCompile(`(?m)^[ \t]*use[ \t]+(\\?[A-Za-z_][A-Za-z0-9_]*(?:\\[A-Za-z_][A-Za-z0-9_]*)*)(?:[ \t]+as[ \t]+([A-Za-z_][A-Za-z0-9_]*))?[ \t]*;`)
var phpSimpleReceiverRE = regexp.MustCompile(`^(?:\$[A-Za-z_][A-Za-z0-9_]*|\\?[A-Za-z_][A-Za-z0-9_]*(?:\\[A-Za-z_][A-Za-z0-9_]*)*)$`)

type phpCallerFile struct {
	namespace          string
	ambiguousNamespace bool
	imports            map[string]string
	scopes             []phpTypeScope
}

func newPHPCallerFile(masked []byte) phpCallerFile {
	out := phpCallerFile{imports: map[string]string{}, scopes: phpTypeScopes(masked)}
	namespaces := phpNamespaceRE.FindAllSubmatch(masked, -1)
	if len(namespaces) == 1 {
		out.namespace = string(namespaces[0][1])
	} else if len(namespaces) > 1 {
		out.ambiguousNamespace = true
	}
	for _, m := range phpClassImportRE.FindAllSubmatchIndex(masked, -1) {
		if out.classAt(m[0]) != nil {
			continue
		}
		full := strings.TrimPrefix(string(masked[m[2]:m[3]]), "\\")
		alias := full
		if pos := strings.LastIndex(alias, "\\"); pos >= 0 {
			alias = alias[pos+1:]
		}
		if m[4] >= 0 {
			alias = string(masked[m[4]:m[5]])
		}
		key := strings.ToLower(alias)
		if prev, ok := out.imports[key]; ok && !strings.EqualFold(prev, full) {
			out.imports[key] = ""
		} else {
			out.imports[key] = full
		}
	}
	return out
}

func (f phpCallerFile) classAt(offset int) *phpTypeScope {
	for i := range f.scopes {
		scope := &f.scopes[i]
		if offset > scope.start && offset < scope.end && scope.kind == "class" {
			return scope
		}
	}
	return nil
}

func (f phpCallerFile) withNamespace(name string) string {
	if f.namespace == "" {
		return name
	}
	return f.namespace + "\\" + name
}

func (f phpCallerFile) qualifyReceiver(receiver string, offset int, static bool) (string, bool) {
	if f.ambiguousNamespace {
		return "", false
	}
	switch strings.ToLower(receiver) {
	case "$this":
		if !static {
			if scope := f.classAt(offset); scope != nil {
				return f.withNamespace(scope.name), true
			}
		}
		return "", false
	case "self":
		if static {
			if scope := f.classAt(offset); scope != nil {
				return f.withNamespace(scope.name), true
			}
		}
		return "", false
	case "static", "parent":
		return "", false
	}
	if !static || receiver == "" || strings.HasPrefix(receiver, "$") {
		return "", false
	}
	if strings.HasPrefix(receiver, "\\") {
		return strings.TrimPrefix(receiver, "\\"), true
	}
	head, tail, hasTail := strings.Cut(receiver, "\\")
	if alias, exists := f.imports[strings.ToLower(head)]; exists {
		if alias == "" {
			return "", false
		}
		if hasTail {
			return alias + "\\" + tail, true
		}
		return alias, true
	}
	return f.withNamespace(receiver), true
}

// Only a lexical identifier or variable qualifies; arbitrary receiver
// expressions remain candidates instead of being assigned a made-up class.
func readPHPReceiver(masked []byte, operatorOffset int) string {
	i := operatorOffset
	for i > 0 && (masked[i-1] == ' ' || masked[i-1] == '\t') {
		i--
	}
	end := i
	for i > 0 {
		b := masked[i-1]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '\\' || b == '$' {
			i--
			continue
		}
		break
	}
	if end == i {
		return ""
	}
	s := string(masked[i:end])
	if !phpSimpleReceiverRE.MatchString(s) {
		return ""
	}
	return s
}

// resolved_call = static receiver identified against one indexed declaration;
// candidate_call = dynamic/ambiguous receiver, not a proven edge.
func (r phpReferenceResolver) classifyCall(f phpCallerFile, masked []byte, offset int, isMethod bool) (string, bool) {
	if !isMethod || r.targetFQN == "" {
		return "candidate_call", true
	}
	if offset+2 > len(masked) {
		return "candidate_call", true
	}
	receiver := readPHPReceiver(masked, offset)
	owner, known := f.qualifyReceiver(receiver, offset, string(masked[offset:offset+2]) == "::")
	if !known {
		return "candidate_call", true
	}
	if strings.EqualFold(owner, r.targetClass) {
		return "resolved_call", true
	}
	if r.directOwners[strings.ToLower(owner)] {
		return "", false // Different class defines its own implementation.
	}
	return "candidate_call", true // Could be an inherited method.
}
