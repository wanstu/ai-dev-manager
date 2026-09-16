package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"
)

const asyncVerifierTestWriter = "phase21-verifier-writer"

func TestVerifierRunTerminalStatesAndLiveOutput(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()

	definition := addAsyncVerifier(t, service, environmentID, "async-main", true, os.Args[0], "", 5)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "success")
	started, err := owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(started.ID, "vfrun_") || started.State != verifierRunRunning {
		t.Fatalf("unexpected start status: %+v", started)
	}
	succeeded := waitVerifierRunTerminal(t, owner, environmentID, started.ID, 5*time.Second)
	if succeeded.State != verifierRunSucceeded || succeeded.Result == nil || succeeded.Result.Status != verifier.StatusPassed || succeeded.Result.ExitCode != 0 || !strings.Contains(succeeded.Stdout, "phase21-success") {
		t.Fatalf("unexpected success status: %+v", succeeded)
	}

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "fail")
	started, err = owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	failed := waitVerifierRunTerminal(t, owner, environmentID, started.ID, 5*time.Second)
	if failed.State != verifierRunFailed || failed.Result == nil || failed.Result.Status != verifier.StatusFailed || failed.Result.ExitCode != 9 || !strings.Contains(failed.Stderr, "phase21-fail") {
		t.Fatalf("unexpected failure status: %+v", failed)
	}

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "stream-large")
	started, err = owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 64)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, statusErr := owner.VerifierRunStatus(environmentID, started.ID)
		if statusErr != nil {
			t.Fatal(statusErr)
		}
		if status.State == verifierRunRunning && status.StdoutTruncated && status.StderrTruncated {
			if len(status.Stdout) != 64 || len(status.Stderr) != 64 {
				t.Fatalf("bounded running output lengths = stdout %d stderr %d", len(status.Stdout), len(status.Stderr))
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, err := owner.VerifierRunStatus(environmentID, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != verifierRunRunning || !status.StdoutTruncated || !status.StderrTruncated {
		t.Fatalf("live output was not observable/bounded: %+v", status)
	}
	canceled, err := owner.CancelVerifierRun(environmentID, asyncVerifierTestWriter, started.ID)
	if err != nil || canceled.State != verifierRunCanceled {
		t.Fatalf("cancel bounded verifier: status=%+v err=%v", canceled, err)
	}
}

func TestVerifierRunConfiguredTimeoutUsesVerifierClassification(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	definition := addAsyncVerifier(t, service, environmentID, "async-timeout", true, os.Args[0], "", 1)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "timeout")

	started, err := owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	status := waitVerifierRunTerminal(t, owner, environmentID, started.ID, 5*time.Second)
	if status.State != verifierRunFailed || status.Result == nil || !status.Result.TimedOut || status.Result.Status != verifier.StatusFailed || status.Result.Summary != "verifier timed out" || status.ErrorKind != "timeout" {
		t.Fatalf("configured timeout lost verifier semantics: %+v", status)
	}
}

func TestVerifierRunAuthorityFailuresInstallNothing(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "success")

	disabled := addAsyncVerifier(t, service, environmentID, "disabled", false, os.Args[0], "", 5)
	forbidden := addAsyncVerifier(t, service, environmentID, "forbidden", true, "definitely-not-allowed", "", 5)
	escaped := addAsyncVerifier(t, service, environmentID, "escaped", true, os.Args[0], "../escape", 5)
	before, err := owner.ListVerifierRuns(environmentID)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		writer     string
		verifierID string
	}{
		{name: "missing verifier", writer: asyncVerifierTestWriter, verifierID: "vf_missing"},
		{name: "disabled verifier", writer: asyncVerifierTestWriter, verifierID: disabled.ID},
		{name: "wrong writer", writer: "wrong-writer", verifierID: escaped.ID},
		{name: "forbidden executable", writer: asyncVerifierTestWriter, verifierID: forbidden.ID},
		{name: "escaped cwd", writer: asyncVerifierTestWriter, verifierID: escaped.ID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := owner.StartVerifierRun(environmentID, tc.writer, tc.verifierID, 1024); err == nil {
				t.Fatal("unsafe verifier run start unexpectedly succeeded")
			}
		})
	}
	after, err := owner.ListVerifierRuns(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("failed authority checks installed verifier runs: before=%d after=%d", len(before), len(after))
	}
}

