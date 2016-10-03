package vndrtest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testRepo = "github.com/docker/swarmkit"

func setGopath(cmd *exec.Cmd, gopath string) {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GOPATH=") {
			continue
		}
		cmd.Env = append(cmd.Env, env)
	}
	cmd.Env = append(cmd.Env, "GOPATH="+gopath)
}

func TestVndr(t *testing.T) {
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
