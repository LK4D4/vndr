package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func getGOPATH() (string, error) {
	out, err := exec.Command("go", "env", "GOPATH").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error obtaining GOPATH: %v, %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
