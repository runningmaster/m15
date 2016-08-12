package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internal/csvutil"
	"internal/ftputil"
	"internal/txtutil"
	"internal/ziputil"

	"github.com/klauspost/compress/gzip"
)

const (
	capFile = 3
	capShop = 3000
	capDrug = 200000
	capProp = 5000

	aveHead = "АВЕ" // magic const
)

var (
	timeFmt = time.Now().Format("02.01.06")
	fileApt = fmt.Sprintf("apt_%s.zip", timeFmt)  // magic file name
	fileTov = fmt.Sprintf("tov_%s.zip", timeFmt)  // magic file name
	fileOst = fmt.Sprintf("ost_%s.zip", timeFmt)  // magic file name
	walkWay = []string{fileApt, fileTov, fileOst} // strong order files
)

// Data structs

type shop struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Head string `json:"head,omitempty"`
	Addr string `json:"addr,omitempty"`
	Code string `json:"code,omitempty"`
}

type drug struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type prop struct {
	ID    string  `json:"id,omitempty"`
	Name  string  `json:"name,omitempty"`
	Quant float64 `json:"quant,omitempty"`
	Price float64 `json:"price,omitempty"`
}

type price struct {
	Meta shop   `json:"meta,omitempty"`
	Data []prop `json:"data,omitempty"`
}

// Command

type cmdAve struct {
	cmdBase

	mapFile map[string]ftputil.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}

func newCmdAve() *cmdAve {
	cmd := &cmdAve{
		mapFile: make(map[string]ftputil.Filer, capFile),
		mapShop: make(map[string]shop, capShop),
		mapDrug: make(map[string]drug, capDrug),
		mapProp: make(map[string][]prop, capProp),
	}
	cmd.mustInitBase(cmd, "ave", "download, transform and send to skynet zip(csv) files from ftp")
	return cmd
}

func (c *cmdAve) exec() error {
	err := c.failFast()
	if err != nil {
		return err
	}

	err = c.downloadZIPs()
	if err != nil {
		return err
	}

	err = c.transformCSVs()
	if err != nil {
		return err
	}

	err = c.uploadGzipJSONs()
	if err != nil {
		return err
	}

	return c.deleteZIPs()
}

func (c *cmdAve) downloadZIPs() error {
	vCh := ftputil.NewFileChan(
		c.flagSRC,
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
	return ftputil.Delete(c.flagSRC, f...)
}

func (c *cmdAve) transformCSVs() error {
	for i := range walkWay {
		s := walkWay[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("ave: file not found '%v'", s)
		}

		rc, err := ziputil.ExtractFile(f)
		if err != nil {
			return err
		}

		vCh := csvutil.NewRecordChan(txtutil.Win1251ToUTF8(rc), ';', false, 1)
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

	quant, err := strconv.ParseFloat(r[2], 64)
	if err != nil {
		return
	}
	price, err := strconv.ParseFloat(r[3], 64)
	if err != nil {
		return
	}

	p := prop{
		ID:    d.ID,
		Name:  d.Name,
		Quant: quant,
		Price: price,
	}

	c.mapProp[s.ID] = append(c.mapProp[s.ID], p)
}

func (c *cmdAve) uploadGzipJSONs() error {
	b := new(bytes.Buffer)

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

	var n int
	for k, v := range c.mapProp {
		p := price{
			Meta: c.mapShop[k],
			Data: v,
		}

		b.Reset()
		w.Reset(b)

		err = json.NewEncoder(w).Encode(p)
		if err != nil {
			return err
		}

		err = w.Close()
		if err != nil {
			return err
		}

		n++
		err = c.pushGzipV2(b, fmt.Sprintf("%s (%d) %s %d", c.name, n, p.Meta.ID, len(p.Data)))
		if err != nil {
			return err
		}
	}

	return nil
}
