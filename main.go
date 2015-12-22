package main

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
)

const vendorDir = "vendor"

var (
	ctx = build.Default
)

func init() {
	ctx.UseAllFiles = true
}

func collectAllDeps(wd string, initPkgs ...*build.Package) ([]*build.Package, error) {
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
				pkg, err := ctx.Import(imp, wd, build.AllowVendor)
				if pkg.Goroot {
					continue
				}
				if err != nil {
					log.Printf("WARN: unsatisfied dep: %s\n", imp)
					continue
				}
				if _, ok := pkgCache[pkg.ImportPath]; ok {
					continue
				}
				newDeps = append(newDeps, pkg)
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

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	log.Println("Collecting local packages list")
	initPkgs, err := collectPkgs(wd)
	if err != nil {
		log.Fatalf("Error collecting initial packages: %v", err)
	}
	log.Println("Init pkgs:")
	for _, pkg := range initPkgs {
		log.Printf("\t%s\n", pkg.ImportPath)
	}
	cfg, err := os.Open("vendorConfig")
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	deps, err := parseDeps(cfg, filepath.Join(wd, vendorDir))
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
	log.Println("Download dependencies")
	if err := cloneAll(deps); err != nil {
		log.Fatal(err)
	}
	log.Println("Dependencies downloaded")
	log.Println("Collecting all dependencies")
	pkgs, err := collectAllDeps(wd, initPkgs...)
	if err != nil {
		log.Fatalf("Error on collecting all dependencies: %v", err)
	}
	log.Println("All dependencies collected")
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(filepath.Join(wd, vendorDir), pkgs); err != nil {
		log.Fatal(err)
	}
	log.Println("Success")
}
