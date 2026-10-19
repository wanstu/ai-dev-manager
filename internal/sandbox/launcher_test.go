package sandbox

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLaunchRequestValidate(t *testing.T) {
	base := t.TempDir()
	valid := LaunchRequest{
		Executable: filepath.Join(base, "tool.exe"),
		Cwd:        base,
		ProfileSID: "S-1-15-2-1",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	for name, mutate := range map[string]func(*LaunchRequest){
		"missing executable": func(v *LaunchRequest) { v.Executable = "" },
		"relative executable": func(v *LaunchRequest) { v.Executable = "tool.exe" },
		"relative cwd": func(v *LaunchRequest) { v.Cwd = "." },
		"missing SID": func(v *LaunchRequest) { v.ProfileSID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestLaunchUnsupportedPlatformReturnsError(t *testing.T) {
	if PlatformSupport().Platform == "windows" {
		t.Skip("non-Windows behavior only")
	}
	_, err := Launch(context.Background(), LaunchRequest{
		Executable: filepath.Join(t.TempDir(), "tool"),
		Cwd:        t.TempDir(),
		ProfileSID: "unused",
	})
	if err == nil {
		t.Fatal("unsupported platform launch unexpectedly succeeded")
	}
}
