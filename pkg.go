package main

import (
	"fmt"
	"go/build"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	ctx = build.Default
)

func init() {
	ctx.UseAllFiles = true
}

func collectAllDeps(wd string, downloadFunc func(importPath, dir string) error, initPkgs ...*build.Package) ([]*build.Package, error) {
	pkgCache := make(map[string]*build.Package)
	var deps []*build.Package
	for _, pkg := range initPkgs {
		pkgCache[pkg.ImportPath] = pkg
		deps = append(deps, pkg)
	}
	gopath := os.Getenv("GOPATH")
	rel, err := filepath.Rel(filepath.Join(gopath, "src"), wd)
	if err != nil {
		return nil, err
	}
	vdImportPrefix := filepath.Join(rel, "vendor")
	for {
		var newDeps []*build.Package
		for _, pkg := range deps {
			if pkg.Goroot {
				continue
			}
			for _, imp := range pkg.Imports {
				if imp == "C" {
					continue
				}
				ipkg, err := ctx.Import(imp, wd, build.AllowVendor)
				if ipkg.Goroot {
					continue
				}
				if err != nil || !strings.HasPrefix(ipkg.ImportPath, rel) {
					if downloadFunc == nil {
						log.Printf("WARN: unsatisfied dep: %s for %s\n", imp, pkg.ImportPath)
						continue
					}
					if err := downloadFunc(imp, filepath.Join(wd, vendorDir)); err != nil {
						return nil, err
					}
					dlPkg, err := ctx.Import(imp, wd, build.AllowVendor)
					if err != nil {
						return nil, err
					}
					if !strings.HasPrefix(dlPkg.ImportPath, vdImportPrefix) {
						return nil, fmt.Errorf("%s was not vendored properly", imp)
					}
				}
				if _, ok := pkgCache[ipkg.ImportPath]; ok {
					continue
				}
				newDeps = append(newDeps, ipkg)
			}
			pkgCache[pkg.ImportPath] = pkg
		}
		if len(newDeps) == 0 {
			break
		}
		deps = newDeps
	}
	var pkgs []*build.Package
	for _, pkg := range pkgCache {
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func collectPkgs(dir string) ([]*build.Package, error) {
	var pkgs []*build.Package
	err := filepath.Walk(dir, func(path string, i os.FileInfo, err error) error {
		if i == nil {
			return err
		}
		if !i.IsDir() {
			return nil
		}
		// skip vendoring directory itself
		if path == filepath.Join(dir, vendorDir) {
			return filepath.SkipDir
		}
		pkg, err := ctx.ImportDir(path, build.ImportMode(0))
		if err != nil {
			// not a package
			if _, ok := err.(*build.NoGoError); ok {
				return nil
			}
			return err
		}
		pkgs = append(pkgs, pkg)
		return nil
	})
	return pkgs, err
}
