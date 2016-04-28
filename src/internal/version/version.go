//go:generate go run gen.go

package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// String returns the version according to http://semver.org/
func String() string {
	v := fmt.Sprintf("%d.%d.%d-%s", Major, Minor, Patch, PreRelease)
	if strings.HasSuffix(v, "-") {
		return v[:len(v)-1]
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
