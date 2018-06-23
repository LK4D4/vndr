// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package godl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/LK4D4/vndr/godl/singleflight"
)

// envForDir returns a copy of the environment
// suitable for running in the given directory.
// The environment is the current process's environment
// but with an updated $PWD, so that an os.Getwd in the
// child will be faster.
func envForDir(dir string, base []string) []string {
	// Internally we only use rooted paths, so dir is rooted.
	// Even if dir is not rooted, no harm done.
	return mergeEnvLists([]string{"PWD=" + dir}, base)
}

// mergeEnvLists merges the two environment lists such that
// variables with the same name in "in" replace those in "out".
func mergeEnvLists(in, out []string) []string {
NextVar:
	for _, inkv := range in {
		k := strings.SplitAfterN(inkv, "=", 2)[0]
		for i, outkv := range out {
			if strings.HasPrefix(outkv, k) {
				out[i] = inkv
				continue NextVar
			}
		}
		out = append(out, inkv)
	}
	return out
}

// A vcsCmd describes how to use a version control system
// like Mercurial, Git, or Subversion.
type vcsCmd struct {
	name string
	cmd  string // name of binary to invoke command

	createCmd    []string // commands to download a fresh copy of a repository
	createRevCmd []string // commands to download specified revision of a repository

	scheme  []string
	pingCmd string

	remoteRepo  func(v *vcsCmd, rootDir string) (remoteRepo string, err error)
	resolveRepo func(v *vcsCmd, rootDir, remoteRepo string) (realRepo string, err error)
}

var isSecureScheme = map[string]bool{
	"https":   true,
	"git+ssh": true,
	"bzr+ssh": true,
	"svn+ssh": true,
	"ssh":     true,
}

func (v *vcsCmd) isSecure(repo string) bool {
	u, err := url.Parse(repo)
	if err != nil {
		// If repo is not a URL, it's not secure.
		return false
	}
	return isSecureScheme[u.Scheme]
}

// vcsList lists the known version control systems
var vcsList = []*vcsCmd{
	vcsHg,
	vcsGit,
	vcsSvn,
	vcsBzr,
}

// vcsByCmd returns the version control system for the given
// command name (hg, git, svn, bzr).
func vcsByCmd(cmd string) *vcsCmd {
	for _, vcs := range vcsList {
		if vcs.cmd == cmd {
			return vcs
		}
	}
	return nil
}

// vcsHg describes how to use Mercurial.
var vcsHg = &vcsCmd{
	name: "Mercurial",
	cmd:  "hg",

	createCmd:    []string{"clone -U {repo} {dir}"},
	createRevCmd: []string{"clone --updaterev {rev} {repo} {dir}"},

	scheme:     []string{"https", "http", "ssh"},
	pingCmd:    "identify {scheme}://{repo}",
	remoteRepo: hgRemoteRepo,
}

func hgRemoteRepo(vcsHg *vcsCmd, rootDir string) (remoteRepo string, err error) {
	out, err := vcsHg.runOutput(rootDir, "paths default")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// vcsGit describes how to use Git.
var vcsGit = &vcsCmd{
	name: "Git",
	cmd:  "git",

	createCmd:    []string{"clone {repo} {dir}", "-C {dir} submodule update --init --recursive"},
	createRevCmd: []string{"clone {repo} {dir}", "-C {dir} submodule update --init --recursive", "-C {dir} checkout {rev}", "-C {dir} reset --hard {rev}"},

	scheme:     []string{"git", "https", "http", "git+ssh", "ssh"},
	pingCmd:    "ls-remote {scheme}://{repo}",
	remoteRepo: gitRemoteRepo,
}

// scpSyntaxRe matches the SCP-like addresses used by Git to access
// repositories by SSH.
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

func gitRemoteRepo(vcsGit *vcsCmd, rootDir string) (remoteRepo string, err error) {
	cmd := "config remote.origin.url"
	errParse := errors.New("unable to parse output of git " + cmd)
	errRemoteOriginNotFound := errors.New("remote origin not found")
	outb, err := vcsGit.run1(rootDir, cmd, nil, false)
	if err != nil {
		// if it doesn't output any message, it means the config argument is correct,
		// but the config value itself doesn't exist
		if outb != nil && len(outb) == 0 {
			return "", errRemoteOriginNotFound
		}
		return "", err
	}
	out := strings.TrimSpace(string(outb))

	var repoURL *url.URL
	if m := scpSyntaxRe.FindStringSubmatch(out); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		repoURL = &url.URL{
			Scheme:  "ssh",
			User:    url.User(m[1]),
			Host:    m[2],
			RawPath: m[3],
		}
	} else {
		repoURL, err = url.Parse(out)
		if err != nil {
			return "", err
		}
	}

	// Iterate over insecure schemes too, because this function simply
	// reports the state of the repo. If we can't see insecure schemes then
	// we can't report the actual repo URL.
	for _, s := range vcsGit.scheme {
		if repoURL.Scheme == s {
			return repoURL.String(), nil
		}
	}
	return "", errParse
}

