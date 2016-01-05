package main

import (
	"go/build"
	"log"
	"os"
	"path/filepath"

	"github.com/LK4D4/vndr/godl"
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
	cfgPath := filepath.Join(wd, configFile)
	var pkgs []*build.Package
	if len(os.Args) > 1 {
		if os.Args[1] == "init" {
			if _, err := os.Stat(configFile); !os.IsNotExist(err) {
				if err == nil {
					log.Fatal("You already have vndr.cfg, remove it if you want to reinit")
				}
				log.Fatal(err)
			}
			if _, err := os.Stat(vd); !os.IsNotExist(err) {
				if err == nil {
					log.Fatal("You already have vendor directory, remove it if you want to reinit")
				}
				log.Fatal(err)
			}
			log.Printf("Go get dependencies to %s", vd)
			var dlDeps []*godl.VCS
			dlFunc := func(importPath, dir string) error {
				vcs, err := godl.Download(importPath, dir, "")
				if err != nil {
					return err
				}
				log.Printf("Downloaded to vendor dir %s", vcs.ImportPath)
				dlDeps = append(dlDeps, vcs)
				return nil
			}
			// traverse dependency tree with on-fly downloading
			initPkgs, err := collectAllDeps(wd, dlFunc, initPkgs...)
			if err != nil {
				log.Fatalf("Error on collecting all dependencies: %v", err)
			}
			log.Printf("All dependencies downloaded")
			// cleanup vcs and vendor dirs
			deps, err := cleanDeps(dlDeps)
			if err != nil {
				log.Fatal(err)
			}
			if err := writeConfig(deps, cfgPath); err != nil {
				log.Fatal(err)
			}
			log.Printf("Config written to %s", cfgPath)
			pkgs = initPkgs
		}
	} else {
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
	}
	log.Println("Collecting all dependencies")
	// pkgs != nil if init was used
	if pkgs != nil {
		upkgs, err := collectAllDeps(wd, nil, initPkgs...)
		if err != nil {
			log.Fatalf("Error on collecting all dependencies: %v", err)
		}
		pkgs = upkgs
	}
	log.Println("All dependencies collected")
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	log.Println("Success")
}
