package subcommands

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"internal/net/mailutil"
	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

// Register registers commands in subcommands
func Register() {
	subcommands.Register(newCmdAve(), "")
	subcommands.Register(newCmdFoz(), "")
	subcommands.Register(newCmdBel(), "")
}

type cmdBase struct {
	name string
	desc string

	flagPOP string // flagSRC ?
	flagFTP string // flagSRC ?
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

func (c *cmdBase) initBase(name, desc string) {
	c.name = name
	c.desc = desc
	c.httpCli = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // workaround
			},
		},
	}
	c.httpCtx = context.Background()
	c.httpUsr = fmt.Sprintf("%s %s", version.Stamp.AppName(), version.Stamp.Extended())
}

// Name returns the name of the command.
func (c *cmdBase) Name() string {
	return c.name
}

// Synopsis returns a short string (less than one line) describing the command.
func (c *cmdBase) Synopsis() string {
	return c.desc
}

// Usage returns a long string explaining the command and giving usage information.
func (c *cmdBase) Usage() string {
	return fmt.Sprintf("%s %s", version.Stamp.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdBase) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagPOP, "pop", "", "POP server 'mail://user:pass@host:port'")
	f.StringVar(&c.flagFTP, "ftp", "", "network address for FTP server 'ftp://user:pass@host:port[,...]'")
	f.StringVar(&c.flagWEB, "web", "", "network address for WEB server 'scheme://domain.com'")

	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")

	f.StringVar(&c.flagMGn, "mgn", "", "Mailgun service 'mail://key@box.mailgun.org'")
	f.StringVar(&c.flagMFm, "mfm", "noreplay@example.com", "Mailgun from")
	f.StringVar(&c.flagMTo, "mto", "", "Mailgun to")
}

func (c *cmdBase) makeURL(path string) string {
	return c.flagWEB + path
}

func (c *cmdBase) failFast() error {
	ctx, _ := context.WithTimeout(c.httpCtx, 5*time.Second)
	cli := c.httpCli
	url := c.makeURL("/ping")

	res, err := ctxhttp.Get(ctx, cli, url)
	if err != nil {
		return err
	}

	err = res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("cmd: fail fast with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdBase) pushGzipV1(r io.Reader) error {
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

	err = res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("cmd: push failed with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdBase) pushGzipV2(r io.Reader) error {
	ctx, _ := context.WithTimeout(c.httpCtx, 30*time.Second)
	cli := c.httpCli
	url := c.makeURL("/data/add")

	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Encoding", "application/x-gzip")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", c.httpUsr)
	req.Header.Set("X-Morion-Skynet-Key", c.flagKey)
	req.Header.Set("X-Morion-Skynet-Tag", c.flagTag)

	res, err := ctxhttp.Do(ctx, cli, req)
	if err != nil {
		return err
	}

	err = res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("ave: push failed with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdBase) sendError(err error) error {
	if c.flagMGn != "" {
		err = mailutil.Send(
			c.flagMGn,
			c.flagMFm,
			fmt.Sprintf("ERROR [%s]", c.Name()),
			fmt.Sprintf("%v: version %v: %v", time.Now(), version.Stamp.Extended(), err),
			c.flagMTo,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
