//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestOtherDesktopProcessIDsOnlyIgnoresSelf(t *testing.T) {
	entries := []desktopProcessEntry{
		{pid: 120, name: "adm-desktop.exe"},
		{pid: 21, name: "ADM-DESKTOP.EXE"},
		{pid: 31, name: "adm-desktop.exe"},
		{pid: 41, name: "adm.exe"},
		{pid: 51, name: "adm-desktop-helper.exe"},
	}
	got := otherDesktopProcessIDs(entries, 120, 0)
	if want := []uint32{21, 31}; !reflect.DeepEqual(got, want) {
		t.Fatalf("otherDesktopProcessIDs=%v, want %v", got, want)
	}
	if got := otherDesktopProcessIDs(entries, 120, 31); !reflect.DeepEqual(got, []uint32{21}) {
		t.Fatalf("known Gateway PID must be the only extra exception: %v", got)
	}
	if got := otherDesktopProcessIDs(nil, 120, 0); len(got) != 0 {
		t.Fatalf("no processes must not block install: %v", got)
	}
}
