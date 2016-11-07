package run

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internal/archive/ziputil"
	"internal/encoding/csvutil"
	"internal/encoding/txtutil"
	"internal/net/ftpcli"
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

	mapFile map[string]ftpcli.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}

func NewCmdAve() *cmdAve {
	cmd := &cmdAve{
		mapFile: make(map[string]ftpcli.Filer, capFile),
		mapShop: make(map[string]shop, capShop),
		mapDrug: make(map[string]drug, capDrug),
		mapProp: make(map[string][]prop, capProp),
	}
	cmd.mustInitBase(cmd, "ave", "download, transform and send to skynet zip(csv) files from ftp")
	return cmd
}

func (c *cmdAve) exec() error {
	err := c.downloadZIPs()
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
	vCh := ftpcli.NewFileChan(
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
	return ftpcli.Delete(c.flagSRC, f...)
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
				err = c.parseRecordApt(v.Record)
			case s == fileTov:
				err = c.parseRecordTov(v.Record)
			case s == fileOst:
				err = c.parseRecordOst(v.Record)
			}
			if err != nil {
				return err
			}
		}
		_ = rc.Close()
	}
	return nil
}

// cvs scheme (apt): [0]codeapt [1]brendname [2]adressapt [3]regimname
func (c *cmdAve) parseRecordApt(r []string) error {
	csvLen := 3
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	s := shop{
		ID:   strings.TrimSpace(r[0]),
		Name: strings.TrimSpace(r[1]),
		Head: aveHead,
		Addr: strings.TrimSpace(r[2]),
	}

	// special tuning if [1] is empty
	if s.Name == "" {
		s.Name = s.Head
	}

	c.mapShop[s.ID] = s
	return nil
}

// cvs scheme (tov): [0]code [1]barname [2]brand [3]grpname [4]grpcode
func (c *cmdAve) parseRecordTov(r []string) error {
	csvLen := 2
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	d := drug{
		ID:   strings.TrimSpace(r[0]),
		Name: strings.TrimSpace(r[1]),
	}

	c.mapDrug[d.ID] = d
	return nil
}

// cvs scheme (ost): [0]codegood [1]codeapt [2]qnt [3]pricesale
func (c *cmdAve) parseRecordOst(r []string) error {
	csvLen := 4
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	s, ok := c.mapShop[strings.TrimSpace(r[1])]
	if !ok {
		return fmt.Errorf("shop not found %s", r[1])
	}

	d, ok := c.mapDrug[strings.TrimSpace(r[0])]
	if !ok {
		return fmt.Errorf("drug not found %s", r[0])
	}

	quant, err := strconv.ParseFloat(strings.TrimSpace(r[2]), 64)
	if err != nil {
		fmt.Println(err, r[0], r[1], r[2], r[3])
		//return err
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(r[3]), 64)
	if err != nil {
		fmt.Println(err, r[0], r[1], r[2], r[3])
		//return err
	}

	p := prop{
		ID:    d.ID,
		Name:  d.Name,
		Quant: quant,
		Price: price,
	}

	c.mapProp[s.ID] = append(c.mapProp[s.ID], p)
	return nil
}

func (c *cmdAve) uploadGzipJSONs() error {
	b := new(bytes.Buffer)
	w := gzip.NewWriter(b)

	var n int
	var err error
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
