package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LK4D4/vndr/build"
	"github.com/LK4D4/vndr/godl"
	"github.com/LK4D4/vndr/versioned"
)

const (
	vendorDir  = "vendor"
	configFile = "vendor.conf"
)

var (
	verbose        bool
	cleanWhitelist regexpSlice
	strict         bool
)

type regexpSlice []*regexp.Regexp

var _ flag.Value = new(regexpSlice)

func (rs *regexpSlice) Set(exp string) error {
	regex, err := regexp.Compile(exp)
	if err != nil {
		return err
	}

	*rs = append(*rs, regex)
	return nil
}

func (rs *regexpSlice) String() string {
	exps := []string{}
	for _, regex := range *rs {
		exps = append(exps, fmt.Sprintf("%q", regex.String()))
	}
	return fmt.Sprintf("%v", exps)
}

func (rs *regexpSlice) matchString(str string) bool {
	for _, regex := range *rs {
		if regex.MatchString(str) {
			return true
		}
	}
	return false
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s [[import path] [revision]] [repository]\n%s init\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&verbose, "verbose", false, "shows all warnings")
	flag.Var(&cleanWhitelist, "whitelist", "regular expressions to whitelist for cleaning phase of vendoring, relative to the vendor/ directory")
	flag.BoolVar(&strict, "strict", false, "checking mode. treat non-trivial warning as an error")
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

func checkUnused(deps []depEntry, vd string) {
	for _, d := range deps {
		if _, err := os.Stat(filepath.Join(vd, d.importPath)); err != nil && os.IsNotExist(err) {
			Warnf("package %s is unused, consider removing it from vendor.conf", d.importPath)
		}
	}
}

func checkLicense(deps []depEntry, vd string) {
	for _, d := range deps {
		dir := filepath.Join(vd, d.importPath)
		files, err := os.ReadDir(dir)
		if err != nil {
			// err can be nil for unused package
			continue
		}
		licenseFiles := 0
		for _, file := range files {
			p := filepath.Join(dir, file.Name())
			if isLicenseFile(p) {
				licenseFiles++
			}
		}
		if licenseFiles == 0 && verbose {
			log.Printf("WARNING(verbose): package %s may lack license information", d.importPath)
		}
	}
}

func mergeDeps(root string, deps []depEntry) depEntry {
	merged := depEntry{importPath: root}
	merged.rev = deps[0].rev
	for _, d := range deps {
		if d.repoPath != "" {
			merged.repoPath = d.repoPath
			break
		}
	}
	return merged
}

var rootImportCache = map[string]string{}

func rootImport(dep depEntry) (string, error) {
	cachedRoot, ok := rootImportCache[dep.importPath]
	if ok {
		return cachedRoot, nil
	}
	var (
		root string
		err  error
	)
	if dep.repoPath != "" {
		// A custom repository URL is passed. Skip validating if the package
		// name is at the root of the repository, and assume it is.
		root = dep.importPath
	} else {
		// Analyze the import path to determine the version control system,
		// repository, and the import path for the root of the repository.
		root, err = godl.RootImport(dep.importPath)
		if err != nil {
			return "", err
		}
	}

	rootImportCache[dep.importPath] = root
	return root, nil
}

func validateDeps(deps []depEntry) error {
	roots := make(map[string][]depEntry)
	var rootsOrder []string
	for _, d := range deps {
		root, err := rootImport(d)
		if err != nil {
			return err
		}
		if _, ok := roots[root]; !ok {
			rootsOrder = append(rootsOrder, root)
		}
		roots[root] = append(roots[root], d)
	}
	var newDeps []depEntry
	var invalid bool
	for _, r := range rootsOrder {
		rootDeps := roots[r]
		if len(rootDeps) == 1 {
			d := rootDeps[0]
			if d.importPath != r && !versioned.IsVersioned(d.importPath) {
				Warnf("package %s is not root import, should be %s", d.importPath, r)
				invalid = true
				newDeps = append(newDeps, depEntry{importPath: r, rev: d.rev, repoPath: d.repoPath})
				continue
			}
			newDeps = append(newDeps, d)
			continue
		}
		invalid = true
		var imps []string
		for _, d := range rootDeps {
			imps = append(imps, d.importPath)
		}
		Warnf("packages '%s' has same root import %s", strings.Join(imps, ", "), r)
		newDeps = append(newDeps, mergeDeps(r, rootDeps))
	}
	if !invalid {
		return nil
	}
	tmpConfig := configFile + ".tmp"
	if err := writeConfig(newDeps, tmpConfig); err != nil {
		return err
	}
	Warnf("suggested vendor.conf is written to %s, use diff and common sense before using it", tmpConfig)
	return errors.New("There were some validation errors")
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
		return nil, err
	}
	return deps, nil
}

func getFlagDep(cfgDeps []depEntry) (depEntry, error) {
	dep := depEntry{
		importPath: flag.Arg(0),
		rev:        flag.Arg(1),
		repoPath:   flag.Arg(2),
	}
	// if there is no revision, try to find it in config
	if dep.rev == "" {
		for _, d := range cfgDeps {
			if d.importPath == dep.importPath {
				dep.rev = d.rev
				dep.repoPath = d.repoPath
				break
			}
		}
		if dep.rev == "" {
			return depEntry{}, fmt.Errorf("Failed to find %s in config file and revision was not specified", dep.importPath)
		}
	}
	return dep, nil
}

func main() {
	start := time.Now()
	defer func() {
		log.Printf("Running time: %v", time.Since(start))
	}()
	flag.Parse()
	validateArgs()
	gp, err := getGOPATH()
	if err != nil {
		log.Fatal(err)
	}
	if gp == "" {
		log.Fatal("GOPATH is not set")
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
	vd := filepath.Join(wd, vendorDir)

	log.Println("Collecting initial packages")
	initPkgs, err := collectPkgs(wd)
	if err != nil {
		log.Fatalf("Error collecting initial packages: %v", err)
	}
	// variables for init
	var dlFunc func(string) (*build.Package, error)
	var deps []depEntry
	if !init {
		log.Println("Download dependencies")
		cfgDeps, err := getDeps()
		if err != nil {
			log.Fatal(err)
		}
		if len(flag.Args()) != 0 {
			flagDep, err := getFlagDep(cfgDeps)
			if err != nil {
				log.Fatal(err)
			}
			cfgDeps = []depEntry{flagDep}
		} else {
			log.Println("Starting whole vndr cycle because no package specified")
			if len(cleanWhitelist) > 0 {
				for _, regex := range cleanWhitelist {
					log.Printf("\tIgnoring paths matching %q", regex.String())
				}
				if err := cleanVendor(vd, nil); err != nil {
					log.Fatal(err)
				}
			} else {
				if err := os.RemoveAll(vd); err != nil {
					log.Fatal(err)
				}
			}
		}
		startDownload := time.Now()
		if err := cloneAll(vd, cfgDeps); err != nil {
			log.Fatal(err)
		}
		deps = cfgDeps
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
	for _, regex := range cleanWhitelist {
		log.Printf("\tIgnoring paths matching %q", regex.String())
	}
	if err := cleanVendor(vd, pkgs); err != nil {
		log.Fatal(err)
	}
	if init {
		if err := writeConfig(deps, configFile); err != nil {
			log.Fatal(err)
		}
		log.Println("Vendor initialized and result is in", configFile)
	} else {
		checkUnused(deps, vd)
	}
	checkLicense(deps, vd)
	if strict {
		if w := Warns(); len(w) > 0 {
			log.Fatalf("Treating %d warnings as errors", len(w))
		}
	}
	log.Println("Success")
}
