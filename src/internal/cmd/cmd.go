package cmd

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"internal/mailutil"
	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

type apiV int

const (
	v1 apiV = iota + 1
	v2
)

// Run registers commands in subcommands and execute it
func Run() int {
	subcommands.Register(newCmdAve(), "")
	subcommands.Register(newCmdFoz(), "")
	subcommands.Register(newCmdBel(), "")
	subcommands.Register(newCmdA24(), "")
	subcommands.Register(newCmdStl(), "")
	subcommands.Register(newCmdTst(), "")

	return int(subcommands.Execute(context.Background()))
}

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

	httpCli *http.Client
	httpCtx context.Context
	httpUsr string

	timeout time.Duration
}

func (c *cmdBase) mustInitBase(cmd interface{}, name, desc string) {
	if cmd == nil {
		panic("cmd must be defined")
	}

	c.cmd = cmd
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
	c.httpUsr = fmt.Sprintf("%s %s", version.AppName(), version.WithBuildInfo())
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

	var err error
	if i, ok := c.cmd.(execer); ok {
		err = i.exec()
	} else {
		err = fmt.Errorf("no exec() in interface")
	}

	if err != nil {
		log.Println(c.name, "err:", err)
		err = c.sendError(err)
		if err != nil {
			log.Println(err)
		}
		return subcommands.ExitFailure
	}

	log.Println(c.name, "done", time.Since(t).String())
	return subcommands.ExitSuccess
}

func (c *cmdBase) makeURL(path string) string {
	return fmt.Sprintf("%s%s", c.flagSRV, path)
}

func (c *cmdBase) failFast() error {
	ctx, cancel := context.WithTimeout(c.httpCtx, c.timeout)
	defer cancel()

	url := c.makeURL("/ping")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	res, err := c.httpCli.Do(req)
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

func (c *cmdBase) pullData(url string) (io.Reader, error) {
	t := time.Now()

	ctx, cancel := context.WithTimeout(c.httpCtx, c.timeout)
	defer cancel()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	res, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("cmd: pull failed with code %d", res.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(res.Body)

	log.Println("pull", url, time.Since(t).String())
	return buf, err
}

func (c *cmdBase) pushGzip(r io.Reader, s string, v apiV) error {
	t := time.Now()

	ctx, cancel := context.WithTimeout(c.httpCtx, c.timeout)
	defer cancel()

	var url string
	switch v {
	case v1:
		url = c.makeURL("/data/add")
	case v2:
		url = c.makeURL("/data/add?key=") + c.flagKey
	}

	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	req.Header.Set("Content-Encoding", "application/x-gzip")
	req.Header.Set("User-Agent", c.httpUsr)
	switch v {
	case v1:
		req.Header.Set("Content-Type", "application/json; charset=utf-8; hashtag="+c.flagTag)
	case v2:
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("X-Morion-Skynet-Key", c.flagKey)
		req.Header.Set("X-Morion-Skynet-Tag", c.flagTag)
	}

	res, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("%v (%s)", err, time.Since(t).String())
	}

	err = res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("cmd: push failed with code %d", res.StatusCode)
	}

	log.Println(s, time.Since(t).String())
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
		err = mailutil.Send(
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
