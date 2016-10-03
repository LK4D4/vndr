package main

import (
	"flag"
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

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s [[import path] [revision]] [repository]\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func validateArgs() {
	args := flag.Args()
	if len(args) != 0 && len(args) != 2 && len(args) != 3 {
		flag.Usage()
		os.Exit(2)
	}
}

func getDeps() ([]depEntry, error) {
	if len(flag.Args()) != 0 {
		dep := depEntry{
			importPath: flag.Arg(0),
			rev:        flag.Arg(1),
			repoPath:   flag.Arg(2),
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
	flag.Parse()
	validateArgs()
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
