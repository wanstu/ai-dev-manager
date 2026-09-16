package gateway

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const verifierAcceptanceFilename = "verifier_acceptance_test.go"

func TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy(t *testing.T) {
	sourceRoot := repositoryRootForVerifierAcceptance(t)
	copyRoot := filepath.Join(t.TempDir(), "repo-copy")
	copyRepositoryForVerifierAcceptance(t, sourceRoot, copyRoot)
	assertVerifierAcceptanceCopyIsNonRecursive(t, copyRoot)

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(copyRoot, "verifier-http-acceptance")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "non-git-verifier", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable("go"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "verifier-http-owner"); err != nil {
		t.Fatal(err)
	}

	httpServer := httptest.NewServer(NewHTTPHandler(service))
	defer httpServer.Close()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-verifier-acceptance", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect verifier acceptance HTTP Gateway: %v", err)
	}
	defer session.Close()

	// V-R1: one trivial configured verifier runs successfully in a non-Git Environment.
	trivial, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:           "go-version",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     "go",
		Args:           []string{"version"},
		TimeoutSeconds: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	trivialResult := callVerifierAcceptanceHTTP(t, ctx, session, env.ID, trivial.ID, 4096)
	if trivialResult.ID != trivial.ID || trivialResult.Status != verifier.StatusPassed || trivialResult.ExitCode != 0 || trivialResult.TimedOut {
		t.Fatalf("V-R1 trivial verifier result = %+v", trivialResult)
	}

	// V-R2: the product capability invokes this repository's own go test ./... through the verifier.
	// The copy assertion above runs before this call, so the inner suite cannot re-enter this acceptance test.
	fullSuite, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:           "v2-go-test-all",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     "go",
		Args:           []string{"test", "./..."},
		TimeoutSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	fullResult := callVerifierAcceptanceHTTP(t, ctx, session, env.ID, fullSuite.ID, 16384)
	if fullResult.ID != fullSuite.ID || fullResult.Status != verifier.StatusPassed || fullResult.ExitCode != 0 || fullResult.TimedOut {
		t.Fatalf("V-R2 go test ./... verifier result = %+v", fullResult)
	}
	if len(fullResult.Stdout) > 16384 || len(fullResult.Stderr) > 16384 {
		t.Fatalf("V-R2 output exceeded bound: stdout=%d stderr=%d", len(fullResult.Stdout), len(fullResult.Stderr))
	}

	// V-R3 fixtures are created only after V-R2 succeeds so they do not alter the repository suite V-R2 proves.
	failFixture := filepath.Join(copyRoot, "verifier_fail_fixture")
	if err := os.MkdirAll(failFixture, 0o755); err != nil {
		t.Fatal(err)
	}
	failSource := `package verifier_fail_fixture

import (
    "fmt"
    "strings"
    "testing"
)

func TestDeliberateVerifierFailure(t *testing.T) {
    fmt.Print(strings.Repeat("FAIL-OUTPUT-", 512))
    t.Fatal("deliberate verifier failure")
}
`
	if err := os.WriteFile(filepath.Join(failFixture, "fail_test.go"), []byte(failSource), 0o644); err != nil {
		t.Fatal(err)
	}
	failVerifier, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:           "deliberate-fail",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     "go",
		Args:           []string{"test", "./verifier_fail_fixture"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	failedResult := callVerifierAcceptanceHTTP(t, ctx, session, env.ID, failVerifier.ID, 256)
	if failedResult.ID != failVerifier.ID || failedResult.Status != verifier.StatusFailed || failedResult.ExitCode == 0 || failedResult.TimedOut {
		t.Fatalf("V-R3 deliberate failure result = %+v", failedResult)
	}
	if len(failedResult.Stdout) > 256 || len(failedResult.Stderr) > 256 {
		t.Fatalf("V-R3 failure output exceeded bound: stdout=%d stderr=%d", len(failedResult.Stdout), len(failedResult.Stderr))
	}

	timeoutFixture := filepath.Join(copyRoot, "verifier_timeout_fixture")
	if err := os.MkdirAll(timeoutFixture, 0o755); err != nil {
		t.Fatal(err)
	}
	timeoutSource := `package verifier_timeout_fixture

import (
    "fmt"
    "strings"
    "testing"
    "time"
)

func TestDeliberateVerifierTimeout(t *testing.T) {
    fmt.Print(strings.Repeat("TIMEOUT-OUTPUT-", 256))
    time.Sleep(5 * time.Second)
}
`
	if err := os.WriteFile(filepath.Join(timeoutFixture, "timeout_test.go"), []byte(timeoutSource), 0o644); err != nil {
		t.Fatal(err)
	}
	timeoutVerifier, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:           "deliberate-timeout",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     "go",
		Args:           []string{"test", "./verifier_timeout_fixture"},
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	timedOutResult := callVerifierAcceptanceHTTP(t, ctx, session, env.ID, timeoutVerifier.ID, 256)
	if timedOutResult.ID != timeoutVerifier.ID || timedOutResult.Status != verifier.StatusFailed || !timedOutResult.TimedOut || timedOutResult.ExitCode != -1 {
		t.Fatalf("V-R3 timeout result = %+v", timedOutResult)
	}
	if len(timedOutResult.Stdout) > 256 || len(timedOutResult.Stderr) > 256 {
		t.Fatalf("V-R3 timeout output exceeded bound: stdout=%d stderr=%d", len(timedOutResult.Stdout), len(timedOutResult.Stderr))
	}
}

