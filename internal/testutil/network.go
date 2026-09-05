package testutil

import (
	"os"
	"testing"
)

const NetworkAcceptanceEnv = "ADM_NETWORK_ACCEPTANCE"
const DaemonExecutableEnv = "ADM_TEST_DAEMON_EXECUTABLE"

// RequireNetworkAcceptance keeps normal unit/integration runs free of real TCP
// listeners. Real network acceptance is opt-in so unattended development on
// Windows does not trigger a firewall prompt for every temporary go test exe.
func RequireNetworkAcceptance(t testing.TB) {
	t.Helper()
	if os.Getenv(NetworkAcceptanceEnv) != "1" {
		t.Skip("real network acceptance disabled; set ADM_NETWORK_ACCEPTANCE=1 to enable")
	}
}
