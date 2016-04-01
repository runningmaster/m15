package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internal/archive/zip"
	"internal/encoding/csv"
	"internal/encoding/txt"
	"internal/log"
	"internal/net/ftp"

	"github.com/google/subcommands"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/net/context"
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
)

func newCmdAve() *cmdAve {
	cmd := &cmdAve{
		mapFile: make(map[string]ftp.Filer, capFile),
		mapShop: make(map[string]shop, capShop),
		mapDrug: make(map[string]drug, capDrug),
		mapProp: make(map[string][]prop, capProp),
	}
	cmd.initBase("ave", "download, transform and send to skynet zip(csv) files from ftp")
	return cmd
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdAve) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadZIPs(); err != nil {
		goto fail
	}

	if err = c.transformCSVs(); err != nil {
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

func (c *cmdAve) downloadZIPs() error {
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

func (c *cmdAve) deleteZIPs() error {
	f := make([]string, 0, len(c.mapFile))
	for k := range c.mapFile {
		f = append(f, k)
	}
	return ftp.Delete(c.flagFTP, f...)
}

func (c *cmdAve) transformCSVs() error {
	for i := range walkWay {
		s := walkWay[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("ave: file not found '%v'", s)
		}

		rc, err := zip.ExtractFile(f, f.Size())
		if err != nil {
			return err
		}

		vCh := csv.NewRecordChan(txt.Win1251ToUTF8(rc), ';', 1)
		for v := range vCh {
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
func (c *cmdAve) parseRecordApt(r []string) {
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
func (c *cmdAve) parseRecordTov(r []string) {
	d := drug{
		ID:   r[0],
		Name: r[1],
	}

	c.mapDrug[d.ID] = d
}

// cvs scheme (ost): [0]codegood [1]codeapt [2]qnt [3]pricesale
func (c *cmdAve) parseRecordOst(r []string) {
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

func (c *cmdAve) uploadGzipJSONs() error {
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

		if err = c.pushGzipV2(b); err != nil {
			return err
		}
	}

	return nil
}

// Util funcs

func mustParseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Data structs

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

type cmdAve struct {
	cmdBase

	mapFile map[string]ftp.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}
