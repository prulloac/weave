package main

import (
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path through a temporary file in the same directory, so a failed
// write never leaves a partial output file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".weave-export-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	cleanup := func() {
		file.Close()
		os.Remove(temp)
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}
