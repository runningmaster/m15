package run

import (
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"internal/net/httpcli"
	"internal/net/mailcli"
	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

type apiV int

const (
	v1 apiV = iota + 1
	v2
)

type execer interface {
	exec() error
}

type flager interface {
	setFlags(*flag.FlagSet)
}

type cmdBase struct {
	cmd  interface{}
	name string
	desc string

	flagSRC string
	flagSRV string
	flagKey string
	flagTag string
	flagMGn string
	flagMFm string
	flagMTo string

	timeout time.Duration
}

func (c *cmdBase) mustInitBase(cmd interface{}, name, desc string) {
	if cmd == nil {
		panic("cmd must be defined")
	}

	c.cmd = cmd
	c.name = name
	c.desc = desc

	c.timeout = 60 * time.Second
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
	return fmt.Sprintf("%s %s", version.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdBase) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagSRC, "src", "", "source scheme://user:pass@host:port[,...]")
	f.StringVar(&c.flagSRV, "srv", "", "network address for WEB server scheme://domain.com")

	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")

	f.StringVar(&c.flagMGn, "mgn", "", "mailgun service mail://api:key@box.mailgun.org")
	f.StringVar(&c.flagMFm, "mfm", "noreplay@example.com", "Mailgun from")
	f.StringVar(&c.flagMTo, "mto", "", "mailgun to")

	if i, ok := c.cmd.(flager); ok {
		i.setFlags(f)
	}
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdBase) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	t := time.Now()
	log.Println(c.name, "executing...")

	err := c.failFast()
	if err != nil {
		goto fail
	}

	if i, ok := c.cmd.(execer); ok {
		err = i.exec()
	} else {
		err = fmt.Errorf("no exec() in interface")
	}
	if err != nil {
		goto fail
	}

	log.Println(c.name, "done", time.Since(t).String())
	return subcommands.ExitSuccess
fail:
	log.Println(c.name, "err:", err)
	err = c.sendError(err)
	if err != nil {
		log.Println(c.name, "err:", err)
	}
	return subcommands.ExitFailure
}

func (c *cmdBase) makeURL(path string) string {
	return fmt.Sprintf("%s%s", c.flagSRV, path)
}

func (c *cmdBase) failFast() error {
	_, _, err := httpcli.DoWithTimeoutAndMust2xx("GET", c.makeURL("/ping"), c.timeout, nil)
	return err
}

func (c *cmdBase) pullData(url string) (io.Reader, error) {
	defer func(t time.Time) {
		log.Println("pull", url, time.Since(t).String())
	}(time.Now())

	_, body, err := httpcli.DoWithTimeoutAndMust2xx("GET", url, c.timeout, nil)

	return body, err
}

func (c *cmdBase) pushGzip(r io.Reader, s string, v apiV) error {
	defer func(t time.Time) {
		log.Println("push", s, time.Since(t).String())
	}(time.Now())

	var url string
	var hdr []string
	hdr = append(hdr, "Content-Encoding: application/x-gzip")
	hdr = append(hdr, "User-Agent: "+fmt.Sprintf("%s %s", version.AppName(), version.WithBuildInfo()))
	switch v {
	case v1:
		url = c.makeURL("/data/add?key=") + c.flagKey
		hdr = append(hdr, "Content-Type: application/json; charset=utf-8; hashtag="+c.flagTag)
	case v2:
		url = c.makeURL("/data/add")
		hdr = append(hdr, "Content-Type: application/json; charset=utf-8")
		hdr = append(hdr, "X-Morion-Skynet-Key: "+c.flagKey)
		hdr = append(hdr, "X-Morion-Skynet-Tag: "+c.flagTag)
	}

	_, _, err := httpcli.DoWithTimeoutAndMust2xx("POST", url, c.timeout, r)
	if err != nil {
		return fmt.Errorf("%v: %s", err, s)
	}

	return nil
}

func (c *cmdBase) pushGzipV1(r io.Reader, s string) error {
	return c.pushGzip(r, s, v1)
}

func (c *cmdBase) pushGzipV2(r io.Reader, s string) error {
	return c.pushGzip(r, s, v2)
}

func (c *cmdBase) sendError(err error) error {
	if c.flagMGn != "" {
		err = mailcli.Send(
			c.flagMGn,
			c.flagMFm,
			fmt.Sprintf("ERROR [%s]", c.Name()),
			fmt.Sprintf("%v: version %v: %v", time.Now(), version.WithBuildInfo(), err),
			c.flagMTo,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
