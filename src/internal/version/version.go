//go:generate go run gen.go

package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// String returns the version according to http://semver.org/
func String() string {
	v := strings.Join(
		[]string{
			strconv.Itoa(Major),
			strconv.Itoa(Minor),
			strconv.Itoa(Patch),
		},
		".",
	)
	if Prerelease != "" {
		v = fmt.Sprintf("%s-%s", v, Prerelease)
	}
	return v
}

// WithBuildInfo returns the version with build metadata
func WithBuildInfo() string {
	return fmt.Sprintf("%s+%s.%s", String(), BuildTime, GitCommit)
}

// FIXME (for testing golint)
func AppName() string {
	return filepath.Base(os.Args[0])
}
