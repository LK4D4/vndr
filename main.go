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
	cfgPath := filepath.Join(wd, configFile)
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
			if err := goGetToVendor(wd, initPkgs); err != nil {
				log.Fatalf("go get: %v", err)
			}
			log.Printf("All dependencies downloaded")
			deps, err := collectDeps(vd)
			if err != nil {
				log.Fatal(err)
			}
			if err := writeConfig(deps, cfgPath); err != nil {
				log.Fatal(err)
			}
			log.Printf("Config written to %s", cfgPath)
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
	pkgs, err := collectAllDeps(wd, initPkgs...)
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
