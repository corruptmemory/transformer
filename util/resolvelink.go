package util

import (
	"os"
	"path/filepath"
	"strings"
)

type LStatFileNotFoundError struct {
	Path string
}

func (L *LStatFileNotFoundError) Error() string {
	return "lstat " + L.Path + ": no such file or directory"
}

func ResolveSymlink(path string) (string, error) {
	for {
		info, err := os.Lstat(path)
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				return "", &LStatFileNotFoundError{Path: path}
			}
			return "", err
		}

		// Return if it's not a symbolic link
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}

		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}

		// Handle relative paths
		if !filepath.IsAbs(link) {
			dir := filepath.Dir(path)
			path = filepath.Join(dir, link)
		} else {
			path = link
		}
	}
}
