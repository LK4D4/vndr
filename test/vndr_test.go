package vndrtest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const (
	testRepo       = "github.com/docker/swarmkit"
	testRepoCommit = "f420c4b9e1535170fc229db97ee8ac32374020b1" // May 6, 2017
)

func setGopath(cmd *exec.Cmd, gopath string) {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GOPATH=") {
			continue
		}
		cmd.Env = append(cmd.Env, env)
	}
	cmd.Env = append(cmd.Env, "GOPATH="+gopath)
}

func skipOnShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
}

func TestVndr(t *testing.T) {
	skipOnShort(t)
	vndrBin, err := exec.LookPath("vndr")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "test-vndr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	repoDir := filepath.Join(tmp, "src", testRepo)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}

	gitCmd := exec.Command("git", "clone", "https://"+testRepo+".git", repoDir)
	out, err := gitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to clone %s to %s: %v, out: %s", testRepo, repoDir, err, out)
	}
	gitCheckoutCmd := exec.Command("git", "checkout", testRepoCommit)
	gitCheckoutCmd.Dir = repoDir
	out, err = gitCheckoutCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to checkout %s: %v, out: %s", testRepoCommit, err, out)
	}
	if err := os.RemoveAll(filepath.Join(repoDir, "vendor")); err != nil {
		t.Fatal(err)
	}

	vndrCmd := exec.Command(vndrBin)
	vndrCmd.Dir = repoDir
	setGopath(vndrCmd, tmp)

	out, err = vndrCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("vndr failed: %v, out: %s", err, out)
	}
	if !bytes.Contains(out, []byte("Success")) {
		t.Fatalf("Output did not report success: %s", out)
	}

	installCmd := exec.Command("go", "install", testRepo)
	setGopath(installCmd, tmp)
	out, err = installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install %s failed: %v, out: %s", testRepo, err, out)
	}

	// revendor only etcd
	vndrRevendorCmd := exec.Command(vndrBin, "github.com/coreos/etcd")
	vndrRevendorCmd.Dir = repoDir
	setGopath(vndrRevendorCmd, tmp)

	out, err = vndrRevendorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("vndr failed: %v, out: %s", err, out)
	}
	if !bytes.Contains(out, []byte("Success")) {
		t.Fatalf("Output did not report success: %s", out)
	}
}

func TestVndrInit(t *testing.T) {
	skipOnShort(t)
	vndrBin, err := exec.LookPath("vndr")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "test-vndr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	repoPath := "github.com/LK4D4"
	repoDir := filepath.Join(tmp, "src", repoPath)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}

	cpCmd := exec.Command("cp", "-r", "./testdata/dumbproject", repoDir)
	out, err := cpCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cp failed: %v, out: %s", err, out)
	}
	vndrCmd := exec.Command(vndrBin, "init")
	vndrCmd.Dir = filepath.Join(repoDir, "dumbproject")
	setGopath(vndrCmd, tmp)

	out, err = vndrCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("vndr failed: %v, out: %s", err, out)
	}
	if !bytes.Contains(out, []byte("Success")) {
		t.Fatalf("Output did not report success: %s", out)
	}

	pkgPath := filepath.Join(repoPath, "dumbproject")
	installCmd := exec.Command("go", "install", pkgPath)
	setGopath(installCmd, tmp)
	out, err = installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install %s failed: %v, out: %s", pkgPath, err, out)
	}
	vndr2Cmd := exec.Command(vndrBin, "init")
	vndr2Cmd.Dir = filepath.Join(repoDir, "dumbproject")
	setGopath(vndr2Cmd, tmp)

	out, err = vndr2Cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("There must not be")) {
		t.Fatalf("vndr is expected to fail about existing vendor, got %v: %s", err, out)
	}
}

