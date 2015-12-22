package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type depEntry struct {
	vcsType string
	url     string
	target  string
	rev     string
}

func parseDeps(r io.Reader, vendorDir string) ([]depEntry, error) {
	var deps []depEntry
	s := bufio.NewScanner(r)
	for s.Scan() {
		parts := strings.Split(s.Text(), " ")
		if len(parts) < 3 || len(parts) > 4 {
			return nil, errors.New("invalid config format")
		}
		d := depEntry{
			vcsType: parts[0],
			url:     "https://" + parts[1],
			target:  filepath.Join(vendorDir, parts[1]),
			rev:     parts[2],
		}
		if len(parts) == 4 {
			d.url = parts[3]
		}
		deps = append(deps, d)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return deps, nil
}

func preCheck(ds []depEntry) error {
	check := map[string]bool{}
	for _, d := range ds {
		switch d.vcsType {
		case "git":
			check["git"] = true
		case "hg":
			check["hg"] = true
		default:
			return fmt.Errorf("Unknown vcs: %s", d.vcsType)
		}
	}
	for vcs := range check {
		if _, err := exec.LookPath(vcs); err != nil {
			return err
		}
	}
	return nil
}

func cloneAll(ds []depEntry) error {
	if err := preCheck(ds); err != nil {
		return err
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(ds))
	for _, d := range ds {
		wg.Add(1)
		go func(d depEntry) {
			errCh <- cloneDep(d)
			wg.Done()
		}(d)
	}
	wg.Wait()
	close(errCh)
	var errs []string
	for err := range errCh {
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("Errors on clone:\n%s", strings.Join(errs, "\n"))
}

func cleanupVendor(dir string) error {
	for _, vDir := range []string{"vendor", "Godeps", "_vendor"} {
		if err := os.RemoveAll(filepath.Join(dir, vDir)); err != nil {
			return err
		}
	}
	return nil
}

func cloneDep(d depEntry) error {
	log.Printf("\tClone %s", d.url)
	defer log.Printf("\tFinished clone %s finished\n", d.url)
	_, err := os.Stat(d.target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !os.IsNotExist(err) {
		if err := os.RemoveAll(d.target); err != nil {
			return err
		}
	}
	switch d.vcsType {
	case "git":
		if err := cloneGIT(d.url, d.target, d.rev); err != nil {
			return fmt.Errorf("Failed to clone git: %v", err)
		}
	case "hg":
		if err := cloneHG(d.url, d.target, d.rev); err != nil {
			return fmt.Errorf("Failed to clone hg: %v", err)
		}
	default:
		return fmt.Errorf("Unknown vcs: %s", d.vcsType)
	}
	return cleanupVendor(d.target)
}

func cloneGIT(url, target, rev string) error {
	if err := exec.Command("git", "clone", "--quiet", "--no-checkout", url, target).Run(); err != nil {
		log.Println("ERROR cln", err)
		return err
	}
	checkoutCmd := exec.Command("git", "checkout", "--quiet", rev)
	checkoutCmd.Dir = target
	var buf bytes.Buffer
	checkoutCmd.Stderr = &buf
	if err := checkoutCmd.Run(); err != nil {
		log.Println("ERROR chk", err)
		log.Println(buf.String())
		return err
	}
	resetCmd := exec.Command("git", "reset", "--quiet", "--hard", rev)
	resetCmd.Dir = target
	if err := resetCmd.Run(); err != nil {
		log.Println("ERROR rst", err)
		return err
	}
	return os.RemoveAll(filepath.Join(target, ".git"))
}

func cloneHG(url, target, rev string) error {
	if err := exec.Command("hg", "clone", "--quiet", "--updaterev", rev, url, target).Run(); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(target, ".git"))
}
