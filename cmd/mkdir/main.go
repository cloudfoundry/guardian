package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	usage              = "usage: mkdirer <path>"
	defaultPermissions = os.FileMode(0755)
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	var path = os.Args[1]

	if path == "" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	err := mkdir(path, defaultPermissions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not mkdir %q: %v\n", path, err)
		os.Exit(3)
	}
}

func mkdir(path string, mode os.FileMode) error {
	if stat, err := os.Lstat(path); err == nil {
		if isSymlink(stat) {
			if err := mkdir(mustReadSymlink(path), mode); err != nil {
				return err
			}
		}
		return nil
	}

	if err := mkdir(filepath.Dir(path), mode); err != nil {
		return err
	}

	if err := os.Mkdir(path, mode); err != nil {
		return err
	}

	return nil
}

func isSymlink(stat os.FileInfo) bool {
	return stat.Mode()&os.ModeSymlink == os.ModeSymlink
}

func mustReadSymlink(path string) string {
	path, err := os.Readlink(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read symlink %q: %v\n", path, err)
		os.Exit(2)
	}

	return path
}
