package codeintel

import "ai-dev-manager-v2/internal/projectanalysis"

type Capabilities struct {
	Definitions bool `json:"definitions"`
	References  bool `json:"references"`
	Hierarchy   bool `json:"hierarchy"`
}

type ProviderInfo struct {
	ID                     string       `json:"provider_id"`
	Name                   string       `json:"name"`
	Source                 string       `json:"source"`
	RequiresGeneratedIndex bool         `json:"requires_generated_index"`
	Capabilities           Capabilities `json:"capabilities"`
}

type Provider interface {
	Info() ProviderInfo
	QuerySymbols(root string, query projectanalysis.IndexQuery) (projectanalysis.IndexQueryResult, error)
	Status(root string, maxChanges int) (projectanalysis.IndexStatusResult, error)
}

type StaticIndexProvider struct{}

func NewStaticIndexProvider() Provider {
	return StaticIndexProvider{}
}

func (StaticIndexProvider) Info() ProviderInfo {
	return ProviderInfo{
		ID:                     "adm_static_index",
		Name:                   "ADM Static Project Index",
		Source:                 ".adm/index",
		RequiresGeneratedIndex: true,
		Capabilities: Capabilities{
			Definitions: true,
			References:  false,
			Hierarchy:   false,
		},
	}
}

func (StaticIndexProvider) QuerySymbols(root string, query projectanalysis.IndexQuery) (projectanalysis.IndexQueryResult, error) {
	return projectanalysis.QueryIndex(root, query)
}

func (StaticIndexProvider) Status(root string, maxChanges int) (projectanalysis.IndexStatusResult, error) {
	return projectanalysis.IndexStatus(root, maxChanges)
}
