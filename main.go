package main

import (
	"log"
	"os"
	"path/filepath"
)

const (
	vendorDir  = "vendor"
	configFile = "vndr.cfg"
	tmpDir     = ".vndr-tmp"
)

func main() {
	if os.Getenv("GOPATH") == "" {
		log.Fatal("GOPATH must be set")
	}
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
		log.Printf("\t%s", pkg.ImportPath)
	}
	vd := filepath.Join(wd, vendorDir)
	cfg, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	deps, err := parseDeps(cfg, vd)
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
	log.Println("Removing old vendor directory")
	if err := os.RemoveAll(vd); err != nil {
		log.Fatalf("Remove old vendor dir: %v", err)
	}
	log.Println("Download dependencies")
	if err := cloneAll(vd, deps); err != nil {
		log.Fatal(err)
	}
	log.Println("Dependencies downloaded")
	log.Println("Collecting all dependencies")
	pkgs, err := collectAllDeps(wd, nil, initPkgs...)
	if err != nil {
		log.Fatalf("Error on collecting all dependencies: %v", err)
	}
	log.Println("All dependencies collected")
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	log.Println("Success")
}