func callVerifierAcceptanceHTTP(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, verifierID string, maxOutputBytes int) verifier.Result {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "environment_verifier_run",
		Arguments: map[string]any{
			"environment_id":   environmentID,
			"writer_owner":     "verifier-http-owner",
			"verifier_id":      verifierID,
			"max_output_bytes": maxOutputBytes,
		},
	})
	if err != nil {
		t.Fatalf("environment_verifier_run %s transport error: %v", verifierID, err)
	}
	if result.IsError {
		t.Fatalf("environment_verifier_run %s returned tool error: %s", verifierID, toolText(t, result))
	}
	var envelope struct {
		Result verifier.Result `json:"result"`
	}
	if err := json.Unmarshal([]byte(toolText(t, result)), &envelope); err != nil {
		t.Fatalf("decode environment_verifier_run %s result: %v; body=%s", verifierID, err, toolText(t, result))
	}
	return envelope.Result
}

func repositoryRootForVerifierAcceptance(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	current := cwd
	for {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatalf("could not locate repository root from %s", cwd)
		}
		current = parent
	}
}

func copyRepositoryForVerifierAcceptance(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if entry.IsDir() {
			if shouldSkipVerifierAcceptanceDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destinationRoot, rel), 0o755)
		}
		if relSlash == "internal/gateway/"+verifierAcceptanceFilename {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if shouldSkipVerifierAcceptanceFile(entry.Name()) {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			_ = source.Close()
			return err
		}
		destinationPath := filepath.Join(destinationRoot, rel)
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
			_ = source.Close()
			return err
		}
		destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			_ = source.Close()
			return err
		}
		_, copyErr := io.Copy(destination, source)
		sourceCloseErr := source.Close()
		destinationCloseErr := destination.Close()
		if copyErr != nil {
			return copyErr
		}
		if sourceCloseErr != nil {
			return sourceCloseErr
		}
		return destinationCloseErr
	})
	if err != nil {
		t.Fatalf("copy repository for verifier acceptance: %v", err)
	}
}

func shouldSkipVerifierAcceptanceDirectory(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".planning", ".gitnexus", "node_modules", "dist", "build", "coverage", ".cache", ".tmp", "tmp":
		return true
	default:
		return false
	}
}

func shouldSkipVerifierAcceptanceFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == ".git" || strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".test") || strings.HasSuffix(lower, ".out")
}

func assertVerifierAcceptanceCopyIsNonRecursive(t *testing.T, copyRoot string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(copyRoot, ".git")); !os.IsNotExist(err) {
		t.Fatalf("verifier acceptance copy must be non-Git; .git stat err=%v", err)
	}
	acceptancePath := filepath.Join(copyRoot, "internal", "gateway", verifierAcceptanceFilename)
	if _, err := os.Stat(acceptancePath); !os.IsNotExist(err) {
		t.Fatalf("recursive verifier acceptance file exists in copy: %s", acceptancePath)
	}
	var found []string
	if err := filepath.WalkDir(copyRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), verifierAcceptanceFilename) {
			found = append(found, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("verifier acceptance copy still contains self-recursive acceptance file(s): %v", found)
	}
	if _, err := os.Stat(filepath.Join(copyRoot, "go.mod")); err != nil {
		t.Fatalf("verifier acceptance copy missing go.mod: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(copyRoot, "go.mod")); err != nil || !strings.Contains(string(data), "module ai-dev-manager-v2") {
		t.Fatalf("verifier acceptance copy is not the V2 module: err=%v data=%q", err, data)
	}
}
