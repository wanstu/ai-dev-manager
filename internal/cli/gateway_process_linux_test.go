//go:build linux

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxListeningSocketInodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tcp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:A881 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 0100007F:A882 00000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 67890 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	inodes, err := linuxListeningSocketInodes(path, 0xA881)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inodes["12345"]; !ok {
		t.Fatalf("expected LISTEN inode 12345, got %#v", inodes)
	}
	if _, ok := inodes["67890"]; ok {
		t.Fatalf("non-LISTEN inode must not be returned: %#v", inodes)
	}
}
