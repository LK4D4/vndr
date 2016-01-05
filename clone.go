package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/LK4D4/vndr/godl"
)

type depEntry struct {
	importPath string
	rev        string
}

func (d depEntry) String() string {
	return fmt.Sprintf("%s %s\n", d.importPath, d.rev)
}

func parseDeps(r io.Reader, vendorDir string) ([]depEntry, error) {
	var deps []depEntry
	s := bufio.NewScanner(r)
	for s.Scan() {
		ln := strings.TrimSpace(s.Text())
		if strings.HasPrefix(ln, "#") || ln == "" {
			continue
		}
		cidx := strings.Index(ln, "#")
		if cidx > 0 {
			ln = ln[:cidx]
		}
		ln = strings.TrimSpace(ln)
		parts := strings.Fields(ln)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config format: %s", ln)
		}
		d := depEntry{
			importPath: parts[0],
			rev:        parts[1],
		}
		deps = append(deps, d)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return deps, nil
}

func cloneAll(vd string, ds []depEntry) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(ds))
	for _, d := range ds {
		wg.Add(1)
		go func(d depEntry) {
			errCh <- cloneDep(vd, d)
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

func cloneDep(vd string, d depEntry) error {
	log.Printf("\tClone %s %s", d.importPath, d.rev)
	defer log.Printf("\tFinished clone %s", d.importPath)
	vcs, err := godl.Download(d.importPath, vd, d.rev)
	if err != nil {
		return err
	}
	return cleanVCS(vcs)
}
