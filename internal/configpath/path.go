package configpath

import (
	"path/filepath"

	kitpaths "github.com/wanstu/wails-desktop-kit/paths"
)

const DirName = "adm"

func Dir() (string, error) {
	return kitpaths.ConfigDir(DirName)
}

func File(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
