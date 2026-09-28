package codeintel

import "strings"

type SymbolLocator struct {
	Path          string `json:"path,omitempty"`
	Line          int    `json:"line,omitempty"`
	Name          string `json:"name,omitempty"`
	QualifiedName string `json:"qualified_name,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Language      string `json:"language,omitempty"`
}

func (s SymbolLocator) Valid() bool {
	return strings.TrimSpace(s.Path) != "" ||
		strings.TrimSpace(s.Name) != "" ||
		strings.TrimSpace(s.QualifiedName) != ""
}

type Reference struct {
	Path          string `json:"path"`
	Line          int    `json:"line"`
	Column        int    `json:"column,omitempty"`
	EndLine       int    `json:"end_line,omitempty"`
	EndColumn     int    `json:"end_column,omitempty"`
	Language      string `json:"language,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Name          string `json:"name,omitempty"`
	QualifiedName string `json:"qualified_name,omitempty"`
	Context       string `json:"context,omitempty"`
}

type ReferencesResult struct {
	Symbol     SymbolLocator `json:"symbol"`
	References []Reference   `json:"references"`
	Returned   int           `json:"returned"`
	Truncated  bool          `json:"truncated,omitempty"`
}

type HierarchyNode struct {
	ID            string `json:"id,omitempty"`
	Path          string `json:"path,omitempty"`
	Line          int    `json:"line,omitempty"`
	Language      string `json:"language,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name,omitempty"`
}

type HierarchyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type HierarchyResult struct {
	Symbol    SymbolLocator   `json:"symbol"`
	Direction string          `json:"direction"`
	Nodes     []HierarchyNode `json:"nodes"`
	Edges     []HierarchyEdge `json:"edges"`
	Returned  int             `json:"returned"`
	Truncated bool            `json:"truncated,omitempty"`
}
