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
)

var (
	verbose bool
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s [[import path] [revision]] [repository]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&verbose, "verbose", false, "shows all warnings")
}

func validateArgs() {
	if len(flag.Args()) > 3 {
		flag.Usage()
		os.Exit(2)
	}
}

func getDeps() ([]depEntry, error) {
	cfg, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %v", err)
	}
	deps, err := parseDeps(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse config: %v", err)
	}
	if len(flag.Args()) != 0 {
		dep := depEntry{
			importPath: flag.Arg(0),
			rev:        flag.Arg(1),
			repoPath:   flag.Arg(2),
		}
		// if there is no revision, try to find it in config
		if dep.rev == "" {
			for _, d := range deps {
				if d.importPath == dep.importPath {
					dep.rev = d.rev
					dep.repoPath = d.repoPath
					break
				}
			}
			if dep.rev == "" {
				return nil, fmt.Errorf("Failed to find %s in config file and revision was not specified", dep.importPath)
			}
		}
		return []depEntry{dep}, nil
	}
	return deps, nil
}

func main() {
	start := time.Now()
	defer func() {
		log.Printf("Running time: %v", time.Since(start))
	}()
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
	log.Println("Collecting initial packages")
	initPkgs, err := collectPkgs(wd)
	if err != nil {
		log.Fatalf("Error collecting initial packages: %v", err)
	}
	vd := filepath.Join(wd, vendorDir)
	log.Println("Download dependencies")
	if err := cloneAll(vd, deps); err != nil {
		log.Fatal(err)
	}
	log.Println("Dependencies downloaded")
	log.Println("Collecting all dependencies")
	pkgs, err := collectAllDeps(wd, initPkgs...)
	if err != nil {
		log.Fatalf("Error on collecting all dependencies: %v", err)
	}
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	log.Println("Success")
}
