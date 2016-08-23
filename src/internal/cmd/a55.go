package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"internal/mailutil"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/CentaurWarchief/dbf"
)

type cmdA55 struct {
	cmdBase
	files []*bytes.Reader
	jsons [][]byte
}

func newCmdA55() *cmdA55 {
	cmd := &cmdA55{}
	cmd.mustInitBase(cmd, "a55", "download and send to skynet dbf files from site")
	return cmd
}

func (c *cmdA55) exec() error {
	err := c.downloadDBF()
	if err != nil {
		return err
	}

	err = c.transformDBF()
	if err != nil {
		return err
	}

	return c.uploadGzipJSONs()
}

func (c *cmdA55) downloadDBF() error {
	//vCh := mailutil.NewMailChan(c.flagSRC, true)

	vCh := mailutil.NewFileChan(
		c.flagSRC,
		func(name string) bool {
			return strings.HasPrefix(strings.ToLower(filepath.Ext(name)), ".dbf")
		},
		false, // FIXME
	)

	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}

		f, err := ioutil.ReadAll(v.File)
		if err != nil {
			return err
		}

		c.files = append(c.files, bytes.NewReader(f))
	}

	return nil

}

func (c *cmdA55) transformDBF() error {
	for i := range c.files {
		t, err := dbf.NewTableFromReader(c.files[i])
		if err != nil {
			return err
		}
		l := t.ReadAll()

		p := price1{
			Meta: shop1{
				Name:   "",
				Head:   "",
				Addr:   "",
				EGRPOU: "",
			},
			Data: make([]prop1, 0, len(l)),
		}
		cp866 := &cp866Decoder{new(bytes.Buffer)}

		for i := range l {
			if i == 0 {
				continue
			}
			p.Data = append(p.Data, prop1{
				Code: intfToString(l[i]["KOD"]),
				Name: drugPlusMaker(
					cp866.DecodeString(intfToString(l[i]["NAME"])),
					cp866.DecodeString(intfToString(l[i]["PROIZVODIT"])),
				),
				Quant: 5,
				Price: intfToFloat64(l[i]["CENA"]),
			})
		}

		b, err := json.Marshal(p)
		if err != nil {
			return err
		}

		c.jsons = append(c.jsons, b)
	}
	return nil
}

func (c *cmdA55) uploadGzipJSONs() error {
	var err error
	for i := range c.jsons {
		//err = c.pushGzipV1(bytes.NewReader(c.jsons[i]), fmt.Sprintf("%s (%d)", c.name, i))
		//if err != nil {
		//	return err
		//}
		err = ioutil.WriteFile(fmt.Sprintf("%d.json", i), c.jsons[i], 0666)
		if err != nil {
			return err
		}
	}
	return nil
}
