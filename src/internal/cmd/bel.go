package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"internal/ftputil"
	"internal/ziputil"

	"github.com/CentaurWarchief/dbf"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Data structs

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

// Command

type cmdBel struct {
	cmdBase

	mapFile map[string]ftputil.Filer
	mapDele map[string][]string // for clean up
	mapJSON map[string]priceOld
}

func newCmdBel() *cmdBel {
	cmd := &cmdBel{
		mapFile: make(map[string]ftputil.Filer, 100),
		mapDele: make(map[string][]string, 100),
		mapJSON: make(map[string]priceOld, 100),
	}
	cmd.mustInitBase(cmd, "bel", "download, transform and send to skynet zip(dbf) files from ftp")
	return cmd
}

func (c *cmdBel) exec() error {
	err := c.downloadZIPs()
	if err != nil {
		return err
	}

	err = c.transformDBFs()
	if err != nil {
		return err
	}

	return c.uploadGzipJSONs()

	//err = c.deleteZIPs()
	//if err != nil {
	//	return err
	//}
}

func (c *cmdBel) downloadZIPs() error {
	splitFlag := strings.Split(c.flagSRC, ",")
	for i := range splitFlag {
		vCh := ftputil.NewFileChan(
			splitFlag[i],
			nil,
			true,
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
		return ftputil.Delete(k, v...)
	}

	return nil
}

func (c *cmdBel) transformDBFs() error {
	for k, v := range c.mapFile {
		rc, err := ziputil.ExtractFile(v)
		if err != nil {
			return err
		}

		f, err := ioutil.ReadAll(rc)
		if err != nil {
			return err
		}
		_ = rc.Close()

		t, err := dbf.NewTableFromReader(bytes.NewReader(f))
		if err != nil {
			return err
		}
		l := t.ReadAll()

		var (
			name, date string
			items      = make([]item, 0, len(l))
			cp866      = &cp866Decoder{new(bytes.Buffer)}
		)

		for i := range l {
			if i == 0 {
				name = cp866.DecodeString(intfToString(l[i]["APTEKA"]))
				date = intfToTimeAsString(l[i]["DATE"])
			}

			items = append(items, item{
				Code: "",
				Drug: drugPlusMaker(
					cp866.DecodeString(intfToString(l[i]["TOVAR"])),
					cp866.DecodeString(intfToString(l[i]["PROIZV"])),
				),
				QuantInp: intfToFloat64(l[i]["APTIN"]),
				QuantOut: intfToFloat64(l[i]["OUT"]),
				PriceInp: intfToFloat64(l[i]["PRICEIN"]),
				PriceOut: intfToFloat64(l[i]["PRICE"]),
				PriceRoc: intfToFloat64(l[i]["ROC"]),
				Balance:  intfToFloat64(l[i]["KOLSTAT"]),
				BalanceT: intfToFloat64(l[i]["AMOUNT"]),
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
	b := new(bytes.Buffer)

	w, err := gzip.NewWriterLevel(b, gzip.DefaultCompression)
	if err != nil {
		return err
	}

	var n int
	for _, v := range c.mapJSON {
		b.Reset()
		w.Reset(b)

		err = json.NewEncoder(w).Encode(v)
		if err != nil {
			return err
		}

		err = w.Close()
		if err != nil {
			return err
		}

		n++
		s := fmt.Sprintf("%s (%d)", c.name, n)
		if len(v.Data) > 0 {
			s = fmt.Sprintf("%s %s %d", s, v.Data[0].Head.Source, len(v.Data[0].Item))
		} else {
			s = fmt.Sprintf("%s %s %d", s, "?", 0)
		}

		err = c.pushGzipV1(b, s)
		if err != nil {
			return err
		}

		//err = ioutil.WriteFile(strings.Replace(k, ".zip", ".json", -1)+".gz", b.Bytes(), 0666)
		//if err != nil {
		//	return err
		//}
	}

	return nil
}

// Util funcs

type cp866Decoder struct {
	buf *bytes.Buffer
}

func (d *cp866Decoder) DecodeString(s string) string {
	if utf8.Valid([]byte(s)) {
		return s
	}

	r := transform.NewReader(strings.NewReader(s), charmap.CodePage866.NewDecoder())

	if d.buf == nil {
		d.buf = new(bytes.Buffer)
	}
	d.buf.Reset()
	_, _ = io.Copy(d.buf, r)

	return d.buf.String()
}

func intfToString(v interface{}) string {
	if s, ok := v.(string); ok {
		return string(s)
	}

	return ""
}

func intfToFloat64(v interface{}) float64 {
	var f float64
	if s, ok := v.(string); ok {
		s = strings.Replace(s, ",", ".", -1)
		f, _ = strconv.ParseFloat(s, 64)
	}

	return f
}

func intfToTimeAsString(v interface{}) string {
	if t, ok := v.(time.Time); ok {
		return t.Format("02.01.2006")
	}

	return ""
}

func drugPlusMaker(name, maker string) string {
	if !strings.Contains(strings.ToLower(name), strings.ToLower(maker)) {
		return fmt.Sprintf("%s %s", name, maker)
	}
	return name
}
