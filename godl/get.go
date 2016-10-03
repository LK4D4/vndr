// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package godl

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// VCS represents package vcs root.
type VCS struct {
	Root       string
	ImportPath string
	Type       string
}

// Download downloads package by its import path. It can be a subpackage,
// whole repo will be downloaded anyway.
// if repoPath is not empty string, it will be uses for vcs.
// target is top directory for download. i.e. if target is vendor/ and
// importPath is github.com/LK4D4/vndr, package will be downloaded to
// vendor/github.com/LK4D4/vndr.
// rev is desired revision of package.
func Download(importPath, repoPath, target, rev string) (*VCS, error) {
	security := secure
	// Analyze the import path to determine the version control system,
	// repository, and the import path for the root of the repository.
	rr, err := repoRootForImportPath(importPath, security)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(target, rr.root)
	if repoPath != "" {
		rr.repo = repoPath
	}

	if err := os.RemoveAll(root); err != nil {
		return nil, fmt.Errorf("remove package root: %v", err)
	}
	// Some version control tools require the parent of the target to exist.
	parent, _ := filepath.Split(root)
	if err = os.MkdirAll(parent, 0777); err != nil {
		return nil, err
	}
	if rev == "" {
		if err = rr.vcs.create(root, rr.repo); err != nil {
			return nil, err
		}
	} else {
		if err = rr.vcs.createRev(root, rr.repo, rev); err != nil {
			return nil, err
		}
	}
	return &VCS{Root: root, ImportPath: importPath, Type: rr.vcs.cmd}, nil
}

// goTag matches go release tags such as go1 and go1.2.3.
// The numbers involved must be small (at most 4 digits),
// have no unnecessary leading zeros, and the version cannot
// end in .0 - it is go1, not go1.0 or go1.0.0.
var goTag = regexp.MustCompile(
	`^go((0|[1-9][0-9]{0,3})\.)*([1-9][0-9]{0,3})$`,
)

// selectTag returns the closest matching tag for a given version.
// Closest means the latest one that is not after the current release.
// Version "goX" (or "goX.Y" or "goX.Y.Z") matches tags of the same form.
// Version "release.rN" matches tags of the form "go.rN" (N being a floating-point number).
// Version "weekly.YYYY-MM-DD" matches tags like "go.weekly.YYYY-MM-DD".
//
// NOTE(rsc): Eventually we will need to decide on some logic here.
// For now, there is only "go1".  This matches the docs in go help get.
func selectTag(goVersion string, tags []string) (match string) {
	for _, t := range tags {
		if t == "go1" {
			return "go1"
		}
	}
	return ""

	/*
		if goTag.MatchString(goVersion) {
			v := goVersion
			for _, t := range tags {
				if !goTag.MatchString(t) {
					continue
				}
				if cmpGoVersion(match, t) < 0 && cmpGoVersion(t, v) <= 0 {
					match = t
				}
			}
		}

		return match
	*/
}

// cmpGoVersion returns -1, 0, +1 reporting whether
// x < y, x == y, or x > y.
func cmpGoVersion(x, y string) int {
	// Malformed strings compare less than well-formed strings.
	if !goTag.MatchString(x) {
		return -1
	}
	if !goTag.MatchString(y) {
		return +1
	}

	// Compare numbers in sequence.
	xx := strings.Split(x[len("go"):], ".")
	yy := strings.Split(y[len("go"):], ".")

	for i := 0; i < len(xx) && i < len(yy); i++ {
		// The Atoi are guaranteed to succeed
		// because the versions match goTag.
		xi, _ := strconv.Atoi(xx[i])
		yi, _ := strconv.Atoi(yy[i])
		if xi < yi {
			return -1
		} else if xi > yi {
			return +1
		}
	}

	if len(xx) < len(yy) {
		return -1
	}
	if len(xx) > len(yy) {
		return +1
	}
	return 0
}