// vcsBzr describes how to use Bazaar.
var vcsBzr = &vcsCmd{
	name: "Bazaar",
	cmd:  "bzr",

	createCmd:    []string{"branch {repo} {dir}"},
	createRevCmd: []string{"branch {repo} -r {rev} {dir}"},

	scheme:      []string{"https", "http", "bzr", "bzr+ssh"},
	pingCmd:     "info {scheme}://{repo}",
	remoteRepo:  bzrRemoteRepo,
	resolveRepo: bzrResolveRepo,
}

func bzrRemoteRepo(vcsBzr *vcsCmd, rootDir string) (remoteRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "config parent_location")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outb)), nil
}

func bzrResolveRepo(vcsBzr *vcsCmd, rootDir, remoteRepo string) (realRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "info "+remoteRepo)
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	// ...
	//   (branch root|repository branch): <URL>
	// ...

	found := false
	for _, prefix := range []string{"\n  branch root: ", "\n  repository branch: "} {
		i := strings.Index(out, prefix)
		if i >= 0 {
			out = out[i+len(prefix):]
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}

	i := strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}
	out = out[:i]
	return strings.TrimSpace(string(out)), nil
}

// vcsSvn describes how to use Subversion.
var vcsSvn = &vcsCmd{
	name: "Subversion",
	cmd:  "svn",

	createCmd:    []string{"checkout {repo} {dir}"},
	createRevCmd: []string{"checkout {repo} -r {rev} {dir}"},

	// There is no tag command in subversion.
	// The branch information is all in the path names.

	scheme:     []string{"https", "http", "svn", "svn+ssh"},
	pingCmd:    "info {scheme}://{repo}",
	remoteRepo: svnRemoteRepo,
}

func svnRemoteRepo(vcsSvn *vcsCmd, rootDir string) (remoteRepo string, err error) {
	outb, err := vcsSvn.runOutput(rootDir, "info")
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	// ...
	// Repository Root: <URL>
	// ...

	i := strings.Index(out, "\nRepository Root: ")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[i+len("\nRepository Root: "):]
	i = strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[:i]
	return strings.TrimSpace(string(out)), nil
}

func (v *vcsCmd) String() string {
	return v.name
}

// run runs the command line cmd in the given directory.
// keyval is a list of key, value pairs.  run expands
// instances of {key} in cmd into value, but only after
// splitting cmd into individual arguments.
// If an error occurs, run prints the command line and the
// command's combined stdout+stderr to standard error.
// Otherwise run discards the command's output.
func (v *vcsCmd) run(dir, cmd string, keyval ...string) error {
	_, err := v.run1(dir, cmd, keyval, true)
	return err
}

// runVerboseOnly is like run but only generates error output to standard error in verbose mode.
func (v *vcsCmd) runVerboseOnly(dir, cmd string, keyval ...string) error {
	_, err := v.run1(dir, cmd, keyval, false)
	return err
}

