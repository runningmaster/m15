package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"internal/archive/zip"
	"internal/encoding/csv"
	"internal/encoding/txt"
	"internal/log"
	"internal/net/ftp"
	"internal/net/mail"
	"internal/version"

	"github.com/google/subcommands"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	capFile = 3
	capShop = 3000
	capDrug = 200000
	capProp = capShop

	aveHead = "АВЕ" // magic const
)

var (
	timeFmt = time.Now().Format("02.01.06")
	fileApt = fmt.Sprintf("apt_%s.zip", timeFmt)  // magic file name
	fileTov = fmt.Sprintf("tov_%s.zip", timeFmt)  // magic file name
	fileOst = fmt.Sprintf("ost_%s.zip", timeFmt)  // magic file name
	walkWay = []string{fileApt, fileTov, fileOst} // strong order files

	ave = &cmdAVE{
		mapFile: make(map[string]ftp.Filer, capFile),
		mapShop: make(map[string]shop, capShop),
		mapDrug: make(map[string]drug, capDrug),
		mapProp: make(map[string][]prop, capProp),
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

type shop struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Head string `json:"head"`
	Addr string `json:"addr"`
	Code string `json:"code"`
}

type drug struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type prop struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Quant float64 `json:"quant"`
	Price float64 `json:"price"`
}

type price struct {
	Meta shop   `json:"meta"`
	Data []prop `json:"data"`
}

type cmdAVE struct {
	flagFTP string
	flagWEB string
	flagKey string
	flagTag string
	flagMGn string
	flagMFm string
	flagMTo string
	mapFile map[string]ftp.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
	httpCli *http.Client
	httpCtx context.Context
	httpUsr string
}

// Name returns the name of the command.
func (c *cmdAVE) Name() string {
	return "ave"
}

// Synopsis returns a short string (less than one line) describing the command.
func (c *cmdAVE) Synopsis() string {
	return "download, transform and send to skynet 3 zip(csv) files from ftp"
}

// Usage returns a long string explaining the command and giving usage information.
func (c *cmdAVE) Usage() string {
	return fmt.Sprintf("%s %s", version.Stamp.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdAVE) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagFTP, "ftp", "", "network address for FTP server 'ftp://user:pass@host:port'")
	f.StringVar(&c.flagWEB, "web", "", "network address for WEB server 'scheme://domain.com'")
	f.StringVar(&c.flagKey, "key", "", "service key")
	f.StringVar(&c.flagTag, "tag", "", "service tag")

	f.StringVar(&c.flagMGn, "mgn", "", "Mailgun service 'mail://key@box.mailgun.org'")
	f.StringVar(&c.flagMFm, "mfm", "noreplay@example.com", "Mailgun from")
	f.StringVar(&c.flagMTo, "mto", "", "Mailgun to")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdAVE) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadZIPs(); err != nil {
		goto fail
	}

	if err = c.transformCSVs(walkWay...); err != nil {
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
	log.Println(c.sendError(err))
	return subcommands.ExitFailure
}

func (c *cmdAVE) sendError(err error) error {
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

func (c *cmdAVE) downloadZIPs() error {
	vCh := ftp.NewFileChan(
		c.flagFTP,
		func(name string) bool {
			return strings.Contains(strings.ToLower(name), timeFmt)
		},
		false,
	)

	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}
		c.mapFile[v.File.Name()] = v.File
	}

	return nil
}

func (c *cmdAVE) deleteZIPs(file ...string) error {
	return ftp.Delete(c.flagFTP, file...)
}

func (c *cmdAVE) transformCSVs(file ...string) error {
	for i := range file {
		s := file[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("cmd: ave: file not found '%v'", s)
		}

		rc, err := zip.ExtractFile(f, f.Size())
		if err != nil {
			return err
		}

		cvsMiner := csv.MineRecords(txt.Win1251ToUTF8(rc), ';', 1)
		for v := range cvsMiner {
			if v.Error != nil {
				continue
			}
			switch {
			case s == fileApt:
				c.parseRecordApt(v.Record)
			case s == fileTov:
				c.parseRecordTov(v.Record)
			case s == fileOst:
				c.parseRecordOst(v.Record)
			}
		}
		_ = rc.Close()
	}
	return nil
}

// cvs scheme (apt): [0]codeapt [1]brendname [2]adressapt [3]regimname
func (c *cmdAVE) parseRecordApt(r []string) {
	s := shop{
		ID:   r[0],
		Name: r[1],
		Head: aveHead,
		Addr: r[2],
	}

	// special tuning if [1] is empty
	if s.Name == "" {
		s.Name = s.Head
	}

	c.mapShop[s.ID] = s
}

// cvs scheme (tov): [0]code [1]barname [2]brand [3]grpname [4]grpcode
func (c *cmdAVE) parseRecordTov(r []string) {
	d := drug{
		ID:   r[0],
		Name: r[1],
	}

	c.mapDrug[d.ID] = d
}

// cvs scheme (ost): [0]codegood [1]codeapt [2]qnt [3]pricesale
func (c *cmdAVE) parseRecordOst(r []string) {
	s, ok := c.mapShop[r[1]]
	if !ok {
		return
	}

	d, ok := c.mapDrug[r[0]]
	if !ok {
		return
	}

	p := prop{
		ID:    d.ID,
		Name:  d.Name,
		Quant: mustParseFloat64(r[2]),
		Price: mustParseFloat64(r[3]),
	}

	c.mapProp[s.ID] = append(c.mapProp[s.ID], p)
}

func mustParseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (c *cmdAVE) uploadGzipJSONs() error {
	b := &bytes.Buffer{}

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

	for k, v := range c.mapProp {
		p := price{
			Meta: c.mapShop[k],
			Data: v,
		}

		b.Reset()
		w.Reset(b)

		if err = json.NewEncoder(w).Encode(p); err != nil {
			return err
		}

		if err = w.Close(); err != nil {
			return err
		}

		if err = c.pushGzip(b); err != nil {
			return err
		}
	}

	return nil
}

func (c *cmdAVE) pushGzip(r io.Reader) error {
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

	if err = res.Body.Close(); err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("ave: push failed with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdAVE) failFast() error {
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
		return fmt.Errorf("ave: fail fast with code %d", res.StatusCode)
	}

	return nil
}

func (c *cmdAVE) makeURL(path string) string {
	return c.flagWEB + path
}
