package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
	"unicode/utf8"

	"internal/ftputil"
	"internal/ziputil"

	dbf "github.com/CentaurWarchief/godbf"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

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
	err := c.failFast()
	if err != nil {
		return err
	}

	err = c.downloadZIPs()
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

type cp866Decoder struct{}

func (d *cp866Decoder) Decode(in []byte) ([]byte, error) {
	if utf8.Valid(in) {
		return in, nil
	}

	r := transform.NewReader(bytes.NewReader(in), charmap.CodePage866.NewDecoder())

	return ioutil.ReadAll(r)
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
