package versioned

import (
	"regexp"
)

var re = regexp.MustCompile(`/v[0-9]+$`)

// IsVersioned true for string like "github.com/coreos/go-systemd/v22"
func IsVersioned(s string) bool {
	return re.MatchString(s)
}
