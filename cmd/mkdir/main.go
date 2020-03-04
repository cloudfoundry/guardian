package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	usage              = "usage: mkdirer <path>"
	defaultPermissions = os.FileMode(0755)
)

func main() {
	user := flag.Int("u", 0, "user id")
	group := flag.Int("g", 0, "group id")

	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	var path = args[0]

	if path == "" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	err := mkdir(path, defaultPermissions, *user, *group)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not set permissions on %q: %v\n", path, err)
		os.Exit(3)
	}
}

func mkdir(path string, mode os.FileMode, user int, group int) error {
	if stat, err := os.Lstat(path); err == nil {
		if isSymlink(stat) {
			if err := mkdir(mustReadSymlink(path), mode, user, group); err != nil {
				return err
			}
		}
		return nil
	}

	if err := mkdir(filepath.Dir(path), mode, user, group); err != nil {
		return err
	}

	if err := os.Mkdir(path, mode); err != nil {
		return err
	}

	if err := os.Chown(path, user, group); err != nil {
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
