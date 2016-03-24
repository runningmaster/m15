package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"internal/encoding/csv"
	"internal/encoding/txt"
	"internal/net/ftp"
	"internal/version"

	"github.com/google/subcommands"
	"github.com/pivotal-golang/bytefmt"
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

	ave = &cmdAVE{
		mapFile: make(map[string]ftp.Filer, capFile),
		mapShop: make(map[string]shop, capShop),
		mapDrug: make(map[string]drug, capDrug),
		mapProp: make(map[string][]prop, capProp),
	}
)

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

type cmdAVE struct {
	flagFTP string
	flagWEB string
	mapFile map[string]ftp.Filer
	mapShop map[string]shop
	mapDrug map[string]drug
	mapProp map[string][]prop
}

// Name returns the name of the command.
func (c *cmdAVE) Name() string {
	return "ave"
}

// Synopsis returns a short string (less than one line) describing the command.
func (c *cmdAVE) Synopsis() string {
	return "download, transform and send to skynet 3 zip(csv) files from ftp"
}

// Usage returns a long string explaining the command and giving usage information.
func (c *cmdAVE) Usage() string {
	return fmt.Sprintf("%s %s", version.Stamp.AppName(), c.Name())
}

// SetFlags adds the flags for this command to the specified set.
func (c *cmdAVE) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.flagFTP, "ftp", "", "network address for FTP server 'ftp://user:pass@host:port'")
	f.StringVar(&c.flagWEB, "web", "", "network address for WEB server 'scheme://domain/method'")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdAVE) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	var err error

	if err = c.downloadZIPFiles(); err != nil {
		fmt.Println(err)
	}

	if err = c.transformCSVFiles(); err != nil {
		fmt.Println(err)
	}

	if err = c.prepareJSONFiles(); err != nil {
		fmt.Println(err)
	}

	if err != nil {
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func (c *cmdAVE) downloadZIPFiles() error {
	ftpMiner := ftp.MineFiles(
		c.flagFTP,
		func(name string) bool {
			return strings.Contains(strings.ToLower(name), timeFmt)
		},
		false,
	)

	for v := range ftpMiner {
		if v.Error != nil {
			return v.Error
		}
		c.mapFile[v.File.Name()] = v.File
	}

	return nil
}

func (c *cmdAVE) transformCSVFiles() error {
	for i := range walkWay {
		s := walkWay[i]
		f, ok := c.mapFile[s]
		if !ok {
			return fmt.Errorf("cmd: ave: file not found '%v'", s)
		}

		rc, err := f.Unzip()
		if err != nil {
			return err
		}

		count_all := 0
		count_err := 0
		cvsMiner := csv.MineRecords(txt.Win1251ToUTF8(rc), ';', 1)
		for v := range cvsMiner {
			count_all++
			if v.Error != nil {
				count_err++
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
		fmt.Println(s, count_all, count_err, bytefmt.ByteSize(uint64(f.Size())))
		_ = rc.Close()
	}
	return nil
}

// cvs scheme (apt): [0]codeapt [1]brendname [2]adressapt [3]regimname
func (c *cmdAVE) parseRecordApt(r []string) {
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
func (c *cmdAVE) parseRecordTov(r []string) {
	d := drug{
		ID:   r[0],
		Name: r[1],
	}

	c.mapDrug[d.ID] = d
}

// cvs scheme (ost): [0]codegood [1]codeapt [2]qnt [3]pricesale
func (c *cmdAVE) parseRecordOst(r []string) {
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

func mustParseFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (c *cmdAVE) prepareJSONFiles() error {
	t := time.Now()
	for k, v := range c.mapProp {
		p := price{
			Meta: c.mapShop[k],
			Data: v,
		}

		bts, err := json.MarshalIndent(p, "", "\t")
		if err != nil {
			return err
		}

		if err = ioutil.WriteFile(fmt.Sprintf("./json/%v.json", k), bts, 0666); err != nil {
			return err
		}
	}
	fmt.Println(len(c.mapShop), len(c.mapDrug), len(c.mapProp))
	fmt.Println(time.Since(t))
	return nil
}
