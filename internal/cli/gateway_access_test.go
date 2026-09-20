package cli

import (
	"encoding/hex"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayAccessKeyArgGenerate(t *testing.T) {
	value, generated, err := gatewayAccessKeyArg(
		"gateway access set-admin-key",
		[]string{"--generate"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !generated {
		t.Fatal("generated = false; want true")
	}
	if len(value) != 64 {
		t.Fatalf("generated key length = %d; want 64", len(value))
	}
	if _, err := hex.DecodeString(value); err != nil {
		t.Fatalf("generated key is not hex: %v", err)
	}
}

func TestGatewayAccessKeyArgManualKeyIsNotGenerated(t *testing.T) {
	value, generated, err := gatewayAccessKeyArg(
		"gateway access set-agent-key",
		[]string{"--key", "manual-key-value-1234"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if generated {
		t.Fatal("generated = true; want false")
	}
	if value != "manual-key-value-1234" {
		t.Fatalf("value = %q", value)
	}
}

func TestGatewayAccessKeyArgRejectsGenerateWithKey(t *testing.T) {
	_, _, err := gatewayAccessKeyArg(
		"gateway access set-agent-key",
		[]string{"--key", "manual-key-value-1234", "--generate"},
	)
	if err == nil || !strings.Contains(err.Error(), "不能同时使用") {
		t.Fatalf("expected mutually-exclusive flag error, got %v", err)
	}
}

func TestGatewayAccessKeyArgDoesNotReadEnvironment(t *testing.T) {
	t.Setenv(admLegacyAdminAPIKeyEnv, "legacy-server-input-must-not-be-read")
	_, _, err := gatewayAccessKeyArg("gateway access set-admin-key", nil)
	if err == nil || !strings.Contains(err.Error(), "--key 或 --generate") {
		t.Fatalf("expected explicit server-key input error, got %v", err)
	}
}

func TestWriteGatewayAccessKeyResultOnlyRevealsGeneratedKey(t *testing.T) {
	status := app.GatewayAccessStatus{AdminAPIKeyConfigured: true}

	generatedOutput := captureStdout(t, func() {
		if err := writeGatewayAccessKeyResult(status, "generated-secret-value", true); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(generatedOutput, `"generated_key": "generated-secret-value"`) {
		t.Fatalf("generated output did not reveal one-time key: %s", generatedOutput)
	}

	manualOutput := captureStdout(t, func() {
		if err := writeGatewayAccessKeyResult(status, "manual-secret-value", false); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(manualOutput, "manual-secret-value") || strings.Contains(manualOutput, "generated_key") {
		t.Fatalf("manual key leaked in output: %s", manualOutput)
	}
}
