package cmd

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"internal/archive/zip"
	"internal/log"
	"internal/net/ftp"
	"internal/net/mail"
	"internal/version"

	dbf "github.com/CentaurWarchief/godbf"
	"github.com/google/subcommands"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

var (
	bel = &cmdBel{
		mapFile: make(map[string]ftp.Filer, 100),
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

type meta struct {
	Version     int
	Agent       string
	Timestamp   string
	TRangeLower string
	TRangeUpper string
}

type item struct {
	Code     string
	Drug     string
	QuantInp float64
	QuantOut float64
	PriceInp float64
	PriceOut float64
	PriceRoc float64
	Balance  float64
	BalanceT float64
}

type head struct {
	Source    string
	Drugstore string
}

type data struct {
	Head head
	Item []item
}

type priceOld struct {
	Meta meta
	Data []data
}

type cmdBel struct {
	flagFTP string
	flagWEB string
	flagKey string
	flagTag string
	flagMGn string
	flagMFm string
	flagMTo string
	mapFile map[string]ftp.Filer
	//mapShop map[string]shop
	//mapDrug map[string]drug
	//mapProp map[string][]prop
	httpCli *http.Client
	httpCtx context.Context
	httpUsr string
}

// Name returns the name of the command.
func (c *cmdBel) Name() string {
	return "bel"
}

// Synopsis returns a short string (less than one line) describing the command.
func (c *cmdBel) Synopsis() string {
	return "download, transform and send to skynet zip(dbf) files from ftp"
}

// Usage returns a long string explaining the command and giving usage information.
func (c *cmdBel) Usage() string {
	return fmt.Sprintf("%s %s", version.Stamp.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdBel) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagFTP, "ftp", "", "network address for FTP server 'ftp://user:pass@host:port[,...]'")
	f.StringVar(&c.flagWEB, "web", "", "network address for WEB server 'scheme://domain.com'")

	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")

	f.StringVar(&c.flagMGn, "mgn", "", "Mailgun service 'mail://key@box.mailgun.org'")
	f.StringVar(&c.flagMFm, "mfm", "noreplay@example.com", "Mailgun from")
	f.StringVar(&c.flagMTo, "mto", "", "Mailgun to")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdBel) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadZIPs(); err != nil {
		goto fail
	}

	if err = c.transformDBFs(); err != nil {
		goto fail
	}

	if err = c.uploadGzipJSONs(); err != nil {
		goto fail
	}

	//if err = c.deleteZIPs(walkWay...); err != nil {
	//	goto Fail
	//}

	return subcommands.ExitSuccess

fail:
	log.Println(err)
	if err = c.sendError(err); err != nil {
		log.Println(err)
	}
	return subcommands.ExitFailure
}

func (c *cmdBel) sendError(err error) error {
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

func (c *cmdBel) downloadZIPs() error {
	splitFlag := strings.Split(c.flagFTP, ",")
	for i := range splitFlag {
		vCh := ftp.NewFileChan(
			splitFlag[i],
			nil,
			false,
		)

		for v := range vCh {
			if v.Error != nil {
				return v.Error
			}
			c.mapFile[v.File.Name()] = v.File
		}
	}

	return nil
}

func (c *cmdBel) transformDBFs() error {
	for k, v := range c.mapFile {
		rc, err := zip.ExtractFile(v, v.Size())
		if err != nil {
			return err
		}

		b, err := ioutil.ReadAll(rc)
		if err != nil {
			return err
		}
		_ = rc.Close()

		f, err := dbf.NewReader(bytes.NewReader(b), &cp866Decoder{})
		if err != nil {
			return err
		}

		for i := uint32(0); i < f.RecordCount(); i++ {
			r, err := f.Read(uint16(i))
			if err != nil {
				return err
			}
			if i == 0 {
				fmt.Println(k)
				fmt.Println(castToStringSafely(r["APTEKA"]))
				fmt.Println(castToStringSafely(r["DATE"]))
			}
		}
	}
	return nil
}

func (c *cmdBel) uploadGzipJSONs() error {
	return nil
}

func (c *cmdBel) pushGzip(r io.Reader) error {
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
		return fmt.Errorf("bel: push failed with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdBel) failFast() error {
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
		return fmt.Errorf("bel: fail fast with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdBel) makeURL(path string) string {
	return c.flagWEB + path
}

type cp866Decoder struct{}

func (d *cp866Decoder) Decode(in []byte) ([]byte, error) {
	if utf8.Valid(in) {
		return in, nil
	}
	r := transform.NewReader(bytes.NewReader(in), charmap.CodePage866.NewDecoder())
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func castToStringSafely(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
