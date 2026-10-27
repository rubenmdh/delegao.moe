//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func replacePublicationPointer(source, target string) error {
	if err := os.Rename(source, target); err != nil {
		return err
	}
	return syncPublicationDirectory(filepath.Dir(target))
}

func syncPublicationDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
