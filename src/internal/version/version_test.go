package version_test

import (
	"fmt"
	"testing"

	"internal/version"
)

var (
	major      = 1
	minor      = 0
	patch      = 23
	prerelease = "devel"
)

func makeGot() string {
	version.Stamp.Reset(major, minor, patch, prerelease)
	return fmt.Sprintf("%s", version.Stamp)
}

func makeWant() string {
	return fmt.Sprintf("%d.%d.%d-%s", major, minor, patch, prerelease)
}

func TestReset(t *testing.T) {
	got, want := makeGot(), makeWant()
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func BenchmarkString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = version.Stamp.String()
	}
}
