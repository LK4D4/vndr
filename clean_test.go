package main

import (
	"testing"
)

func TestLicenseFilesRegexp(t *testing.T) {
	cases := map[string]bool{
		"LICENSE":         true,
		"LICENSE.code":    true,
		"License.txt":     true,
		"license.go":      false,
		"license_test.go": false,
		"foo_license.go":  false,
		"license.c":       false,
	}
	for s, expected := range cases {
		result := isLicenseFile(s)
		if result != expected {
			t.Fatalf("isLicenseFile(%q): expected %v, got %v", s, expected, result)
		}
	}
}
