// +build ignore

package main

// This program generates version_gen.go. Invoke it as
//	go run gen.go

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const srcFormat = `
		// Generated by go run gen.go
		// Do not edit.
		// This file must be included in .gitignore.

		package version

		const (
			// Major version when you make incompatible API changes.
			Major = major
			
			// Minor version when you add functionality in a backwards-compatible manner.
			Minor = minor
			
			// Patch version when you make backwards-compatible bug fixes.
			Patch = patch
			
			// PreRelease version may be denoted by appending a hyphen and a series of dot separated identifiers immediately following the patch version.
			PreRelease = prerelease
			
			// BuildTime is build metadata and it may be denoted by appending a plus sign and a series of dot separated identifiers immediately following the patch or pre-release version.
			BuildTime = %q
			
			// GitCommit is build metadata and it may be denoted by appending a plus sign and a series of dot separated identifiers immediately following the patch or pre-release version.
			GitCommit = %q
		)
`

var flagFile = flag.String("o", "version_generated.go", "output file name")

func main() {
	flag.Parse()

	buildtime := time.Now().Format("20060102150405")
	gitcommit := "00000000"
	if isGitRepo() {
		//gitCommand := []string{"git", "log", "-n", "1", "--format=format: +%h %cd", "HEAD"}
		gitCommand := []string{"git", "rev-parse", "HEAD"}
		res := make([]byte, len(gitcommit))
		err := execCmd(res, gitCommand...)
		if err != nil {
		}
		gitcommit = string(res)
	}

	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, srcFormat, buildtime, gitcommit)

	out, err := format.Source(buf.Bytes())
	if err != nil {
		panicWithErr(err)
	}

	err = ioutil.WriteFile(*flagFile, out, 0644)
	if err != nil {
		panicWithErr(err)
	}

	outputName := os.Getenv("GOPACKAGE") + string(os.PathSeparator) + *flagFile
	log.Println("Generated", outputName)
}

func panicWithErr(err error) {
	panic(fmt.Errorf("can not generate %s: %v", *flagFile, err))
}

// isGitRepo reports whether the working directory is inside a Git repository.
func isGitRepo() bool {
	p := ".git"
	for {
		fi, err := os.Stat(p)
		if os.IsNotExist(err) {
			p = filepath.Join("..", p)
			continue
		}
		if err != nil || !fi.IsDir() {
			return false
		}
		return true
	}
}

// execCmd is simple wrapper for exec.Command
func execCmd(dst []byte, cmd ...string) error {
	b, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return err
	}
	copy(dst, b)
	return nil
}