// runOutput is like run but returns the output of the command.
func (v *vcsCmd) runOutput(dir, cmd string, keyval ...string) ([]byte, error) {
	return v.run1(dir, cmd, keyval, true)
}

// run1 is the generalized implementation of run and runOutput.
func (v *vcsCmd) run1(dir, cmdline string, keyval []string, verbose bool) ([]byte, error) {
	m := make(map[string]string)
	for i := 0; i < len(keyval); i += 2 {
		m[keyval[i]] = keyval[i+1]
	}
	args := strings.Fields(cmdline)
	for i, arg := range args {
		args[i] = expand(m, arg)
	}

	_, err := exec.LookPath(v.cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"go: missing %s command. See https://golang.org/s/gogetcmd\n",
			v.name)
		return nil, err
	}
	cmd := exec.Command(v.cmd, args...)
	cmd.Dir = dir
	cmd.Env = envForDir(cmd.Dir, os.Environ())
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	out := buf.Bytes()
	if err != nil {
		return out, err
	}
	return out, nil
}

// ping pings to determine scheme to use.
func (v *vcsCmd) ping(scheme, repo string) error {
	return v.runVerboseOnly(".", v.pingCmd, "scheme", scheme, "repo", repo)
}

// create creates a new copy of repo in dir.
// The parent of dir must exist; dir must not.
func (v *vcsCmd) create(dir, repo string) error {
	for _, cmd := range v.createCmd {
		if out, err := v.runOutput(".", cmd, "dir", dir, "repo", repo); err != nil {
			return fmt.Errorf("Err: %v, out: %s", err, out)
		}
	}
	return nil
}

func (v *vcsCmd) createRev(dir, repo, rev string) error {
	for _, cmd := range v.createRevCmd {
		if out, err := v.runOutput(".", cmd, "dir", dir, "repo", repo, "rev", rev); err != nil {
			return fmt.Errorf("Err: %v, out: %s", err, out)
		}
	}
	return nil
}

// A vcsPath describes how to convert an import path into a
// version control system and repository name.
type vcsPath struct {
	prefix string                              // prefix this description applies to
	re     string                              // pattern for import path
	repo   string                              // repository to use (expand with match of re)
	vcs    string                              // version control system to use (expand with match of re)
	check  func(match map[string]string) error // additional checks
	ping   bool                                // ping for scheme to use to download repo

	regexp *regexp.Regexp // cached compiled form of re
}

// repoRoot represents a version control system, a repo, and a root of
// where to put it on disk.
type repoRoot struct {
	vcs *vcsCmd

	// repo is the repository URL, including scheme
	repo string

	// root is the import path corresponding to the root of the
	// repository
	root string
}

var httpPrefixRE = regexp.MustCompile(`^https?:`)

// securityMode specifies whether a function should make network
// calls using insecure transports (eg, plain text HTTP).
// The zero value is "secure".
type securityMode int

const (
	secure securityMode = iota
	insecure
)

// repoRootForImportPath analyzes importPath to determine the
// version control system, and code repository to use.
func repoRootForImportPath(importPath string, security securityMode) (*repoRoot, error) {
	rr, err := repoRootFromVCSPaths(importPath, "", security, vcsPaths)
	if err == errUnknownSite {
		// If there are wildcards, look up the thing before the wildcard,
		// hoping it applies to the wildcarded parts too.
		// This makes 'go get rsc.io/pdf/...' work in a fresh GOPATH.
		lookup := strings.TrimSuffix(importPath, "/...")
		if i := strings.Index(lookup, "/.../"); i >= 0 {
			lookup = lookup[:i]
		}
		rr, err = repoRootForImportDynamic(lookup, security)
		if err != nil {
			err = fmt.Errorf("unrecognized import path %q (%v)", importPath, err)
		}
	}
	if err != nil {
		rr1, err1 := repoRootFromVCSPaths(importPath, "", security, vcsPathsAfterDynamic)
		if err1 == nil {
			rr = rr1
			err = nil
		}
	}

	if err == nil && strings.Contains(importPath, "...") && strings.Contains(rr.root, "...") {
		// Do not allow wildcards in the repo root.
		rr = nil
		err = fmt.Errorf("cannot expand ... in %q", importPath)
	}
	return rr, err
}

