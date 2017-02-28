package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/LK4D4/vndr/godl"
)

type depEntry struct {
	importPath string
	rev        string
	repoPath   string
}

func (d depEntry) String() string {
	return fmt.Sprintf("%s %s\n", d.importPath, d.rev)
}

type byImportPath []depEntry

func (b byImportPath) Len() int {
	return len(b)
}

func (b byImportPath) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byImportPath) Less(i, j int) bool {
	return b[i].importPath < b[j].importPath
}

func parseDeps(r io.Reader) ([]depEntry, error) {
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
		if len(parts) != 2 && len(parts) != 3 {
			return nil, fmt.Errorf("invalid config format: %s", ln)
		}
		d := depEntry{
			importPath: parts[0],
			rev:        parts[1],
		}
		if len(parts) == 3 {
			d.repoPath = parts[2]
		}
		deps = append(deps, d)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	sort.Sort(byImportPath(deps))
	return deps, nil
}

func cloneAll(vd string, ds []depEntry, cleanVCSFiles bool) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(ds))
	limit := make(chan struct{}, 16)
	for _, d := range ds {
		wg.Add(1)
		go func(d depEntry) {
			limit <- struct{}{}
			errCh <- cloneDep(vd, d, cleanVCSFiles)
			wg.Done()
			<-limit
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

func cloneDep(vd string, d depEntry, cleanVCSFiles bool) error {
	if d.repoPath != "" {
		log.Printf("\tClone %s to %s, revision %s", d.repoPath, d.importPath, d.rev)
	} else {
		log.Printf("\tClone %s, revision %s", d.importPath, d.rev)
	}
	defer log.Printf("\tFinished clone %s", d.importPath)
	vcs, err := godl.Download(d.importPath, d.repoPath, vd, d.rev)
	if err != nil {
		return fmt.Errorf("%s: %v", d.importPath, err)
	}
	if cleanVCSFiles {
		return cleanVCS(vcs)
	}
	return nil
}
