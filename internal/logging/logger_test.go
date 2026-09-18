package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerSeparatesLevelsAndRedactsSensitiveFields(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, 1024*1024, 3)
	if err := logger.Log(LevelInfo, "gateway.start", map[string]string{
		"listen":        "127.0.0.1:43137",
		"api_key":       "must-not-be-written",
		"Authorization": "Bearer must-not-be-written",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "info.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "must-not-be-written") {
		t.Fatalf("sensitive field leaked to log: %s", text)
	}
	if !strings.Contains(text, "gateway.start") || !strings.Contains(text, "127.0.0.1:43137") {
		t.Fatalf("expected safe metadata in log: %s", text)
	}
}

func TestLoggerRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, 120, 2)
	for i := 0; i < 8; i++ {
		if err := logger.Log(LevelWarn, "event", map[string]string{"detail": strings.Repeat("x", 80)}); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "warn-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 || len(matches) > 2 {
		t.Fatalf("expected 1..2 rotated files, got %d: %v", len(matches), matches)
	}
}
