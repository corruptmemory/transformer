package util

import (
	"os"
)

func WhatsThere(path string) (exists bool, file bool, dir bool, err error) {
	r, err := os.Stat(path)
	// Not exactly right, but for now...
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, false, nil
		}
		return true, false, false, err
	}

	if r.IsDir() {
		return true, false, true, nil
	} else {
		return true, true, false, nil
	}
}

func IsFile(path string) bool {
	if r, err := os.Stat(path); err == nil {
		return !r.IsDir()
	}
	return false
}
