package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/skill"
)

func TestDiscoverAndReadConfiguredSkillArtifactAndSupport(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	support := filepath.Join(t.TempDir(), "gsd-core")
	if err := os.MkdirAll(filepath.Join(root, "gsd-next"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(support, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "gsd-next", "SKILL.md")
	workflow := filepath.Join(support, "workflows", "smart-entry.md")
	if err := os.WriteFile(artifact, []byte("# gsd-next\n@"+filepath.ToSlash(workflow)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflow, []byte("# smart entry\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := skill.Discover(root, []string{support}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "gsd-next" || entries[0].SourceID == "" || entries[0].RelativeArtifactPath != "gsd-next/skill.md" {
		t.Fatalf("discovered entries = %+v", entries)
	}
	artifactContent, err := skill.Read(entries[0], "", 0)
	if err != nil || !strings.Contains(artifactContent.Content, "# gsd-next") {
		t.Fatalf("read artifact = %+v err=%v", artifactContent, err)
	}
	supportContent, err := skill.Read(entries[0], workflow, 0)
	if err != nil || !strings.Contains(supportContent.Content, "# smart entry") {
		t.Fatalf("read support = %+v err=%v", supportContent, err)
	}
}

func TestSkillReadRejectsUnconfiguredPathAndEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(root, "gsd-next"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "gsd-next", "SKILL.md")
	if err := os.WriteFile(artifact, []byte("# gsd-next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := skill.Discover(root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := skill.Read(entries[0], outside, 0); err == nil || !strings.Contains(err.Error(), "outside configured artifact/support roots") {
		t.Fatalf("outside read error = %v", err)
	}
	if _, err := skill.Discover(filepath.Dir(outside), nil, false); err == nil || !strings.Contains(err.Error(), "contains no SKILL.md") {
		t.Fatalf("unconfigured/non-skill root discovery error = %v", err)
	}
}

func TestBrokenSkillArtifactFailsLocallyAfterDiscovery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(root, "gsd-next"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "gsd-next", "SKILL.md")
	if err := os.WriteFile(artifact, []byte("# gsd-next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := skill.Discover(root, nil, false)
	if err != nil || len(entries) != 1 {
		t.Fatalf("discover entries=%+v err=%v", entries, err)
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if _, err := skill.Read(entries[0], "", 0); err == nil {
		t.Fatal("missing Skill artifact must fail at read time")
	}
}

func TestSourceArtifactSkillIDDependsOnSourceAndRelativeArtifactPath(t *testing.T) {
	if skill.SourceArtifactID("source-a", "GSD-Next/SKILL.md") != skill.SourceArtifactID("source-a", "gsd-next/skill.md") {
		t.Fatal("source/artifact Skill ID must normalize relative artifact path case")
	}
	if skill.SourceArtifactID("source-a", "gsd-next/SKILL.md") == skill.SourceArtifactID("source-b", "gsd-next/SKILL.md") {
		t.Fatal("source/artifact Skill ID must include source identity")
	}
}
