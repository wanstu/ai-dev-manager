package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/verifier"
)

func TestEnvironmentContextBundlePlainNonGitNeedsNoOptionalCapabilityOrWriter(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.txt"), []byte("ordinary project content"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	ws, err := service.Workspaces.Add(root, "plain-workspace")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "plain-environment", "")
	if err != nil {
		t.Fatal(err)
	}
	before := mustReadBytes(t, statePath)

	bundle, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Environment.EnvironmentID != env.ID || bundle.Environment.WorkspaceID != ws.ID || bundle.Environment.Root != env.Root {
		t.Fatalf("identity mismatch: %+v", bundle.Environment)
	}
	if bundle.Environment.WriterPresent || bundle.Environment.WriterExpiresAt != nil {
		t.Fatalf("bundle invented writer state: %+v", bundle.Environment)
	}
	if bundle.Limits != (model.EnvironmentContextLimits{MaxDepth: 3, MaxEntries: 1000, MaxDigestEntries: 80, MaxOutputBytes: 65536}) {
		t.Fatalf("unexpected defaults: %+v", bundle.Limits)
	}
	if bundle.Tree.ScanPath != "." || len(bundle.Tree.Digest) == 0 {
		t.Fatalf("missing bounded tree evidence: %+v", bundle.Tree)
	}
	if got := contextGuidanceState(t, bundle, "files.inspect"); got != model.CapabilityStateAvailable {
		t.Fatalf("files.inspect state=%s", got)
	}
	mutation := contextGuidance(t, bundle, "files.mutate")
	if mutation.State != model.CapabilityStateAvailable || !mutation.RequiresWriter {
		t.Fatalf("mutation guidance=%+v", mutation)
	}
	if len(bundle.MCPs) != 0 || len(bundle.Skills) != 0 || len(bundle.Verifiers) != 0 {
		t.Fatalf("unexpected optional summaries: mcps=%+v skills=%+v verifiers=%+v", bundle.MCPs, bundle.Skills, bundle.Verifiers)
	}
	if !hasContextCapability(bundle.AvailableCapabilities, runtime.CapabilityRead, model.CapabilityStateAvailable) {
		t.Fatalf("read capability missing: %+v", bundle.AvailableCapabilities)
	}
	gitFact := contextCapabilityByKey(t, bundle.CapabilityIssues, runtime.CapabilityGitStatus)
	if gitFact.State != model.CapabilityStateDegraded || gitFact.ReasonCode != "not_observed" {
		t.Fatalf("context bundle must passively report Git instead of executing it: %+v", gitFact)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > bundle.Limits.MaxOutputBytes {
		t.Fatalf("bundle bytes=%d limit=%d", len(encoded), bundle.Limits.MaxOutputBytes)
	}
	for _, emptyArray := range []string{`"mcps":[]`, `"skills":[]`, `"verifiers":[]`} {
		if !strings.Contains(string(encoded), emptyArray) {
			t.Fatalf("success evidence must use explicit empty arrays; missing %s in %s", emptyArray, encoded)
		}
	}
	after := mustReadBytes(t, statePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("context bundle changed persisted desired state")
	}
	refreshed, err := service.Environments.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Writer != nil {
		t.Fatalf("context bundle acquired writer: %+v", refreshed.Writer)
	}
}

func TestEnvironmentContextBundleSummariesAreSafeAndDoNotExecute(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	ws, err := service.Workspaces.Add(root, "safe-context")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "safe-context-env", "")
	if err != nil {
		t.Fatal(err)
	}

	const mcpSecret = "MCP_CONTEXT_SECRET_VALUE"
	var mcpRequests atomic.Int32
	mcpServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		mcpRequests.Add(1)
	}))
	defer mcpServer.Close()
	mcpEntry, err := service.MCPs.AddMCP("safe-mcp", mcpServer.URL+"/"+mcpSecret, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(env.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}

	const skillSecret = "SKILL_CONTEXT_SECRET_VALUE"
	skillRoot := filepath.Join(t.TempDir(), "skill-source")
	skillDir := filepath.Join(skillRoot, "safe-skill")
	supportRoot := filepath.Join(t.TempDir(), "skill-support")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(supportRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# safe skill\n"+skillSecret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, err := service.Skills.AddSkillRoot(skillRoot, []string{supportRoot}, false)
	if err != nil || len(skills) != 1 {
		t.Fatalf("skills=%+v err=%v", skills, err)
	}
	if _, err := service.SetEnvironmentSkill(env.ID, skills[0].ID, true); err != nil {
		t.Fatal(err)
	}

	const globalSecret = "GLOBAL_MEMORY_CONTEXT_SECRET"
	const privateSecret = "PRIVATE_MEMORY_CONTEXT_SECRET"
	if err := service.Memory.GlobalWrite("context-global", globalSecret); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(env.ID, "context-private", privateSecret); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(executable); err != nil {
		t.Fatal(err)
	}
	sideEffectPath := filepath.Join(t.TempDir(), "context-verifier-ran.txt")
	t.Setenv("ADM_CONTEXT_VERIFIER_SIDE_EFFECT", sideEffectPath)
	verifierDef, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:       "safe-verifier",
		Kind:       verifier.KindCustom,
		Enabled:    true,
		Executable: executable,
		Args:       []string{"-test.run=TestEnvironmentContextVerifierSideEffectHelper$"},
	})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := service.Environments.AcquireWriter(env.ID, "context-writer")
	if err != nil {
		t.Fatal(err)
	}
	before := mustReadBytes(t, statePath)

	bundle, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := mcpRequests.Load(); got != 0 {
		t.Fatalf("context bundle connected to MCP during passive composition: requests=%d", got)
	}
	if bundle.Environment.PrivateMemoryCount != 1 {
		t.Fatalf("private memory count=%d", bundle.Environment.PrivateMemoryCount)
	}
	if !bundle.Environment.WriterPresent || bundle.Environment.WriterExpiresAt == nil || !bundle.Environment.WriterExpiresAt.Equal(writer.Writer.ExpiresAt) {
		t.Fatalf("writer observation mismatch: bundle=%+v writer=%+v", bundle.Environment, writer.Writer)
	}
	if len(bundle.MCPs) != 1 || bundle.MCPs[0].ID != mcpEntry.ID || bundle.MCPs[0].Name != mcpEntry.Name {
		t.Fatalf("mcp summary=%+v", bundle.MCPs)
	}
	if bundle.MCPs[0].ObservationState != "not_observed" || bundle.MCPs[0].ToolInventoryObserved || len(bundle.MCPs[0].ToolNames) != 0 {
		t.Fatalf("Core bundle invented owner observation: %+v", bundle.MCPs[0])
	}
	if len(bundle.Skills) != 1 || bundle.Skills[0].ID != skills[0].ID || bundle.Skills[0].Name != skills[0].Name || bundle.Skills[0].SupportRootCount != 1 {
		t.Fatalf("skill summary=%+v", bundle.Skills)
	}
	if len(bundle.Verifiers) != 1 || bundle.Verifiers[0].ID != verifierDef.ID || bundle.Verifiers[0].Name != "safe-verifier" || bundle.Verifiers[0].State != model.CapabilityStateAvailable {
		t.Fatalf("verifier summary=%+v", bundle.Verifiers)
	}
	verifierGuidance := contextGuidance(t, bundle, "verifier.run")
	if !strings.Contains(verifierGuidance.Message, "environment_verifier_run_start/status/cancel") || !strings.Contains(verifierGuidance.Message, "remains blocking") {
		t.Fatalf("verifier guidance did not advertise async lifecycle without auto-starting: %+v", verifierGuidance)
	}
	if _, err := os.Stat(sideEffectPath); !os.IsNotExist(err) {
		t.Fatalf("context bundle executed verifier, stat err=%v", err)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{mcpSecret, skillSecret, globalSecret, privateSecret, sideEffectPath, "-test.run=TestEnvironmentContextVerifierSideEffectHelper"} {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("context bundle leaked %q: %s", sentinel, encoded)
		}
	}
	if !reflect.DeepEqual(before, mustReadBytes(t, statePath)) {
		t.Fatal("context bundle changed persisted state")
	}
}

