package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/LK4D4/vndr/godl"
)

var errNotVcs = errors.New("not a vcs dir")

func gitDep(root string) (string, error) {
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = root
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error get revision: %v, out: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func hgDep(root string) (string, error) {
	revCmd := exec.Command("hg", "parent", "--template", "'{node}'")
	revCmd.Dir = root
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error get revision: %v, out: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func svnDep(root string) (string, error) {
	revCmd := exec.Command("svnversion")
	revCmd.Dir = root
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error get revision: %v, out: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func bzrDep(root string) (string, error) {
	revCmd := exec.Command("bzr", "revno")
	revCmd.Dir = root
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error get revision: %v, out: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func cleanDeps(vcsDeps []*godl.VCS) ([]depEntry, error) {
	var deps []depEntry
	for _, v := range vcsDeps {
		var depFn func(string) (string, error)
		switch v.Type {
		case "git":
			depFn = gitDep
		case "hg":
			depFn = hgDep
		case "svn":
			depFn = svnDep
		case "bzr":
			depFn = bzrDep
		}
		rev, err := depFn(v.Root)
		if err != nil {
			return nil, err
		}
		if err := cleanVCS(v); err != nil {
			return nil, err
		}
		deps = append(deps, depEntry{
			rev:        rev,
			importPath: v.ImportPath,
		})
	}
	return deps, nil
}

func writeConfig(deps []depEntry, cfgFile string) error {
	var lines []string
	for _, d := range deps {
		lines = append(lines, d.String())
	}
	sort.Strings(lines)
	return ioutil.WriteFile(cfgFile, []byte(strings.Join(lines, "")), os.FileMode(0666))
}
