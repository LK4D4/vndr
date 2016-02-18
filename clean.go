package main

import (
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LK4D4/vndr/godl"
)

func isCDir(path string) bool {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return false
	}
	for _, fi := range fis {
		ext := filepath.Ext(fi.Name())
		if ext == ".c" || ext == ".h" {
			return true
		}
	}
	return false
}

// cleanVendor removes files from unused pacakges and non-go files
func cleanVendor(vendorDir string, realDeps []*build.Package) error {
	realPaths := make(map[string]bool)
	for _, pkg := range realDeps {
		realPaths[pkg.Dir] = true
	}
	var paths []string
	err := filepath.Walk(vendorDir, func(path string, i os.FileInfo, err error) error {
		if path == vendorDir {
			return nil
		}
		if err != nil {
			return nil
		}
		if i.IsDir() {
			if i.Name() == "testdata" {
				return os.RemoveAll(path)
			}
			if isCDir(path) {
				realPaths[path] = true
				return nil
			}
			if !realPaths[path] {
				paths = append(paths, path)
			}
			return nil
		}
		if i.Name() == "LICENSE" || i.Name() == "COPYING" {
			return nil
		}
		if !realPaths[filepath.Dir(path)] {
			return os.Remove(path)
		}
		if strings.HasSuffix(path, "_test.go") {
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	// iterate over paths (longer first)
	for _, p := range paths {
		// at this point we cleaned all files from unused deps dirs
		lst, err := ioutil.ReadDir(p)
		if err != nil {
			return err
		}
		var keepDir bool
		for _, fi := range lst {
			if fi.IsDir() {
				keepDir = true
				break
			}
		}
		if keepDir {
			continue
		}
		// remove all files if they're not in dependency paths
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

func cleanVCS(v *godl.VCS) error {
	if err := os.RemoveAll(filepath.Join(v.Root, "."+v.Type)); err != nil {
		return err
	}
	for _, otherVndr := range []string{"vendor", "Godeps", "_vendor"} {
		if err := os.RemoveAll(filepath.Join(v.Root, otherVndr)); err != nil {
			return err
		}
	}
	return nil
}