func TestEnvironmentContextBundleRejectsInvalidIdentityPathRootAndBudgets(t *testing.T) {
	root := t.TempDir()
	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "authority")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "authority-env", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnvironmentContextBundle(context.Background(), "missing", model.EnvironmentContextRequest{}); err == nil {
		t.Fatal("unknown Environment accepted")
	}
	for _, request := range []model.EnvironmentContextRequest{
		{Path: ".."},
		{Path: filepath.Join("..", filepath.Base(root))},
		{Path: filepath.Clean(root)},
		{MaxDepth: -1},
		{MaxDepth: 9},
		{MaxEntries: 10001},
		{MaxDigestEntries: 501},
		{MaxOutputBytes: 4095},
		{MaxOutputBytes: 262145},
	} {
		if _, err := service.EnvironmentContextBundle(context.Background(), env.ID, request); err == nil {
			t.Fatalf("invalid request accepted: %+v", request)
		}
	}
	missing := filepath.Join(root, "missing-root")
	if err := service.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == env.ID {
				state.Environments[i].Root = missing
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{}); err == nil {
		t.Fatal("missing Environment root accepted")
	}
}

func TestEnvironmentContextBundleRejectsTamperedManagedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("optional Git executable unavailable")
	}
	source := appTestGitRepo(t)
	service := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := service.Workspaces.Add(source, "managed-context")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateManagedWorktree(context.Background(), ws.ID, "managed-context", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	root := created.ManagedWorktree.Root
	defer appGitCommand(source, "worktree", "remove", "--force", root)
	if _, err := service.EnvironmentContextBundle(context.Background(), created.Environment.ID, model.EnvironmentContextRequest{}); err != nil {
		t.Fatal(err)
	}
	appGitRun(t, root, "checkout", "-b", "tampered-context")
	if _, err := service.EnvironmentContextBundle(context.Background(), created.Environment.ID, model.EnvironmentContextRequest{}); err == nil {
		t.Fatal("tampered managed root accepted")
	}
}

