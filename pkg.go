package main

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
)

var (
	ctx = build.Default
)

func init() {
	ctx.UseAllFiles = true
}

func removeMain(mpErr *build.MultiplePackageError, imp, wd string) (*build.Package, error) {
	for i, pkgName := range mpErr.Packages {
		if pkgName == "main" {
			if err := os.Remove(filepath.Join(mpErr.Dir, mpErr.Files[i])); err != nil {
				return nil, err
			}
		}
	}
	pkg, err := ctx.Import(imp, wd, 0)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func collectAllDeps(wd string, downloadFunc func(importPath, dir string) error, initPkgs ...*build.Package) ([]*build.Package, error) {
	pkgCache := make(map[string]*build.Package)
	var deps []*build.Package
	for _, pkg := range initPkgs {
		pkgCache[pkg.ImportPath] = pkg
		deps = append(deps, pkg)
	}
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
				ipkg, err := ctx.Import(imp, wd, 0)
				if ipkg.Goroot {
					continue
				}
				if err != nil {
					if _, ok := err.(*build.MultiplePackageError); !ok {
						log.Printf("WARNING: %v", err)
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
			if _, ok := err.(*build.MultiplePackageError); !ok {
				// not a package
				if _, ok := err.(*build.NoGoError); ok {
					return nil
				}
				return err
			}
		}
		pkgs = append(pkgs, pkg)
		return nil
	})
	return pkgs, err
}
