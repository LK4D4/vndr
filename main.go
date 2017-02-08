package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LK4D4/vndr/build"
	"github.com/LK4D4/vndr/godl"
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
		fmt.Fprintf(os.Stderr, "%s [[import path] [revision]] [repository]\n%s init\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&verbose, "verbose", false, "shows all warnings")
}

func validateArgs() {
	if len(flag.Args()) > 3 {
		flag.Usage()
		os.Exit(2)
	}
	if flag.Arg(0) == "init" && len(flag.Args()) > 1 {
		flag.Usage()
		os.Exit(2)
	}
}

func validateDeps(deps []depEntry) error {
	pkgs := make([]string, 0, len(deps))
	for _, d := range deps {
		pkgs = append(pkgs, d.importPath)
	}
	repos := make(map[string][]string)
	sort.Strings(pkgs)
loop:
	for _, p := range pkgs {
		for r := range repos {
			if strings.HasPrefix(p, r+"/") || p == r {
				repos[r] = append(repos[r], p)
				continue loop
			}
		}
		repos[p] = []string{}
	}
	var duplicates [][]string
	for r, subs := range repos {
		if len(subs) != 0 {
			allPkgs := append([]string{r}, subs...)
			duplicates = append(duplicates, allPkgs)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("Each line below contains packages which has same repo, please remove subpackages from config:\n")
	for _, d := range duplicates {
		b.WriteString(fmt.Sprintf("\t%v\n", d))
	}
	return errors.New(b.String())
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
	if err := validateDeps(deps); err != nil {
		return nil, fmt.Errorf("Validation error: %v", err)
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
	flag.Parse()
	validateArgs()
	if os.Getenv("GOPATH") == "" {
		log.Fatal("GOPATH must be set")
	}
	var init bool
	if flag.Arg(0) == "init" {
		init = true
		_, cerr := os.Stat(configFile)
		_, verr := os.Stat(vendorDir)
		if cerr == nil || verr == nil {
			log.Fatal("There must not be vendor dir and vendor.conf file for initialization")
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		log.Fatalf("Error getting working directory after evalsymlinks: %v", err)
	}

	log.Println("Collecting initial packages")
	initPkgs, err := collectPkgs(wd)
	if err != nil {
		log.Fatalf("Error collecting initial packages: %v", err)
	}
	vd := filepath.Join(wd, vendorDir)
	// variables for init
	var dlFunc func(string) (*build.Package, error)
	var deps []depEntry
	if !init {
		log.Println("Download dependencies")
		cfgDeps, err := getDeps()
		if err != nil {
			log.Fatal(err)
		}
		startDownload := time.Now()
		if err := cloneAll(vd, cfgDeps); err != nil {
			log.Fatal(err)
		}
		log.Printf("Dependencies downloaded. Download time: %v", time.Since(startDownload))
	} else {
		dlFunc = func(imp string) (*build.Package, error) {
			vcs, err := godl.Download(imp, "", filepath.Join(wd, vendorDir), "")
			if err != nil {
				return nil, err
			}
			rev, err := getRev(vcs)
			if err != nil {
				return nil, err
			}
			log.Printf("\tDownloaded %s, revision %s", imp, rev)
			deps = append(deps, depEntry{importPath: vcs.ImportPath, rev: rev})

			pkg, err := ctx.Import(imp, wd, 0)
			if _, ok := err.(*build.MultiplePackageError); ok {
				return pkg, nil
			}
			return pkg, err
		}
		log.Println("Start vendoring initialization")
	}
	log.Println("Collecting all dependencies")
	pkgs, err := collectAllDeps(wd, dlFunc, initPkgs...)
	if err != nil {
		log.Fatalf("Error on collecting all dependencies: %v", err)
	}
	log.Println("Clean vendor dir from unused packages")
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	if init {
		if err := writeConfig(deps, configFile); err != nil {
			log.Fatal(err)
		}
		log.Println("Vendor initialized and result is in", configFile)
	}
	log.Println("Success")
}
