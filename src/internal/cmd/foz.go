package cmd

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"

	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

var (
	foz = &cmdFOZ{
		httpCli: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // workaround
				},
			},
		},
		httpCtx: context.Background(),
		httpUsr: fmt.Sprintf("%s %s", version.Stamp.AppName(), version.Stamp.Extended()),
	}
)

type cmdFOZ struct {
	httpCli *http.Client
	httpCtx context.Context
	httpUsr string
}

// Name returns the name of the command.
func (c *cmdFOZ) Name() string {
	return "foz"
}

// Synopsis returns a short string (less than one line) describing the command.
func (c *cmdFOZ) Synopsis() string {
	return "download and send to skynet gzip(json) files from email"
}

// Usage returns a long string explaining the command and giving usage information.
func (c *cmdFOZ) Usage() string {
	return fmt.Sprintf("%s %s", version.Stamp.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdFOZ) SetFlags(f *flag.FlagSet) {
	return
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdFOZ) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	return subcommands.ExitFailure
}
