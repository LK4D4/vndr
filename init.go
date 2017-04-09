package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/LK4D4/vndr/godl"
)

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

func getRev(v *godl.VCS) (string, error) {
	switch v.Type {
	case "git":
		return gitDep(v.Root)
	case "hg":
		return hgDep(v.Root)
	case "svn":
		return svnDep(v.Root)
	case "bzr":
		return bzrDep(v.Root)
	}
	return "", errors.New("unknown vcs type")
}

func writeConfig(deps []depEntry, cfgFile string) error {
	var lines []string
	for _, d := range deps {
		lines = append(lines, d.String())
	}
	return ioutil.WriteFile(cfgFile, []byte(strings.Join(lines, "")), os.FileMode(0666))
}