// RootImport returns root path of import.
// i.e. golang.org/x/net for golang.org/x/net/context
func RootImport(importPath string) (string, error) {
	rr, err := repoRootForImportPath(importPath, secure)
	if err != nil {
		return "", err
	}
	return rr.root, err
}

var errUnknownSite = errors.New("dynamic lookup required to find mapping")

// repoRootFromVCSPaths attempts to map importPath to a repoRoot
// using the mappings defined in vcsPaths.
// If scheme is non-empty, that scheme is forced.
func repoRootFromVCSPaths(importPath, scheme string, security securityMode, vcsPaths []*vcsPath) (*repoRoot, error) {
	// A common error is to use https://packagepath because that's what
	// hg and git require. Diagnose this helpfully.
	if loc := httpPrefixRE.FindStringIndex(importPath); loc != nil {
		// The importPath has been cleaned, so has only one slash. The pattern
		// ignores the slashes; the error message puts them back on the RHS at least.
		return nil, fmt.Errorf("%q not allowed in import path", importPath[loc[0]:loc[1]]+"//")
	}
	for _, srv := range vcsPaths {
		if !strings.HasPrefix(importPath, srv.prefix) {
			continue
		}
		m := srv.regexp.FindStringSubmatch(importPath)
		if m == nil {
			if srv.prefix != "" {
				return nil, fmt.Errorf("invalid %s import path %q", srv.prefix, importPath)
			}
			continue
		}

		// Build map of named subexpression matches for expand.
		match := map[string]string{
			"prefix": srv.prefix,
			"import": importPath,
		}
		for i, name := range srv.regexp.SubexpNames() {
			if name != "" && match[name] == "" {
				match[name] = m[i]
			}
		}
		if srv.vcs != "" {
			match["vcs"] = expand(match, srv.vcs)
		}
		if srv.repo != "" {
			match["repo"] = expand(match, srv.repo)
		}
		if srv.check != nil {
			if err := srv.check(match); err != nil {
				return nil, err
			}
		}
		vcs := vcsByCmd(match["vcs"])
		if vcs == nil {
			return nil, fmt.Errorf("unknown version control system %q", match["vcs"])
		}
		if srv.ping {
			if scheme != "" {
				match["repo"] = scheme + "://" + match["repo"]
			} else {
				for _, scheme := range vcs.scheme {
					if security == secure && !isSecureScheme[scheme] {
						continue
					}
					if vcs.ping(scheme, match["repo"]) == nil {
						match["repo"] = scheme + "://" + match["repo"]
						break
					}
				}
			}
		}
		rr := &repoRoot{
			vcs:  vcs,
			repo: match["repo"],
			root: match["root"],
		}
		return rr, nil
	}
	return nil, errUnknownSite
}

