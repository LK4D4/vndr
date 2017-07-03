package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LK4D4/vndr/build"
)

var (
	ctx = &build.Context{
		UseAllFiles: true,
		IgnoreTags:  []string{"ignore"},
		Compiler:    runtime.Compiler,
		CgoEnabled:  true,
		GOROOT:      runtime.GOROOT(),
		GOPATH:      os.Getenv("GOPATH"),
	}
)

func init() {
	gp, err := getGOPATH()
	if err != nil {
		log.Fatal(err)
	}
	ctx.GOPATH = gp
}

func collectAllDeps(wd string, dlFunc func(imp string) (*build.Package, error), initPkgs ...*build.Package) ([]*build.Package, error) {
	pkgCache := make(map[string]*build.Package)
	var deps []*build.Package
	initPkgsMap := make(map[*build.Package]bool)
	for _, pkg := range initPkgs {
		initPkgsMap[pkg] = true
		pkgCache[pkg.ImportPath] = pkg
		deps = append(deps, pkg)
	}
	for len(deps) != 0 {
		pkg := deps[len(deps)-1]
		deps = deps[:len(deps)-1]
		imports := pkg.Imports
		if initPkgsMap[pkg] {
			imports = append(imports, pkg.TestImports...)
			imports = append(imports, pkg.XTestImports...)
		}
		for _, imp := range imports {
			if imp == "C" {
				continue
			}
			if _, ok := pkgCache[imp]; ok {
				continue
			}
			ipkg, err := ctx.Import(imp, wd, 0)
			if ipkg.Goroot {
				continue
			}
			if err != nil {
				if strings.Contains(err.Error(), "cannot find package ") && dlFunc != nil {
					ipkg, err = dlFunc(imp)
				}
			} else if !strings.HasPrefix(ipkg.Dir, wd) {
				// dependency not in vendor
				if dlFunc != nil {
					ipkg, err = dlFunc(imp)
				} else {
					Warnf("dependency is not vendored: %s", imp)
				}
			}
			if _, ok := err.(*build.MultiplePackageError); !ok && err != nil {
				if verbose {
					log.Printf("\tWARNING(verbose) %s: %v", imp, err)
				}
				continue
			}
			pkgCache[imp] = ipkg
			deps = append(deps, ipkg)
		}
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
		if pkg.Goroot {
			return nil
		}
		pkgs = append(pkgs, pkg)
		return nil
	})
	return pkgs, err
}
