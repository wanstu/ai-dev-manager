//go:build windows

package sandbox

import (
	"io"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWaitWindowsPipeCopiesCancelsBlockedRead(t *testing.T) {
	var readHandle, writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, nil, 0); err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(writeHandle)

	copy := startWindowsPipeCopy(readHandle, io.Discard)
	copies := []*windowsPipeCopy{copy}
	done := windowsPipeCopiesDone(copies)
	defer closeWindowsPipeCopies(copies)

	started := time.Now()
	if err := waitWindowsPipeCopiesWithGrace(done, copies, 20*time.Millisecond, 200*time.Millisecond); err != nil {
		t.Fatalf("blocked pipe copy did not cancel: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("blocked pipe cancellation took too long: %s", elapsed)
	}
}

func TestWindowsEnvironmentBlock(t *testing.T) {
	block, err := windowsEnvironmentBlock([]string{"A=1", "B=2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(block) < 2 || block[len(block)-1] != 0 || block[len(block)-2] != 0 {
		t.Fatalf("environment block is not double-NUL terminated: %v", block)
	}
	if _, err := windowsEnvironmentBlock([]string{"A=bad\x00value"}); err == nil {
		t.Fatal("environment block accepted NUL")
	}
}
