package gateway

import (
	"fmt"
	"strings"

	"ai-dev-manager-v2/internal/codeintel"
	"ai-dev-manager-v2/internal/projectanalysis"
)

// staticPHPCallReferences uses the already verified ADM index for bounded,
// lexical PHP call-site candidates. It deliberately does not claim semantic
// class resolution: dynamic PHP dispatch cannot be established by a text scan.
func (o *runtimeOwner) staticPHPCallReferences(environmentID string, symbol codeintel.SymbolLocator, maxResults int) (codeintel.ReferencesResult, error) {
	path := strings.ToLower(strings.TrimSpace(symbol.Path))
	language := strings.TrimSpace(symbol.Language)
	kind := strings.ToLower(strings.TrimSpace(symbol.Kind))
	if (language != "" && !strings.EqualFold(language, "PHP")) ||
		(path != "" && !strings.HasSuffix(path, ".php")) {
		return codeintel.ReferencesResult{}, fmt.Errorf("static PHP call lookup requires PHP source")
	}
	name := strings.TrimSpace(symbol.Name)
	if name == "" {
		name = strings.TrimSpace(symbol.QualifiedName)
	}
	if name == "" {
		return codeintel.ReferencesResult{}, fmt.Errorf("static PHP call lookup requires a function or method name")
	}
	if !strings.EqualFold(language, "PHP") && !strings.HasSuffix(path, ".php") &&
		!strings.Contains(name, "::") && !strings.Contains(symbol.QualifiedName, "::") {
		return codeintel.ReferencesResult{}, fmt.Errorf("static PHP call lookup requires an explicit PHP target")
	}
	if kind != "" && kind != "function" && kind != "method" {
		return codeintel.ReferencesResult{}, fmt.Errorf("static PHP call lookup does not support kind %q", kind)
	}
	if kind == "" && strings.Contains(symbol.QualifiedName, "::") {
		kind = "method"
	}
	rt, _, err := o.service.Runtime(environmentID)
	if err != nil {
		return codeintel.ReferencesResult{}, err
	}
	// Prefer the fully qualified indexed method when the caller supplies it.
	// Short-only names stay candidates whenever ownership is ambiguous.
	if strings.Contains(symbol.QualifiedName, "::") {
		name = symbol.QualifiedName
	}
	candidates, err := projectanalysis.FindPHPCallReferences(rt.Root(), projectanalysis.PHPReferenceQuery{
		Name: name, Path: symbol.Path, Kind: kind, MaxResults: maxResults,
	})
	if err != nil {
		return codeintel.ReferencesResult{}, err
	}
	result := codeintel.ReferencesResult{
		Symbol: symbol, References: make([]codeintel.Reference, 0, len(candidates.References)),
		Returned: len(candidates.References), Truncated: candidates.Truncated,
	}
	for _, match := range candidates.References {
		result.References = append(result.References, codeintel.Reference{
			Path: match.Path, Line: match.Line, Column: match.Column,
			Language: "PHP", Kind: match.Kind, Name: match.Name,
			Reason: match.Reason, TypeHint: match.TypeHint, Context: match.Context,
		})
	}
	return result, nil
}
