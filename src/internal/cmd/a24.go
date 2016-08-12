package cmd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"internal/csvutil"
	"internal/txtutil"
)

// Data structs

type shop1 struct {
	Code   string `json:",omitempty"`
	Name   string `json:",omitempty"`
	Head   string `json:",omitempty"`
	Addr   string `json:",omitempty"`
	EGRPOU string `json:",omitempty"`
	File   string `json:",omitempty"`
}

type prop1 struct {
	Code  string  `json:",omitempty"`
	Name  string  `json:",omitempty"`
	Desc  string  `json:",omitempty"`
	Addr  string  `json:",omitempty"`
	Link  string  `json:",omitempty"`
	Quant float64 `json:",omitempty"`
	Price float64 `json:",omitempty"`
}

type price1 struct {
	Meta shop1   `json:",omitempty"`
	Data []prop1 `json:",omitempty"`
}

type linkXML struct {
	Offers []offer `xml:"shop>offers>offer"`
}

type offer struct {
	ID    string  `xml:"id,attr"`
	URL   string  `xml:"url"`
	Price float64 `xml:"price"`
	Name  string  `xml:"name"`
	Vend  string  `xml:"vendor"`
}

// Command

type cmdA24 struct {
	cmdBase

	flagXML string
	flagCSV string

	mapXML  map[string]offer
	mapShop map[string]shop1
	mapFile map[string]io.Reader
	mapProp map[string][]prop1
}

func newCmdA24() *cmdA24 {
	cmd := &cmdA24{
		mapXML:  make(map[string]offer, 20000),
		mapShop: make(map[string]shop1, 30),
		mapFile: make(map[string]io.Reader, 30),
		mapProp: make(map[string][]prop1, 20000),
	}
	cmd.mustInitBase(cmd, "a24", "download and send to skynet gzip(json) files from site")
	return cmd
}

func (c *cmdA24) setFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagXML, "xml", "", "source scheme://user:pass@host:port[,...]")
	f.StringVar(&c.flagCSV, "csv", "", "source scheme://user:pass@host:port[,...]")
}

func (c *cmdA24) exec() error {
	err := c.failFast()
	if err != nil {
		return err
	}

	err = c.downloadXML()
	if err != nil {
		return err
	}

	err = c.downloadCSVs()
	if err != nil {
		return err
	}

	//err = c.transformXML()
	//if err != nil {
	//	return err
	//}

	err = c.transformCSVs()
	if err != nil {
		return err
	}

	return c.uploadGzipJSONs()
}

func (c *cmdA24) downloadXML() error {
	r, err := c.pullData(c.flagXML)
	if err != nil {
		return err
	}

	v := linkXML{}
	err = xml.NewDecoder(r).Decode(&v)
	if err != nil {
		return err
	}

	for i := range v.Offers {
		c.mapXML[v.Offers[i].ID] = v.Offers[i]
	}

	return nil
}

func (c *cmdA24) downloadCSVs() error {
	r, err := c.pullData(c.flagSRC)
	if err != nil {
		return err
	}

	vCh := csvutil.NewRecordChan(txtutil.Win1251ToUTF8(r), ';', true, 1)
	for v := range vCh {
		if v.Error != nil {
			continue
		}
		c.parseRecordList(v.Record)
	}

	for k, v := range c.mapShop {
		r, err = c.pullData("http://" + v.File)
		if err != nil {
			log.Println(c.name, v.File)
			//return err
			continue
		}
		c.mapFile[k] = r
	}

	return nil
}

func (c *cmdA24) transformXML() error {
	quant, err := strconv.ParseFloat("5", 64)
	if err != nil {
		return err
	}
	for k := range c.mapShop {
		for _, v := range c.mapXML {
			c.mapProp[k] = append(c.mapProp[k],
				prop1{
					Code:  v.ID,
					Name:  strings.TrimSpace(fmt.Sprintf("%s %s", v.Name, v.Vend)),
					Addr:  v.URL,
					Link:  v.URL,
					Quant: quant,
					Price: v.Price,
				})
		}
	}
	return nil
}

func (c *cmdA24) transformCSVs() error {
	var r io.Reader
	for k, v := range c.mapShop {
		r = c.mapFile[k]
		if r == nil {
			continue
		}
		vCh := csvutil.NewRecordChan(txtutil.Win1251ToUTF8(r), ';', true, 1)
		for v := range vCh {
			if v.Error != nil {
				continue
			}
			c.parseRecordFile2(k, v.Record)
		}
		// workaround for json's omitempty
		v.File = ""
		c.mapShop[k] = v
	}
	return nil
}

// cvs scheme (list): [0]ID [1]NAME [2]HEAD [3]ADDR [4]CODE [5]FILE
func (c *cmdA24) parseRecordList(r []string) {
	s := shop1{
		Code:   r[0],
		Name:   r[1],
		Head:   r[2],
		Addr:   r[3],
		EGRPOU: r[4],
		File:   c.flagCSV, //r[5],
	}
	c.mapShop[s.Code] = s
}

// cvs scheme (file): [0]Код товара [1]Товар [2]Производитель [3]НДС % [4]Цена без НДС, грн [5]Цена с НДС, грн
func (c *cmdA24) parseRecordFile(s string, r []string) {
	quant, err := strconv.ParseFloat("1", 64)
	if err != nil {
		return
	}

	price, err := strconv.ParseFloat(strings.Replace(r[5], ",", ".", -1), 64)
	if err != nil {
		return
	}

	l := c.mapXML[r[0]].URL
	p := prop1{
		Code:  r[0],
		Name:  fmt.Sprintf("%s %s", r[1], r[2]),
		Addr:  l,
		Link:  l,
		Quant: quant,
		Price: price,
	}

	c.mapProp[s] = append(c.mapProp[s], p)
}

// cvs scheme (file): [0]Код товара [1]Товар [2]Производитель [3]Факт [4]Упаковка [5]Срок годности [6]Классификация товара [7]Рецептурный отпуск [8]АТС-Классификация [9]АТС-Классификация (код)
func (c *cmdA24) parseRecordFile2(s string, r []string) {
	quant, err := strconv.ParseFloat("5", 64)
	if err != nil {
		return
	}

	v, ok := c.mapXML[r[0]]
	if !ok {
		return
	}

	p := prop1{
		Code:  v.ID,
		Name:  fmt.Sprintf("%s %s", r[1], r[2]),
		Addr:  v.URL,
		Link:  v.URL,
		Quant: quant,
		Price: v.Price,
	}

	c.mapProp[s] = append(c.mapProp[s], p)
}

func (c *cmdA24) uploadGzipJSONs() error {
	b := new(bytes.Buffer)

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

	if len(c.mapProp) == 0 {
		return fmt.Errorf("%s: offers not found", c.name)
	}

	var n int
	for k, v := range c.mapProp {
		p := price1{
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
		err = c.pushGzipV1(b, fmt.Sprintf("%s (%d) %s %d", c.name, n, p.Meta.Code, len(p.Data)))
		if err != nil {
			return err
		}

		//err = ioutil.WriteFile(k+".gz", b.Bytes(), 0666)
		//if err != nil {
		//	return err
		//}
	}

	return nil
}
