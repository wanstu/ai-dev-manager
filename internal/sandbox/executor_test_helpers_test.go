package sandbox

import "os"

func mkdirFixture(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeExecutableFixture(path string) error {
	return os.WriteFile(path, []byte("fixture"), 0o755)
}
