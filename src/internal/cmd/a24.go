package cmd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"internal/csvutil"
	"internal/txtutil"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

type cmdA24 struct {
	cmdBase

	flagXML string

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
	cmd.initBase("a24", "download and send to skynet gzip(json) files from site")
	return cmd
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdA24) SetFlags(f *flag.FlagSet) {
	(&c.cmdBase).SetFlags(f)
	f.StringVar(&c.flagXML, "xml", "", "source scheme://user:pass@host:port[,...]")
}

func (c *cmdA24) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	err := c.failFast()
	if err != nil {
		goto fail
	}

	err = c.downloadXML()
	if err != nil {
		goto fail
	}

	err = c.downloadCSVs()
	if err != nil {
		goto fail
	}

	err = c.transformXML()
	if err != nil {
		goto fail
	}

	//err = c.transformCSVs()
	//if err != nil {
	//	goto fail
	//}

	err = c.uploadGzipJSONs()
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

	var n int
	for i := range v.Offers {
		c.mapXML[v.Offers[i].ID] = v.Offers[i]
		if v.Offers[i].Price == 0 {
			n++
		}
	}
	fmt.Println("price 0.0 count:", n, len(v.Offers))
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

	//for k, v := range c.mapShop {
	//	r, err = c.pullData("http://" + v.File)
	//	if err != nil {
	//		log.Println(v.File)
	//		//return err
	//		continue
	//	}
	//	c.mapFile[k] = r
	//}

	return nil
}

func (c *cmdA24) transformXML() error {
	quant, err := strconv.ParseFloat("5", 64)
	if err != nil {
		return err
	}
	for k, _ := range c.mapShop {
		for _, v := range c.mapXML {
			c.mapProp[k] = append(c.mapProp[k],
				prop1{
					Code:  v.ID,
					Name:  strings.TrimSpace(fmt.Sprintf("%s %s", v.Name, v.Vend)),
					Addr:  v.Url,
					Link:  v.Url,
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
			c.parseRecordFile(k, v.Record)
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
		File:   r[5],
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

	l := c.mapXML[r[0]].Url
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

func (c *cmdA24) uploadGzipJSONs() error {
	b := new(bytes.Buffer)

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

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

		//err = c.pushGzipV1(b)
		//if err != nil {
		//	return err
		//}

		err = ioutil.WriteFile(k+".gz", b.Bytes(), 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

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
	Url   string  `xml:"url"`
	Price float64 `xml:"price"`
	Name  string  `xml:"name"`
	Vend  string  `xml:"vendor"`
}

/*
<offer id="464.0029">
<url>http://apteka24.ua/peritol-tabletki-4mg-n20/</url>
<price>77.52</price>
<name>Перитол 4 мг N20</name>
<vendor>ЗАТ"Фарм.завод Егіс, Угорщина</vendor>
<param name="Quantity">1</param>
</offer>
*/
