package main

import (
	"log"
	"os"
	"path/filepath"
)

const vendorDir = "vendor"

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
