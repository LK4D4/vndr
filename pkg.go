package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/LK4D4/vndr/build"
)

var (
	ctx = &build.Context{
		UseAllFiles: true,
		Compiler:    runtime.Compiler,
		CgoEnabled:  true,
		GOROOT:      runtime.GOROOT(),
		GOPATH:      os.Getenv("GOPATH"),
	}
)

func collectAllDeps(wd string, initPkgs ...*build.Package) ([]*build.Package, error) {
	pkgCache := make(map[string]*build.Package)
	var deps []*build.Package
	initPkgsMap := make(map[*build.Package]bool)
	for _, pkg := range initPkgs {
		initPkgsMap[pkg] = true
		pkgCache[pkg.ImportPath] = pkg
		deps = append(deps, pkg)
	}
	for {
		var newDeps []*build.Package
		for _, pkg := range deps {
			if pkg.Goroot {
				continue
			}
			handleImports := func(pkgs []string) {
				for _, imp := range pkgs {
					if imp == "C" {
						continue
					}
					ipkg, err := ctx.Import(imp, wd, 0)
					if ipkg.Goroot {
						continue
					}
					if err != nil {
						if _, ok := err.(*build.MultiplePackageError); !ok && verbose {
							log.Printf("\tWARNING %s: %v", ipkg.ImportPath, err)
						}
					}
					if _, ok := pkgCache[ipkg.ImportPath]; ok {
						continue
					}
					newDeps = append(newDeps, ipkg)
				}
				pkgCache[pkg.ImportPath] = pkg
			}
			handleImports(pkg.Imports)
			if initPkgsMap[pkg] {
				handleImports(pkg.TestImports)
				handleImports(pkg.XTestImports)
			}
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
