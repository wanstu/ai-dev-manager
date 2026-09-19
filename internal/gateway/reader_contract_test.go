package gateway

import (
	"strings"
	"testing"
)

func TestGatewayAgentInstructionsKeepReadAccessLeaseFree(t *testing.T) {
	if !strings.Contains(gatewayReaderContractNote, "implicitly readable and lease-free") ||
		!strings.Contains(gatewayReaderContractNote, "do not acquire a writer") {
		t.Fatalf("gateway reader contract note is incomplete: %s", gatewayReaderContractNote)
	}

	for _, required := range []string{
		"Environment-scoped inspection is implicitly readable",
		"require no reader lease and no writer lease",
		"Never acquire a writer merely to browse, inspect, search, review, or understand code",
		"acquire it only immediately before an operation whose tool contract explicitly requires writer_owner",
	} {
		if !strings.Contains(gatewayAgentInstructions, required) {
			t.Fatalf("gateway agent instructions missing reader contract %q: %s", required, gatewayAgentInstructions)
		}
	}
}
