package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const usage = "usage: mkdirer -path <path> -perm <perms>"

func main() {
	var path = flag.String("path", "", "directory path to create")
	var perm = flag.Uint("perm", 0755, "Mode to create the directory with")
	// var recreate = flag.Bool("recreate", false, "whether to delete the directory before (re-)creating it")

	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	err := mkdir(*path, os.FileMode(*perm))
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not mkdir %q: %v\n", *path, err)
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
		panic(err)
	}

	return path
}
