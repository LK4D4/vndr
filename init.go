package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func goGetToVendor(wd string, initPkgs []*build.Package) error {
	tmp := filepath.Join(wd, tmpDir)
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Mkdir(tmp, 0777); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	dir, base := filepath.Split(wd)
	rel, err := filepath.Rel(os.Getenv("GOPATH"), dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tmp, rel), 0777); err != nil {
		return err
	}
	if err := os.Symlink(wd, filepath.Join(tmp, rel, base)); err != nil {
		return err
	}
	var tags []string
	var imps []string
	for _, ip := range initPkgs {
		imps = append(imps, ip.ImportPath)
		tags = append(tags, ip.AllTags...)
	}
	args := append([]string{"get", "-d", "-tags=" + strings.Join(tags, " ")}, imps...)
	goGet := exec.Command("go", args...)
	gp := os.Getenv("GOPATH")
	os.Unsetenv("GOPATH")
	goGet.Env = append(os.Environ(), "GOPATH="+tmp)
	os.Setenv("GOPATH", gp)
	goGet.Stdout = os.Stdout
	goGet.Stderr = os.Stderr
	if err := goGet.Run(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(tmp, rel, base)); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(tmp, "src"), filepath.Join(wd, vendorDir)); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(wd, vendorDir)); err != nil {
		log.Fatal(err)
	}
	return nil
}

var errNotVcs = errors.New("not a vcs dir")

func vcsDep(root, base string) (depEntry, error) {
	if _, err := os.Stat(filepath.Join(root, base, ".git")); err == nil {
		return gitDep(root, base)
	}
	if _, err := os.Stat(filepath.Join(root, base, ".hg")); err == nil {
		return hgDep(root, base)
	}
	return depEntry{}, errNotVcs
}

func gitDep(root, base string) (depEntry, error) {
	dir := filepath.Join(root, base)
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = dir
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return depEntry{}, fmt.Errorf("error rev-parse: %v, out: %s", err, out)
	}
	rev := strings.TrimSpace(string(out))

	urlCmd := exec.Command("git", "config", "--get", "remote.origin.url")
	urlCmd.Dir = dir
	out, err = urlCmd.CombinedOutput()
	if err != nil {
		return depEntry{}, fmt.Errorf("error get origin url: %v, out: %s", err, out)
	}
	url := strings.TrimLeft(strings.TrimSpace(string(out)), "https://")
	if url != base {
		url = "https://" + url
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		return depEntry{}, err
	}
	return depEntry{
		vcsType: "git",
		url:     url,
		target:  base,
		rev:     rev,
	}, nil
}

func hgDep(root, base string) (depEntry, error) {
	return depEntry{}, nil
}

func collectDeps(dir string) ([]depEntry, error) {
	var deps []depEntry
	err := filepath.Walk(dir, func(path string, i os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !i.IsDir() {
			return nil
		}
		if path == dir {
			return nil
		}
		if filepath.Base(path) == "vendor" || filepath.Base(path) == "Godeps" {
			return os.RemoveAll(path)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		d, err := vcsDep(dir, rel)
		if err == errNotVcs {
			return nil
		}
		if err != nil {
			return err
		}
		deps = append(deps, d)
		return nil
	})
	return deps, err
}

func writeConfig(deps []depEntry, cfgFile string) error {
	var buf bytes.Buffer
	for _, d := range deps {
		if _, err := buf.WriteString(d.String()); err != nil {
			return err
		}
	}
	return ioutil.WriteFile(cfgFile, buf.Bytes(), os.FileMode(0666))
}
