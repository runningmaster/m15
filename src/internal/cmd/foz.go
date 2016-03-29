package cmd

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"internal/log"
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
	flagMGn string
	flagMFm string
	flagMTo string
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
	f.StringVar(&c.flagPOP, "pop", "", "POP server 'mail://user:pass@host:port'")
	f.StringVar(&c.flagWEB, "web", "", "WEB server 'scheme://domain.com'")

	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")

	f.StringVar(&c.flagMGn, "mgn", "", "Mailgun service 'mail://key@box.mailgun.org'")
	f.StringVar(&c.flagMFm, "mfm", "noreplay@example.com", "Mailgun from")
	f.StringVar(&c.flagMTo, "mto", "", "Mailgun to")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdFOZ) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadAndPushGzips(); err != nil {
		goto fail
	}
	return subcommands.ExitSuccess

fail:
	log.Println(c.sendError(err))
	return subcommands.ExitFailure
}

func (c *cmdFOZ) sendError(err error) error {
	if c.flagMGn != "" {
		if err = mail.Send(
			c.flagMGn,
			c.flagMFm,
			fmt.Sprintf("ERROR [%s]", c.Name()),
			fmt.Sprintf("%v: version %v: %v", time.Now(), version.Stamp.Extended(), err),
			c.flagMTo,
		); err != nil {
			return err
		}
	}
	return nil
}

func extractKeyTag(s string) (key string, tag string, ok bool) {
	subj := struct {
		Key string `json:"key"`
		Tag string `json:"tag"`
	}{}

	if err := json.NewDecoder(strings.NewReader(s)).Decode(&subj); err != nil {
		return "", "", false
	}

	return subj.Key, subj.Tag, subj.Key != "" && subj.Tag != ""
}

func (c *cmdFOZ) downloadAndPushGzips() error {
	vCh := mail.NewFileChan(
		c.flagPOP,
		func(name string) bool {
			return strings.HasPrefix(strings.ToLower(filepath.Ext(name)), ".gz")
		},
		true,
	)

	var err error
	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}

		if key, tag, ok := extractKeyTag(v.File.Subj()); ok {
			c.flagKey, c.flagTag = key, tag
		}

		if err = c.pushGzip(v.File); err != nil {
			return err
		}
	}

	return nil
}

func (c *cmdFOZ) pushGzip(r io.Reader) error {
	ctx, _ := context.WithTimeout(c.httpCtx, 30*time.Second)
	cli := c.httpCli
	url := c.makeURL("/data/add" + "?key=" + c.flagKey)

	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Encoding", "application/x-gzip")
	req.Header.Set("Content-Type", "application/json; charset=utf-8; hashtag="+c.flagTag)
	req.Header.Set("User-Agent", c.httpUsr)

	res, err := ctxhttp.Do(ctx, cli, req)
	if err != nil {
		return err
	}

	if err = res.Body.Close(); err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("foz: push failed with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdFOZ) failFast() error {
	ctx, _ := context.WithTimeout(c.httpCtx, 5*time.Second)
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
