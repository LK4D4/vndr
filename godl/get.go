// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package godl

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	var (
		security = secure
		rr       *repoRoot
		err      error
	)

	if repoPath != "" {
		// A custom repository URL is passed, so we do not have to deduct the
		// VCS repository from the import path. repoPath is expected to contain
		// a scheme, so parse the URL, and pass it without scheme and 'user:pass'
		// (if present).
		u, err := url.Parse(repoPath)
		if err != nil {
			return nil, err
		}
		// repoRootFromVCSPaths expects the URL without `.git` suffix, so we'll
		// remove it. This is a bit hacky, but we need the VCS detection to
		// construct repoRoot.vcs. We're removing the scheme here, as "git://"
		// URLs are no longer supported by GitHub, so we'll let it use "http(s)"
		// instead.
		cleanedRepo := u.Hostname() + strings.TrimSuffix(u.Path, ".git")
		rr, err = repoRootFromVCSPaths(cleanedRepo, "", security, vcsPaths)
		if err != nil {
			return nil, err
		}
		// vendor should use the importPath as root
		rr.root = importPath
		if strings.HasSuffix(u.Path, ".git") {
			// let's be nice, and restore the ".git" suffix if it was there.
			rr.repo += ".git"
		}
	} else {
		// Analyze the import path to determine the version control system,
		// repository, and the import path for the root of the repository.
		rr, err = repoRootForImportPath(importPath, security)
		if err != nil {
			return nil, err
		}
	}

	root := filepath.Join(target, rr.root)
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
	return &VCS{Root: root, ImportPath: rr.root, Type: rr.vcs.cmd}, nil
}
