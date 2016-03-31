package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
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
	"github.com/klauspost/compress/gzip"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

var (
	bel = &cmdBel{
		mapFile: make(map[string]ftp.Filer, 100),
		mapDele: make(map[string][]string, 100),
		mapJSON: make(map[string]interface{}, 100),
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
	Timestamp   string `json:",omitempty"`
	TRangeLower string `json:",omitempty"`
	TRangeUpper string `json:",omitempty"`
}

type item struct {
	Code     string  `json:",omitempty"`
	Drug     string  `json:",omitempty"`
	QuantInp float64 `json:",omitempty"`
	QuantOut float64 `json:",omitempty"`
	PriceInp float64 `json:",omitempty"`
	PriceOut float64 `json:",omitempty"`
	PriceRoc float64 `json:",omitempty"`
	Balance  float64 `json:",omitempty"`
	BalanceT float64 `json:",omitempty"`
}

type head struct {
	Source    string `json:",omitempty"`
	Drugstore string `json:",omitempty"`
}

type data struct {
	Head head   `json:",omitempty"`
	Item []item `json:",omitempty"`
}

type priceOld struct {
	Meta meta   `json:",omitempty"`
	Data []data `json:",omitempty"`
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
	mapDele map[string][]string // for clean up
	mapJSON map[string]interface{}
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

	if err = c.deleteZIPs(); err != nil {
		goto fail
	}

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
			c.mapDele[splitFlag[i]] = append(c.mapDele[splitFlag[i]], v.File.Name())
		}
	}

	return nil
}

func (c *cmdBel) deleteZIPs() error {
	for k, v := range c.mapDele {
		if v == nil {
			continue
		}
		return ftp.Delete(k, v...)
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

		var (
			name, date    string
			drugPlusMaker = func(name, maker string) string {
				if !strings.Contains(strings.ToLower(name), strings.ToLower(maker)) {
					return fmt.Sprintf("%s %s", name, maker)
				}
				return name
			}
		)
		items := make([]item, 0, f.RecordCount())
		for i := uint32(0); i < f.RecordCount(); i++ {
			r, err := f.Read(uint16(i))
			if err != nil {
				return err
			}
			if i == 0 {
				name = castToStringSafely(r["APTEKA"])
				date = castToTimeStringSafely(r["DATE"])
			}

			items = append(items, item{
				Code:     "",
				Drug:     drugPlusMaker(castToStringSafely(r["TOVAR"]), castToStringSafely(r["PROIZV"])),
				QuantInp: castToFloat64Safely(r["APTIN"]),
				QuantOut: castToFloat64Safely(r["OUT"]),
				PriceInp: castToFloat64Safely(r["PRICEIN"]),
				PriceOut: castToFloat64Safely(r["PRICE"]),
				PriceRoc: castToFloat64Safely(r["ROC"]),
				Balance:  castToFloat64Safely(r["KOLSTAT"]),
				BalanceT: castToFloat64Safely(r["AMOUNT"]),
			})
		}

		c.mapJSON[k] = priceOld{
			Meta: meta{
				Timestamp:   time.Now().Format("02.01.2006 15:04:05.999999999"),
				TRangeLower: date + " 00:00:00",
				TRangeUpper: date + " 23:59:59",
			},
			Data: []data{
				{
					Head: head{
						Source:    "file:" + k,
						Drugstore: name,
					},
					Item: items,
				},
			},
		}
	}
	return nil
}

func (c *cmdBel) uploadGzipJSONs() error {
	b := &bytes.Buffer{}

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

	for _, v := range c.mapJSON {
		b.Reset()
		w.Reset(b)

		if err = json.NewEncoder(w).Encode(v); err != nil {
			return err
		}

		if err = w.Close(); err != nil {
			return err
		}

		if err = c.pushGzip(b); err != nil {
			return err
		}

		//if err = ioutil.WriteFile(strings.Replace(k, ".zip", ".json", -1)+".gz", b.Bytes(), 0666); err != nil {
		//	return err
		//}
	}

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

func castToFloat64Safely(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0.0
}

func castToTimeStringSafely(v interface{}) string {
	if t, ok := v.(time.Time); ok {
		return t.Format("02.01.2006")
	}
	return ""
}