func TestValidateSubpackages(t *testing.T) {
	vndrBin, err := exec.LookPath("vndr")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "test-vndr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	repoDir := filepath.Join(tmp, "src", testRepo)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte(`github.com/docker/docker branch
github.com/docker/docker/pkg/idtools branch
github.com/coreos/etcd/raft branch
github.com/docker/docker/pkg/archive branch
github.com/coreos/etcd branch
github.com/docker/swarmkit branch
github.com/docker/go branch
github.com/docker/go-connections branch
github.com/docker/go-units branch
github.com/docker/libcompose branch
github.com/docker/swarmkit branch
`)
	vendorConf := filepath.Join(repoDir, "vendor.conf")
	if err := ioutil.WriteFile(vendorConf, content, 0666); err != nil {
		t.Fatal(err)
	}
	vndrCmd := exec.Command(vndrBin)
	vndrCmd.Dir = repoDir
	setGopath(vndrCmd, tmp)

	out, err := vndrCmd.CombinedOutput()
	if err == nil {
		t.Fatal("error is expected")
	}
	t.Logf("Output of vndr:\n%s", out)
	if !bytes.Contains(out, []byte("WARNING: packages 'github.com/docker/docker, github.com/docker/docker/pkg/idtools, github.com/docker/docker/pkg/archive' has same root import github.com/docker/docker")) {
		t.Error("duplicated docker package not found")
	}
	if !bytes.Contains(out, []byte("WARNING: packages 'github.com/coreos/etcd/raft, github.com/coreos/etcd' has same root import github.com/coreos/etcd")) {
		t.Error("duplicated etcd package not found")
	}
	if !bytes.Contains(out, []byte("WARNING: packages 'github.com/docker/swarmkit, github.com/docker/swarmkit' has same root import github.com/docker/swarmkit")) {
		t.Error("duplicated swarmkit package not found")
	}
	if bytes.Contains(out, []byte("go-units")) {
		t.Errorf("go-units should not be reported: %s", out)
	}
	if bytes.Contains(out, []byte("go-connections")) {
		t.Errorf("go-connections should not be reported: %s", out)
	}
	if bytes.Contains(out, []byte("libcompose")) {
		t.Errorf("libcompose should not be reported: %s", out)
	}

	tmpFileName := "vendor.conf.tmp"
	tmpConfig := filepath.Join(repoDir, tmpFileName)
	if _, err := os.Stat(tmpConfig); err != nil {
		t.Fatalf("error stat %s: %v", tmpFileName, err)
	}
	b, err := ioutil.ReadFile(tmpConfig)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte(`github.com/docker/docker branch
github.com/coreos/etcd branch
github.com/docker/swarmkit branch
github.com/docker/go branch
github.com/docker/go-connections branch
github.com/docker/go-units branch
github.com/docker/libcompose branch
`)
	if !bytes.Equal(b, expected) {
		t.Fatalf("suggested vendor.conf is wrong:\n%s\n Should be %s", b, expected)
	}
}

func TestCleanWhitelist(t *testing.T) {
	vndrBin, err := exec.LookPath("vndr")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "test-vndr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	repoDir := filepath.Join(tmp, "src", testRepo)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte(`github.com/containers/image master
github.com/projectatomic/skopeo master`)
	vendorConf := filepath.Join(repoDir, "vendor.conf")
	if err := ioutil.WriteFile(vendorConf, content, 0666); err != nil {
		t.Fatal(err)
	}
	vndrCmd := exec.Command(vndrBin,
		"-whitelist", `github\.com/containers/image/MAINTAINERS`,
		"-whitelist", `github\.com/projectatomic/skopeo/integration/.*`)
	vndrCmd.Dir = repoDir
	setGopath(vndrCmd, tmp)

	out, err := vndrCmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %v", string(out))
		t.Fatalf("error was not expected: %v", err)
	}

	if !bytes.Contains(out, []byte(fmt.Sprintf(`Ignoring paths matching %q`, `github\.com/containers/image/MAINTAINERS`))) {
		t.Logf("output: %v", string(out))
		t.Errorf(`output missing regular expression "github\.com/containers/image/MAINTAINERS"`)
	}
	if !bytes.Contains(out, []byte(fmt.Sprintf(`Ignoring paths matching %q`, `github\.com/projectatomic/skopeo/integration/.*`))) {
		t.Logf("output: %v", string(out))
		t.Errorf(`output missing regular expression "github\.com/projectatomic/skopeo/integration/.*"`)
	}

	// Make sure that the files were not "cleaned".
	for _, path := range []string{
		"github.com/containers/image/MAINTAINERS",
		"github.com/projectatomic/skopeo/integration",
	} {
		path = filepath.Join(repoDir, "vendor", path)
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s was cleaned but shouldn't have been", path)
		}
	}

	// Run again to make sure the above will be cleaned.
	vndrCmd = exec.Command(vndrBin)
	vndrCmd.Dir = repoDir
	setGopath(vndrCmd, tmp)

	out, err = vndrCmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %v", string(out))
		t.Fatalf("[no -whitelist] error was not expected: %v", err)
	}

	if bytes.Contains(out, []byte(fmt.Sprintf(`Ignoring paths matching %q`, `github\.com/containers/image/MAINTAINERS`))) {
		t.Logf("output: %v", string(out))
		t.Errorf(`[no -whitelist] output missing regular expression "github\.com/containers/image/MAINTAINERS"`)
	}
	if bytes.Contains(out, []byte(fmt.Sprintf(`Ignoring paths matching %q`, `github\.com/projectatomic/skopeo/integration/.*`))) {
		t.Logf("output: %v", string(out))
		t.Errorf(`[no -whitelist] output missing regular expression "github\.com/projectatomic/skopeo/integration/.*"`)
	}

	// Make sure that the files were "cleaned".
	for _, path := range []string{
		"github.com/containers/image/MAINTAINERS",
		"github.com/projectatomic/skopeo/integration",
	} {
		path = filepath.Join(repoDir, "vendor", path)
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("[no -whitelist] %s was NOT cleaned but should have been", path)
		}
	}
}

func TestUnused(t *testing.T) {
	vndrBin, err := exec.LookPath("vndr")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "test-vndr-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	repoDir := filepath.Join(tmp, "src", testRepo)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	unusedPkg := "github.com/docker/go-units"
	content := []byte(unusedPkg + " master\n")
	vendorConf := filepath.Join(repoDir, "vendor.conf")
	if err := ioutil.WriteFile(vendorConf, content, 0666); err != nil {
		t.Fatal(err)
	}
	vndrCmd := exec.Command(vndrBin, "-strict")
	vndrCmd.Dir = repoDir
	setGopath(vndrCmd, tmp)

	msg := fmt.Sprintf("WARNING: package %s is unused, consider removing it from vendor.conf", unusedPkg)
	out, err := vndrCmd.CombinedOutput()
	if !bytes.Contains(out, []byte(msg)) {
		t.Logf("output: %v", string(out))
		t.Errorf("there is no warning about unused package %s", unusedPkg)
	}
	if code := getExitCode(t, err); code == 0 {
		t.Logf("strict mode expects non-zero exit code, got zero")
	}
}

func getExitCode(t *testing.T, err error) int {
	if err == nil {
		return 0
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatal("expected *os.ExitError")
	}
	status, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatal("expected syscall.WaitStatus")
	}
	return status.ExitStatus()
}
