package run

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"internal/encoding/csvutil"
	"internal/encoding/txtutil"
	"internal/net/ftpcli"
)

const (
	stlHead = "STL" // magic const
)

// Command

type cmdStl struct {
	cmdBase
	files []string

	mapFile map[string]ftpcli.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}

func NewCmdStl() *cmdStl {
	cmd := &cmdStl{
		files:   []string{"APT.csv", "SP.csv", "OST.csv"},
		mapFile: make(map[string]ftpcli.Filer, 3),
		mapShop: make(map[string]shop, 20),
		mapDrug: make(map[string]drug, 10000),
		mapProp: make(map[string][]prop, 100000),
	}
	cmd.mustInitBase(cmd, "stl", "download, transform and send to skynet zip(csv) files from ftp")
	return cmd
}

func (c *cmdStl) exec() error {
	err := c.downloadCSVs()
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

	return c.deleteCSVs()
}

func (c *cmdStl) downloadCSVs() error {
	vCh := ftpcli.NewFileChan(
		c.flagSRC,
		nil,
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

func (c *cmdStl) deleteCSVs() error {
	return ftpcli.Delete(c.flagSRC, c.files...)
}

func (c *cmdStl) transformCSVs() error {
	for i := range c.files {
		s := c.files[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("stl: file not found '%v'", s)
		}

		vCh := csvutil.NewRecordChan(txtutil.Win1251ToUTF8(f), ';', false, 1)
		var err error
		for v := range vCh {
			if v.Error != nil {
				continue
			}
			switch i {
			case 0:
				err = c.parseRecordApt(v.Record)
			case 1:
				err = c.parseRecordSp(v.Record)
			case 2:
				err = c.parseRecordOst(v.Record)
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// cvs scheme (apt): [0]AID [1]NAME
func (c *cmdStl) parseRecordApt(r []string) error {
	csvLen := 2
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	s := shop{
		ID:   strings.TrimSpace(r[0]),
		Name: strings.TrimSpace(r[1]),
		Head: stlHead,
	}

	// special tuning if [1] is empty
	if s.Name == "" {
		s.Name = s.Head
	}

	c.mapShop[s.ID] = s
	return nil
}

// cvs scheme (sp): [0]CODE [1]NAME [2]IZG [3]STRANA
func (c *cmdStl) parseRecordSp(r []string) error {
	csvLen := 4
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	d := drug{
		ID:   strings.TrimSpace(r[0]),
		Name: fmt.Sprintf("%s %s %s", strings.TrimSpace(r[1]), strings.TrimSpace(r[2]), strings.TrimSpace(r[3])),
	}

	c.mapDrug[d.ID] = d
	return nil
}

// cvs scheme (ost): [0]AID [1]CODE [2]QTTY [3]PRICE
func (c *cmdStl) parseRecordOst(r []string) error {
	csvLen := 4
	if len(r) < csvLen {
		return fmt.Errorf("invalid csv: got %d, want %d", len(r), csvLen)
	}

	s, ok := c.mapShop[strings.TrimSpace(r[0])]
	if !ok {
		return fmt.Errorf("shop not found %s", r[0])
	}

	d, ok := c.mapDrug[strings.TrimSpace(r[1])]
	if !ok {
		return fmt.Errorf("drug not found %s", r[1])
	}

	quant, err := strconv.ParseFloat(strings.TrimSpace(r[2]), 64)
	if err != nil {
		return err
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(r[3]), 64)
	if err != nil {
		return err
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

func (c *cmdStl) uploadGzipJSONs() error {
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

		//err = ioutil.WriteFile(k+".gz", b.Bytes(), 0666)
		//i/f err != nil {
		//	return err
		//}
	}

	return nil
}