func TestVerifierRunCancelAndWriterHeartbeatFailure(t *testing.T) {
	service, environmentID, _ := asyncVerifierTestService(t)
	owner := newRuntimeOwner(service)
	defer owner.Close()
	definition := addAsyncVerifier(t, service, environmentID, "async-long", true, os.Args[0], "", 30)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "long")

	started, err := owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	waitVerifierRunOutput(t, owner, environmentID, started.ID, "phase21-long-ready")
	if _, err := owner.CancelVerifierRun(environmentID, "wrong-writer", started.ID); err == nil {
		t.Fatal("wrong writer canceled verifier run")
	}
	stillRunning, err := owner.VerifierRunStatus(environmentID, started.ID)
	if err != nil || stillRunning.State != verifierRunRunning {
		t.Fatalf("wrong writer changed verifier run: status=%+v err=%v", stillRunning, err)
	}
	canceled, err := owner.CancelVerifierRun(environmentID, asyncVerifierTestWriter, started.ID)
	if err != nil || canceled.State != verifierRunCanceled {
		t.Fatalf("matching writer cancel: status=%+v err=%v", canceled, err)
	}

	owner.verifierHeartbeatInterval = func(time.Duration) time.Duration { return 10 * time.Millisecond }
	started, err = owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	waitVerifierRunOutput(t, owner, environmentID, started.ID, "phase21-long-ready")
	if _, err := service.Environments.ReleaseWriter(environmentID, asyncVerifierTestWriter, true); err != nil {
		t.Fatal(err)
	}
	heartbeatCanceled := waitVerifierRunTerminal(t, owner, environmentID, started.ID, 5*time.Second)
	if heartbeatCanceled.State != verifierRunCanceled || heartbeatCanceled.ErrorKind != "writer_heartbeat_failed" {
		t.Fatalf("writer heartbeat failure was not observable: %+v", heartbeatCanceled)
	}
}

func TestVerifierRunOwnerAndEnvironmentCleanupAndNoPersistence(t *testing.T) {
	service, environmentID, statePath := asyncVerifierTestService(t)
	definition := addAsyncVerifier(t, service, environmentID, "async-cleanup", true, os.Args[0], "", 30)
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "long")

	owner := newRuntimeOwner(service)
	started, err := owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	waitVerifierRunOutput(t, owner, environmentID, started.ID, "phase21-long-ready")
	owner.DropEnvironment(environmentID)
	if _, err := owner.VerifierRunStatus(environmentID, started.ID); err == nil {
		t.Fatalf("Environment drop retained verifier run %q", started.ID)
	}

	started, err = owner.StartVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	waitVerifierRunOutput(t, owner, environmentID, started.ID, "phase21-long-ready")
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "vfrun_") || strings.Contains(string(data), "verifier_runs") {
		t.Fatalf("owner-local verifier observation leaked into persisted state: %s", data)
	}
	freshOwner := newRuntimeOwner(service)
	defer freshOwner.Close()
	listed, err := freshOwner.ListVerifierRuns(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("fresh owner resurrected verifier runs: %+v", listed)
	}
	if _, err := freshOwner.VerifierRunStatus(environmentID, started.ID); err == nil {
		t.Fatalf("fresh owner accepted old verifier run id %q", started.ID)
	}
}

func asyncVerifierTestService(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	workspace, err := service.Workspaces.Add(root, "phase21-verifier")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase21-verifier", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, asyncVerifierTestWriter); err != nil {
		t.Fatal(err)
	}
	return service, environment.ID, statePath
}

func addAsyncVerifier(t *testing.T, service *app.Service, environmentID, name string, enabled bool, executable, cwd string, timeoutSeconds int64) model.VerifierDefinition {
	t.Helper()
	definition, err := service.AddVerifier(environmentID, model.VerifierDefinition{
		Name:           name,
		Kind:           verifier.KindTest,
		Enabled:        enabled,
		Executable:     executable,
		Args:           []string{"-test.run=^TestAsyncVerifierCommandHelper$"},
		Cwd:            cwd,
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func waitVerifierRunTerminal(t *testing.T, owner *runtimeOwner, environmentID, runID string, timeout time.Duration) verifierRunStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := owner.VerifierRunStatus(environmentID, runID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != verifierRunRunning {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verifier run %s did not reach terminal state", runID)
	return verifierRunStatus{}
}

func waitVerifierRunOutput(t *testing.T, owner *runtimeOwner, environmentID, runID, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := owner.VerifierRunStatus(environmentID, runID)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(status.Stdout, marker) {
			return
		}
		if status.State != verifierRunRunning {
			t.Fatalf("verifier run %s ended before output marker %q: %+v", runID, marker, status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verifier run %s did not emit %q", runID, marker)
}

func TestAsyncVerifierCommandHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_ASYNC_VERIFIER_HELPER") != "1" {
		return
	}
	switch os.Getenv("ADM_TEST_ASYNC_VERIFIER_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "phase21-fail")
		os.Exit(9)
	case "timeout":
		fmt.Fprintln(os.Stdout, "phase21-timeout-ready")
		time.Sleep(3 * time.Second)
	case "stream-large":
		fmt.Fprint(os.Stdout, "phase21-stream "+strings.Repeat("x", 4096))
		fmt.Fprint(os.Stderr, "phase21-stream-err "+strings.Repeat("y", 4096))
		time.Sleep(3 * time.Second)
	case "long":
		fmt.Fprintln(os.Stdout, "phase21-long-ready")
		time.Sleep(30 * time.Second)
	default:
		fmt.Fprintln(os.Stdout, "phase21-success")
	}
}
