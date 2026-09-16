//go:build linux || darwin

package runtime

import (
	"net"
	"os"
	"testing"
)

func TestListeningTCPPortsObservesOwnedUnixProcessOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	ports, err := ListeningTCPPorts(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, observed := range ports {
		if observed == port {
			return
		}
	}
	t.Fatalf("ListeningTCPPorts(%d)=%v missing owned listener port %d", os.Getpid(), ports, port)
}
