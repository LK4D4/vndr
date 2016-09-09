package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	vendorDir  = "vendor"
	configFile = "vendor.conf"
	tmpDir     = ".vndr-tmp"
)

func getDeps() ([]depEntry, error) {
	if len(os.Args) != 1 && len(os.Args) != 3 && len(os.Args) != 4 {
		return nil, fmt.Errorf("USAGE: vndr [[import path] [revision]] [repository]")
	}
	if len(os.Args) != 1 {
		dep := depEntry{
			importPath: os.Args[1],
			rev:        os.Args[2],
		}
		if len(os.Args) == 4 {
			dep.repoPath = os.Args[3]
		}
		return []depEntry{dep}, nil
	}
	cfg, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %v", err)
	}
	deps, err := parseDeps(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse config: %v", err)
	}
	return deps, nil
}

func main() {
	if os.Getenv("GOPATH") == "" {
		log.Fatal("GOPATH must be set")
	}
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	deps, err := getDeps()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Removing old vendor directory")
	vd := filepath.Join(wd, vendorDir)
	log.Println("Download dependencies")
	if err := cloneAll(vd, deps); err != nil {
		log.Fatal(err)
	}
	log.Println("Dependencies downloaded")
	log.Println("Collecting all dependencies")
	start := time.Now()
	initPkgs, err := collectPkgs(wd)
	if err != nil {
		log.Fatalf("Error collecting initial packages: %v", err)
	}
	pkgs, err := collectAllDeps(wd, initPkgs...)
	if err != nil {
		log.Fatalf("Error on collecting all dependencies: %v", err)
	}
	log.Printf("All dependencies collected: %v", time.Since(start))
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	log.Println("Success")
}
