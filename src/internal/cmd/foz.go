package cmd

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"time"

	"internal/net/mail"
	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
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
	flagPOP string
	flagWEB string
	flagKey string
	flagTag string
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
	f.StringVar(&c.flagPOP, "pop", "", "network address for POP server 'mail://user:pass@host:port'")
	f.StringVar(&c.flagWEB, "web", "", "network address for WEB server 'scheme://domain.com'")
	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdFOZ) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadGzips(); err != nil {
		goto fail
	}

fail:
	fmt.Println(err)
	return subcommands.ExitFailure
}

func (c *cmdFOZ) downloadGzips() error {
	vCh := mail.NewFileChan(c.flagPOP, false)
	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}
		fmt.Println(v.File.Name())
	}
	return nil
}

func (c *cmdFOZ) failFast() error {
	ctx, _ := context.WithTimeout(c.httpCtx, 10*time.Second)
	cli := c.httpCli
	url := c.makeURL("/ping")

	res, err := ctxhttp.Get(ctx, cli, url)
	if err != nil {
		return err
	}

	if err = res.Body.Close(); err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("foz: fail fast with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdFOZ) makeURL(path string) string {
	return c.flagWEB + path
}
