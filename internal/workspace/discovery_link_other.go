//go:build !windows

package workspace

import "os"

func discoveryLink(info os.FileInfo) bool { return info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 }
