package subcmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"

	"internal/util/csvutil"
	"internal/util/ftputil"
	"internal/util/txtutil"

	"github.com/google/subcommands"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/net/context"
)

const (
	stlHead = "STL" // magic const
)

type cmdStl struct {
	cmdBase
	files []string

	mapFile map[string]ftputil.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}

func newCmdStl() *cmdStl {
	cmd := &cmdStl{
		files:   []string{"APT.csv", "SP.csv", "OST.csv"},
		mapFile: make(map[string]ftputil.Filer, 3),
		mapShop: make(map[string]shop, 20),
		mapDrug: make(map[string]drug, 10000),
		mapProp: make(map[string][]prop, 100000),
	}
	cmd.initBase("stl", "download, transform and send to skynet zip(csv) files from ftp")
	return cmd
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdStl) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	err := c.failFast()
	if err != nil {
		goto fail
	}

	err = c.downloadCSVs()
	if err != nil {
		goto fail
	}

	err = c.transformCSVs()
	if err != nil {
		goto fail
	}

	err = c.uploadGzipJSONs()
	if err != nil {
		goto fail
	}

	err = c.deleteCSVs()
	if err != nil {
		goto fail
	}

	return subcommands.ExitSuccess

fail:
	log.Println(err)
	err = c.sendError(err)
	if err != nil {
		log.Println(err)
	}

	return subcommands.ExitFailure
}

func (c *cmdStl) downloadCSVs() error {
	vCh := ftputil.NewFileChan(
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
	return ftputil.Delete(c.flagSRC, c.files...)
}

func (c *cmdStl) transformCSVs() error {
	for i := range c.files {
		s := c.files[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("stl: file not found '%v'", s)
		}

		vCh := csvutil.NewRecordChan(txtutil.Win1251ToUTF8(f), ';', false, 1)
		for v := range vCh {
			if v.Error != nil {
				continue
			}
			switch i {
			case 0:
				c.parseRecordApt(v.Record)
			case 1:
				c.parseRecordSp(v.Record)
			case 2:
				c.parseRecordOst(v.Record)
			}
		}
	}

	return nil
}

// cvs scheme (apt): [0]AID [1]NAME
func (c *cmdStl) parseRecordApt(r []string) {
	s := shop{
		ID:   r[0],
		Name: r[1],
		Head: stlHead,
	}

	// special tuning if [1] is empty
	if s.Name == "" {
		s.Name = s.Head
	}

	c.mapShop[s.ID] = s
}

// cvs scheme (sp): [0]CODE [1]NAME [2]IZG [3]STRANA
func (c *cmdStl) parseRecordSp(r []string) {
	d := drug{
		ID:   r[0],
		Name: fmt.Sprintf("%s %s %s", r[1], r[2], r[3]),
	}

	c.mapDrug[d.ID] = d
}

// cvs scheme (ost): [0]AID [1]CODE [2]QTTY [3]PRICE
func (c *cmdStl) parseRecordOst(r []string) {
	s, ok := c.mapShop[r[0]]
	if !ok {
		return
	}

	d, ok := c.mapDrug[r[1]]
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

func (c *cmdStl) uploadGzipJSONs() error {
	b := new(bytes.Buffer)

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

		err = json.NewEncoder(w).Encode(p)
		if err != nil {
			return err
		}

		err = w.Close()
		if err != nil {
			return err
		}

		err = c.pushGzipV2(b)
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