func TestEnvironmentContextBundleBudgetsSectionCapsAndStableOrdering(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 16; i++ {
		dir := filepath.Join(root, "directory-"+strings.Repeat(string(rune('a'+i)), 20))
		if err := os.MkdirAll(filepath.Join(dir, "nested", "deeper"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := New(statePath)
	ws, err := service.Workspaces.Add(root, "budgets")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "budget-env", "")
	if err != nil {
		t.Fatal(err)
	}

	depthBundle, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{MaxDepth: 1, MaxEntries: 1000, MaxDigestEntries: 500, MaxOutputBytes: 262144})
	if err != nil {
		t.Fatal(err)
	}
	if !containsContextString(depthBundle.Tree.StopReasons, "depth_limit") {
		t.Fatalf("depth limit missing: %+v", depthBundle.Tree)
	}
	entryBundle, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{MaxDepth: 8, MaxEntries: 1, MaxDigestEntries: 500, MaxOutputBytes: 262144})
	if err != nil {
		t.Fatal(err)
	}
	if !containsContextString(entryBundle.Tree.StopReasons, "entry_limit") || entryBundle.Tree.VisitedEntries > 1 {
		t.Fatalf("entry limit missing: %+v", entryBundle.Tree)
	}
	digestBundle, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{MaxDepth: 8, MaxEntries: 1000, MaxDigestEntries: 1, MaxOutputBytes: 262144})
	if err != nil {
		t.Fatal(err)
	}
	if !containsContextString(digestBundle.Tree.StopReasons, "digest_limit") || len(digestBundle.Tree.Digest) != 1 || digestBundle.Tree.OmittedDigestEntries == 0 {
		t.Fatalf("digest limit missing: %+v", digestBundle.Tree)
	}

	skillRoot := filepath.Join(t.TempDir(), "many-skills")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	mcps := make([]model.MCPDefinition, 0, 80)
	skills := make([]model.CatalogEntry, 0, 80)
	verifiers := make([]model.VerifierDefinition, 0, 80)
	mcpIDs := make([]string, 0, 80)
	skillIDs := make([]string, 0, 80)
	for i := 0; i < 80; i++ {
		id := fmtContextID("mcp", i)
		mcps = append(mcps, model.MCPDefinition{ID: id, Name: fmtContextID("mcp-name", i), Transport: catalog.MCPTransportStreamableHTTP, AuthMode: catalog.MCPAuthNone, Endpoint: "http://example.test/mcp/" + id})
		mcpIDs = append(mcpIDs, id)

		skillID := fmtContextID("skill", i)
		dir := filepath.Join(skillRoot, skillID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		artifact := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(artifact, []byte("# "+skillID+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		skills = append(skills, model.CatalogEntry{ID: skillID, SourceID: "source-many", Name: fmtContextID("skill-name", i), ArtifactPath: artifact, RelativeArtifactPath: filepath.ToSlash(filepath.Join(skillID, "SKILL.md")), SourceRoot: skillRoot})
		skillIDs = append(skillIDs, skillID)
		verifiers = append(verifiers, model.VerifierDefinition{ID: fmtContextID("verifier", i), Name: fmtContextID("verifier-name", i), Kind: verifier.KindCustom, Enabled: false, Executable: "never-run"})
	}
	if err := service.Store.Update(func(state *model.State) error {
		state.MCPs = append(state.MCPs, mcps...)
		state.Skills = append(state.Skills, skills...)
		for i := range state.Environments {
			if state.Environments[i].ID == env.ID {
				state.Environments[i].EnabledMCPIDs = append([]string{}, mcpIDs...)
				state.Environments[i].EnabledSkillIDs = append([]string{}, skillIDs...)
				state.Environments[i].Verifiers = append([]model.VerifierDefinition{}, verifiers...)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	capped, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{MaxOutputBytes: 262144})
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.MCPs) != maxContextMCPs || capped.Omissions.MCPs != 80-maxContextMCPs {
		t.Fatalf("MCP cap not applied: len=%d omissions=%+v", len(capped.MCPs), capped.Omissions)
	}
	if len(capped.Skills) != maxContextSkills || capped.Omissions.Skills != 80-maxContextSkills {
		t.Fatalf("Skill cap not applied: len=%d omissions=%+v", len(capped.Skills), capped.Omissions)
	}
	if len(capped.Verifiers) != maxContextVerifiers || capped.Omissions.Verifiers != 80-maxContextVerifiers {
		t.Fatalf("Verifier cap not applied: len=%d omissions=%+v", len(capped.Verifiers), capped.Omissions)
	}
	if len(capped.CapabilityIssues) != maxContextCapabilityIssues || capped.Omissions.CapabilityIssues == 0 {
		t.Fatalf("capability issue cap not applied: len=%d omissions=%+v", len(capped.CapabilityIssues), capped.Omissions)
	}
	if capped.Coverage != "partial" {
		t.Fatalf("section caps must make coverage partial: %+v", capped.CoverageReasons)
	}

	small, err := service.EnvironmentContextBundle(context.Background(), env.ID, model.EnvironmentContextRequest{MaxOutputBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(small)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 4096 {
		t.Fatalf("byte cap violated: %d", len(encoded))
	}
	if !containsContextString(small.CoverageReasons, "output_byte_limit") {
		t.Fatalf("output byte truncation not reported: %+v", small.CoverageReasons)
	}
	if small.Environment.EnvironmentID != env.ID || small.Environment.Root != env.Root {
		t.Fatalf("stable identity was truncated: %+v", small.Environment)
	}

	staticService := New(filepath.Join(t.TempDir(), "stable-state.json"))
	stableRoot := t.TempDir()
	stableWS, err := staticService.Workspaces.Add(stableRoot, "stable")
	if err != nil {
		t.Fatal(err)
	}
	stableEnv, err := staticService.Environments.Create(stableWS.ID, "stable-env", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := staticService.EnvironmentContextBundle(context.Background(), stableEnv.ID, model.EnvironmentContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := staticService.EnvironmentContextBundle(context.Background(), stableEnv.ID, model.EnvironmentContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	normalizeContextTimes(&first)
	normalizeContextTimes(&second)
	if !reflect.DeepEqual(first, second) {
		firstJSON, _ := json.Marshal(first)
		secondJSON, _ := json.Marshal(second)
		t.Fatalf("static bundle ordering changed\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}

func TestEnvironmentContextVerifierSideEffectHelper(t *testing.T) {
	path := os.Getenv("ADM_CONTEXT_VERIFIER_SIDE_EFFECT")
	if path == "" || !strings.Contains(strings.Join(os.Args, " "), "TestEnvironmentContextVerifierSideEffectHelper") {
		return
	}
	if err := os.WriteFile(path, []byte("verifier executed"), 0o644); err != nil {
		panic(err)
	}
}

func contextGuidance(t *testing.T, bundle model.EnvironmentContextBundle, operation string) model.EnvironmentContextGuidance {
	t.Helper()
	for _, item := range bundle.Guidance {
		if item.Operation == operation {
			return item
		}
	}
	t.Fatalf("missing guidance %q: %+v", operation, bundle.Guidance)
	return model.EnvironmentContextGuidance{}
}

func contextGuidanceState(t *testing.T, bundle model.EnvironmentContextBundle, operation string) model.CapabilityState {
	t.Helper()
	return contextGuidance(t, bundle, operation).State
}

func hasContextCapability(items []model.EnvironmentContextCapability, key string, state model.CapabilityState) bool {
	for _, item := range items {
		if item.Key == key && item.State == state {
			return true
		}
	}
	return false
}

func contextCapabilityByKey(t *testing.T, items []model.EnvironmentContextCapability, key string) model.EnvironmentContextCapability {
	t.Helper()
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	t.Fatalf("missing context capability %q: %+v", key, items)
	return model.EnvironmentContextCapability{}
}

func containsContextString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fmtContextID(prefix string, index int) string {
	return prefix + "-" + strings.Repeat("0", 3-len(fmtContextIndex(index))) + fmtContextIndex(index)
}

func fmtContextIndex(index int) string {
	if index >= 100 {
		return string(rune('0'+index/100)) + string(rune('0'+(index/10)%10)) + string(rune('0'+index%10))
	}
	if index >= 10 {
		return string(rune('0'+index/10)) + string(rune('0'+index%10))
	}
	return string(rune('0' + index))
}

func normalizeContextTimes(bundle *model.EnvironmentContextBundle) {
	bundle.GeneratedAt = time.Time{}
	bundle.Tree.ObservedAt = time.Time{}
	bundle.Environment.WriterExpiresAt = nil
	for i := range bundle.AvailableCapabilities {
		bundle.AvailableCapabilities[i].ObservedAt = nil
	}
	for i := range bundle.CapabilityIssues {
		bundle.CapabilityIssues[i].ObservedAt = nil
	}
	for i := range bundle.MCPs {
		bundle.MCPs[i].ObservedAt = nil
	}
}