// repoRootForImportDynamic finds a *repoRoot for a custom domain that's not
// statically known by repoRootForImportPathStatic.
//
// This handles custom import paths like "name.tld/pkg/foo" or just "name.tld".
func repoRootForImportDynamic(importPath string, security securityMode) (*repoRoot, error) {
	slash := strings.Index(importPath, "/")
	if slash < 0 {
		slash = len(importPath)
	}
	host := importPath[:slash]
	if !strings.Contains(host, ".") {
		return nil, errors.New("import path does not begin with hostname")
	}
	urlStr, body, err := httpsOrHTTP(importPath, security)
	if err != nil {
		msg := "https fetch: %v"
		if security == insecure {
			msg = "http/" + msg
		}
		return nil, fmt.Errorf(msg, err)
	}
	defer body.Close()
	imports, err := parseMetaGoImports(body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %v", importPath, err)
	}
	// Find the matched meta import.
	mmi, err := matchGoImport(imports, importPath)
	if err != nil {
		if err != errNoMatch {
			return nil, fmt.Errorf("parse %s: %v", urlStr, err)
		}
		return nil, fmt.Errorf("parse %s: no go-import meta tags", urlStr)
	}
	// If the import was "uni.edu/bob/project", which said the
	// prefix was "uni.edu" and the RepoRoot was "evilroot.com",
	// make sure we don't trust Bob and check out evilroot.com to
	// "uni.edu" yet (possibly overwriting/preempting another
	// non-evil student).  Instead, first verify the root and see
	// if it matches Bob's claim.
	if mmi.Prefix != importPath {
		urlStr0 := urlStr
		var imports []metaImport
		urlStr, imports, err = metaImportsForPrefix(mmi.Prefix, security)
		if err != nil {
			return nil, err
		}
		metaImport2, err := matchGoImport(imports, importPath)
		if err != nil || mmi != metaImport2 {
			return nil, fmt.Errorf("%s and %s disagree about go-import for %s", urlStr0, urlStr, mmi.Prefix)
		}
	}

	if !strings.Contains(mmi.RepoRoot, "://") {
		return nil, fmt.Errorf("%s: invalid repo root %q; no scheme", urlStr, mmi.RepoRoot)
	}
	rr := &repoRoot{
		vcs:  vcsByCmd(mmi.VCS),
		repo: mmi.RepoRoot,
		root: mmi.Prefix,
	}
	if rr.vcs == nil {
		return nil, fmt.Errorf("%s: unknown vcs %q", urlStr, mmi.VCS)
	}
	return rr, nil
}

var fetchGroup singleflight.Group
var (
	fetchCacheMu sync.Mutex
	fetchCache   = map[string]fetchResult{} // key is metaImportsForPrefix's importPrefix
)

// metaImportsForPrefix takes a package's root import path as declared in a <meta> tag
// and returns its HTML discovery URL and the parsed metaImport lines
// found on the page.
//
// The importPath is of the form "golang.org/x/tools".
// It is an error if no imports are found.
// urlStr will still be valid if err != nil.
// The returned urlStr will be of the form "https://golang.org/x/tools?go-get=1"
func metaImportsForPrefix(importPrefix string, security securityMode) (urlStr string, imports []metaImport, err error) {
	setCache := func(res fetchResult) (fetchResult, error) {
		fetchCacheMu.Lock()
		defer fetchCacheMu.Unlock()
		fetchCache[importPrefix] = res
		return res, nil
	}

	resi, _ := fetchGroup.Do(importPrefix, func() (resi interface{}, err error) {
		fetchCacheMu.Lock()
		if res, ok := fetchCache[importPrefix]; ok {
			fetchCacheMu.Unlock()
			return res, nil
		}
		fetchCacheMu.Unlock()

		urlStr, body, err := httpsOrHTTP(importPrefix, security)
		if err != nil {
			return setCache(fetchResult{urlStr: urlStr, err: fmt.Errorf("fetch %s: %v", urlStr, err)})
		}
		imports, err := parseMetaGoImports(body)
		if err != nil {
			return setCache(fetchResult{urlStr: urlStr, err: fmt.Errorf("parsing %s: %v", urlStr, err)})
		}
		if len(imports) == 0 {
			err = fmt.Errorf("fetch %s: no go-import meta tag", urlStr)
		}
		return setCache(fetchResult{urlStr: urlStr, imports: imports, err: err})
	})
	res := resi.(fetchResult)
	return res.urlStr, res.imports, res.err
}

type fetchResult struct {
	urlStr  string // e.g. "https://foo.com/x/bar?go-get=1"
	imports []metaImport
	err     error
}

// metaImport represents the parsed <meta name="go-import"
// content="prefix vcs reporoot" /> tags from HTML files.
type metaImport struct {
	Prefix, VCS, RepoRoot string
}

// errNoMatch is returned from matchGoImport when there's no applicable match.
var errNoMatch = errors.New("no import match")

