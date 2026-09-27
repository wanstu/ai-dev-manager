package app

import (
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/codeintel"
	"ai-dev-manager-v2/internal/projectanalysis"
)

type fakeCodeIntelligenceProvider struct{}

func (fakeCodeIntelligenceProvider) Info() codeintel.ProviderInfo {
	return codeintel.ProviderInfo{
		ID:     "fake_phpstorm",
		Name:   "Fake PhpStorm Provider",
		Source: "test",
		Capabilities: codeintel.Capabilities{
			Definitions: true,
			References:  true,
			Hierarchy:   true,
		},
	}
}

func (fakeCodeIntelligenceProvider) QuerySymbols(_ string, _ projectanalysis.IndexQuery) (projectanalysis.IndexQueryResult, error) {
	return projectanalysis.IndexQueryResult{
		IndexPath: "provider://fake",
		Returned:  1,
		Matches: []projectanalysis.IndexQueryMatch{{
			Path: "src/Foo.php", Language: "PHP", Kind: "class", Name: "Foo", QualifiedName: "App\\Foo", Line: 12, Match: "provider",
		}},
	}, nil
}

func (fakeCodeIntelligenceProvider) Status(_ string, _ int) (projectanalysis.IndexStatusResult, error) {
	return projectanalysis.IndexStatusResult{State: "fresh", Fresh: true, Complete: true, ArtifactVerified: true}, nil
}

func TestCodeIntelligenceProviderIsReplaceable(t *testing.T) {
	root := t.TempDir()
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "provider-test")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "provider-test", "")
	if err != nil {
		t.Fatal(err)
	}
	service.CodeIntelligence = fakeCodeIntelligenceProvider{}

	info, err := service.CodeIntelligenceInfo(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "fake_phpstorm" || !info.Capabilities.References || !info.Capabilities.Hierarchy {
		t.Fatalf("provider info=%+v", info)
	}

	query, err := service.ProjectIndexQuery(environment.ID, projectanalysis.IndexQuery{Query: "Foo"})
	if err != nil {
		t.Fatal(err)
	}
	if query.Returned != 1 || query.IndexPath != "provider://fake" || query.Matches[0].QualifiedName != "App\\Foo" {
		t.Fatalf("provider query=%+v", query)
	}

	status, err := service.ProjectIndexStatus(environment.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "fresh" || !status.Fresh {
		t.Fatalf("provider status=%+v", status)
	}
}
