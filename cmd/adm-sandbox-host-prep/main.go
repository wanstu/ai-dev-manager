package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ai-dev-manager-v2/internal/sandbox"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "status", "verify-null-device":
		report := sandbox.InspectHostPreparation()
		writeJSON(report)
		if !report.NullDevicePrepared && report.Platform == "windows" {
			os.Exit(1)
		}
	case "prepare-null-device":
		if err := sandbox.PrepareNullDeviceHost(); err != nil {
			fmt.Fprintln(os.Stderr, "host preparation failed:", err)
			os.Exit(1)
		}
		report := sandbox.InspectHostPreparation()
		writeJSON(report)
		if !report.NullDevicePrepared {
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: adm-sandbox-host-prep <status|verify-null-device|prepare-null-device>")
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(1)
	}
}