// matchGoImport returns the metaImport from imports matching importPath.
// An error is returned if there are multiple matches.
// errNoMatch is returned if none match.
func matchGoImport(imports []metaImport, importPath string) (_ metaImport, err error) {
	match := -1
	for i, im := range imports {
		if !strings.HasPrefix(importPath, im.Prefix) {
			continue
		}
		if match != -1 {
			err = fmt.Errorf("multiple meta tags match import path %q", importPath)
			return
		}
		match = i
	}
	if match == -1 {
		err = errNoMatch
		return
	}
	return imports[match], nil
}

// expand rewrites s to replace {k} with match[k] for each key k in match.
func expand(match map[string]string, s string) string {
	for k, v := range match {
		s = strings.Replace(s, "{"+k+"}", v, -1)
	}
	return s
}

// vcsPaths defines the meaning of import paths referring to
// commonly-used VCS hosting sites (github.com/user/dir)
// and import paths referring to a fully-qualified importPath
// containing a VCS type (foo.com/repo.git/dir)
var vcsPaths = []*vcsPath{
	// Google Code - new syntax
	{
		prefix: "code.google.com/",
		re:     `^(?P<root>code\.google\.com/p/(?P<project>[a-z0-9\-]+)(\.(?P<subrepo>[a-z0-9\-]+))?)(/[A-Za-z0-9_.\-]+)*$`,
		repo:   "https://{root}",
		check:  googleCodeVCS,
	},

	// Google Code - old syntax
	{
		re:    `^(?P<project>[a-z0-9_\-.]+)\.googlecode\.com/(git|hg|svn)(?P<path>/.*)?$`,
		check: oldGoogleCode,
	},

	// Github
	{
		prefix: "github.com/",
		re:     `^(?P<root>github\.com/[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)*$`,
		vcs:    "git",
		repo:   "https://{root}",
		check:  noVCSSuffix,
	},

	// Bitbucket
	{
		prefix: "bitbucket.org/",
		re:     `^(?P<root>bitbucket\.org/(?P<bitname>[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+))(/[A-Za-z0-9_.\-]+)*$`,
		repo:   "https://{root}",
		check:  bitbucketVCS,
	},

	// IBM DevOps Services (JazzHub)
	{
		prefix: "hub.jazz.net/git",
		re:     `^(?P<root>hub.jazz.net/git/[a-z0-9]+/[A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)*$`,
		vcs:    "git",
		repo:   "https://{root}",
		check:  noVCSSuffix,
	},

	// Git at Apache
	{
		prefix: "git.apache.org",
		re:     `^(?P<root>git.apache.org/[a-z0-9_.\-]+\.git)(/[A-Za-z0-9_.\-]+)*$`,
		vcs:    "git",
		repo:   "https://{root}",
	},

	// General syntax for any server.
	// Must be last.
	{
		re:   `^(?P<root>(?P<repo>([a-z0-9.\-]+\.)+[a-z0-9.\-]+(:[0-9]+)?(/~?[A-Za-z0-9_.\-]+)+?)\.(?P<vcs>bzr|git|hg|svn))(/~?[A-Za-z0-9_.\-]+)*$`,
		ping: true,
	},
}

// vcsPathsAfterDynamic gives additional vcsPaths entries
// to try after the dynamic HTML check.
// This gives those sites a chance to introduce <meta> tags
// as part of a graceful transition away from the hard-coded logic.
var vcsPathsAfterDynamic = []*vcsPath{
	// Launchpad. See golang.org/issue/11436.
	{
		prefix: "launchpad.net/",
		re:     `^(?P<root>launchpad\.net/((?P<project>[A-Za-z0-9_.\-]+)(?P<series>/[A-Za-z0-9_.\-]+)?|~[A-Za-z0-9_.\-]+/(\+junk|[A-Za-z0-9_.\-]+)/[A-Za-z0-9_.\-]+))(/[A-Za-z0-9_.\-]+)*$`,
		vcs:    "bzr",
		repo:   "https://{root}",
		check:  launchpadVCS,
	},
}

