package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LK4D4/vndr/build"
	"github.com/LK4D4/vndr/godl"
)

func isCDir(fis []os.FileInfo) bool {
	var hFound bool
	for _, fi := range fis {
		ext := filepath.Ext(fi.Name())
		if ext == ".cc" || ext == ".cpp" || ext == ".py" {
			return false
		}
		if ext == ".h" || ext == ".c" {
			hFound = true
		}
	}
	return hFound
}

func isPBDir(fis []os.FileInfo) bool {
	var pbFound bool
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		ext := filepath.Ext(fi.Name())
		if ext != ".proto" {
			return false
		}
		pbFound = true
	}
	return pbFound
}

func isInterestingDir(path string) bool {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return false
	}
	return isCDir(fis) || isPBDir(fis)
}

func isGoFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".go" || ext == ".c" || ext == ".h" || ext == ".s" || ext == ".proto"
}

// licenseFilesRegexp is a regexp of file names that are likely to contain licensing information
var licenseFilesRegexp = regexp.MustCompile(`(?i).*(LICENSE|COPYING|PATENT|NOTICE|README).*`)

func isLicenseFile(path string) bool {
	return licenseFilesRegexp.MatchString(path) && !isGoFile(path)
}

func isVendorConfFile(path string) bool {
	switch filepath.Base(path) {
	case "go.mod", "vendor.conf":
		return true
	default:
		return false
	}
}

// cleanVendor removes files from unused packages and non-go files
func cleanVendor(vendorDir string, realDeps []*build.Package) error {
	realPaths := make(map[string]bool)
	ignoredGoFiles := []string{}
	for _, pkg := range realDeps {
		realPaths[pkg.Dir] = true
		for _, f := range pkg.IgnoredGoFiles {
			ignoredGoFiles = append(ignoredGoFiles, filepath.Join(pkg.Dir, f))
		}
	}
	var paths []string
	err := filepath.Walk(vendorDir, func(path string, i os.FileInfo, err error) error {
		if path == vendorDir {
			return nil
		}
		if err != nil {
			return nil
		}

		// Make sure we don't delete anything that matches the whitelist. The
		// whitelist is relative to the vendor directory.
		relPath, err := filepath.Rel(vendorDir, path)
		if err != nil {
			return err
		}
		if cleanWhitelist.matchString(relPath) {
			return nil
		}

		if strings.HasPrefix(i.Name(), ".") || strings.HasPrefix(i.Name(), "_") {
			return os.RemoveAll(path)
		}
		if i.IsDir() {
			if i.Name() == "testdata" {
				return os.RemoveAll(path)
			}
			if isInterestingDir(path) {
				realPaths[path] = true
				return nil
			}
			if !realPaths[path] {
				paths = append(paths, path)
			}
			return nil
		}

		// keep files for licenses and recursive vendoring
		if isLicenseFile(i.Name()) || isVendorConfFile(i.Name()) {
			return nil
		}
		// remove files from non-deps, non-go files and test files
		if !realPaths[filepath.Dir(path)] || !isGoFile(path) || strings.HasSuffix(path, "_test.go") {
			return os.Remove(path)
		}
		// remove ignored go files
		for _, f := range ignoredGoFiles {
			if f == path {
				return os.Remove(path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	// iterate over paths (longest first)
	for _, p := range paths {
		// at this point we cleaned all files from unused deps dirs
		lst, err := ioutil.ReadDir(p)
		if err != nil {
			return err
		}
		// remove whole dir if there is only non-go files
		// keep whitelisted, though
		var onlyNonGoFile = true
		for _, fi := range lst {
			relPath, err := filepath.Rel(vendorDir, filepath.Join(p, fi.Name()))
			if err != nil {
				return err
			}
			if cleanWhitelist.matchString(relPath) || fi.IsDir() || isGoFile(fi.Name()) {
				onlyNonGoFile = false
				break
			}
		}
		if !onlyNonGoFile {
			continue
		}
		// remove all directories if they're not in dependency paths
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
	return filepath.Walk(v.Root, func(path string, i os.FileInfo, err error) error {
		if path == vendorDir {
			return nil
		}
		if !i.IsDir() {
			return nil
		}
		name := i.Name()
		if name == "vendor" || name == "Godeps" || name == "_vendor" {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}
		return nil
	})
}
