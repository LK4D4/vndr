package main

import (
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

func cleanVendor(vendorDir string, realDeps []*build.Package) error {
	realPaths := make(map[string]bool)
	realPaths[vendorDir] = true
	for _, pkg := range realDeps {
		realPaths[pkg.Dir] = true
	}
	var paths []string
	err := filepath.Walk(vendorDir, func(path string, i os.FileInfo, err error) error {
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			// at this point we cleaned all files from unused deps dirs
			lst, err := ioutil.ReadDir(p)
			if err != nil {
				return err
			}
			if len(lst) > 1 {
				continue
			}
			// remove empty dirs and dirs with sole LICENSE file
			if len(lst) == 0 || (len(lst) == 1 && lst[0].Name() == "LICENSE") {
				if err := os.RemoveAll(p); err != nil {
					return err
				}
			}
			continue

		}
		if realPaths[filepath.Dir(p)] {
			continue
		}
		// remove all files if they're not in dependency paths
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