func init() {
	// fill in cached regexps.
	// Doing this eagerly discovers invalid regexp syntax
	// without having to run a command that needs that regexp.
	for _, srv := range vcsPaths {
		srv.regexp = regexp.MustCompile(srv.re)
	}
	for _, srv := range vcsPathsAfterDynamic {
		srv.regexp = regexp.MustCompile(srv.re)
	}
}

// noVCSSuffix checks that the repository name does not
// end in .foo for any version control system foo.
// The usual culprit is ".git".
func noVCSSuffix(match map[string]string) error {
	repo := match["repo"]
	for _, vcs := range vcsList {
		if strings.HasSuffix(repo, "."+vcs.cmd) {
			return fmt.Errorf("invalid version control suffix in %s path", match["prefix"])
		}
	}
	return nil
}

var googleCheckout = regexp.MustCompile(`id="checkoutcmd">(hg|git|svn)`)

// googleCodeVCS determines the version control system for
// a code.google.com repository, by scraping the project's
// /source/checkout page.
func googleCodeVCS(match map[string]string) error {
	if err := noVCSSuffix(match); err != nil {
		return err
	}
	data, err := httpGET(expand(match, "https://code.google.com/p/{project}/source/checkout?repo={subrepo}"))
	if err != nil {
		return err
	}

	if m := googleCheckout.FindSubmatch(data); m != nil {
		if vcs := vcsByCmd(string(m[1])); vcs != nil {
			// Subversion requires the old URLs.
			// TODO: Test.
			if vcs == vcsSvn {
				if match["subrepo"] != "" {
					return fmt.Errorf("sub-repositories not supported in Google Code Subversion projects")
				}
				match["repo"] = expand(match, "https://{project}.googlecode.com/svn")
			}
			match["vcs"] = vcs.cmd
			return nil
		}
	}

	return fmt.Errorf("unable to detect version control system for code.google.com/ path")
}

// oldGoogleCode is invoked for old-style foo.googlecode.com paths.
// It prints an error giving the equivalent new path.
func oldGoogleCode(match map[string]string) error {
	return fmt.Errorf("invalid Google Code import path: use %s instead",
		expand(match, "code.google.com/p/{project}{path}"))
}

// bitbucketVCS determines the version control system for a
// Bitbucket repository, by using the Bitbucket API.
func bitbucketVCS(match map[string]string) error {
	if err := noVCSSuffix(match); err != nil {
		return err
	}

	var resp struct {
		SCM string `json:"scm"`
	}
	url := expand(match, "https://api.bitbucket.org/1.0/repositories/{bitname}")
	data, err := httpGET(url)
	if err != nil {
		if httpErr, ok := err.(*httpError); ok && httpErr.statusCode == 403 {
			// this may be a private repository. If so, attempt to determine which
			// VCS it uses. See issue 5375.
			root := match["root"]
			for _, vcs := range []string{"git", "hg"} {
				if vcsByCmd(vcs).ping("https", root) == nil {
					resp.SCM = vcs
					break
				}
			}
		}

		if resp.SCM == "" {
			return err
		}
	} else {
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decoding %s: %v", url, err)
		}
	}

	if vcsByCmd(resp.SCM) != nil {
		match["vcs"] = resp.SCM
		if resp.SCM == "git" {
			match["repo"] += ".git"
		}
		return nil
	}

	return fmt.Errorf("unable to detect version control system for bitbucket.org/ path")
}

// launchpadVCS solves the ambiguity for "lp.net/project/foo". In this case,
// "foo" could be a series name registered in Launchpad with its own branch,
// and it could also be the name of a directory within the main project
// branch one level up.
func launchpadVCS(match map[string]string) error {
	if match["project"] == "" || match["series"] == "" {
		return nil
	}
	_, err := httpGET(expand(match, "https://code.launchpad.net/{project}{series}/.bzr/branch-format"))
	if err != nil {
		match["root"] = expand(match, "launchpad.net/{project}")
		match["repo"] = expand(match, "https://{root}")
	}
	return nil
}
