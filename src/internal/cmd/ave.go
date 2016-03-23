package cmd

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"internal/net/ftp"
	"internal/version"

	"github.com/google/subcommands"
	"github.com/pivotal-golang/bytefmt"
	"golang.org/x/net/context"
)

type cmdAVE struct {
	addrFTP string
	addrWEB string
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
	f.StringVar(&c.addrFTP, "ftp", "", "network address for FTP server 'ftp://user:pass@host:port'")
	f.StringVar(&c.addrWEB, "web", "", "network address for WEB server 'scheme://domain/method'")
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdAVE) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	fmt.Println("Execute", c.Name())
	c.downloadData()
	return subcommands.ExitSuccess
}

func (c *cmdAVE) downloadData() {
	ftpMiner := ftp.MineFiles(c.addrFTP, func(name string) bool {
		return strings.Contains(strings.ToLower(name), time.Now().Format("02.01.06"))
	}, false)

	var (
		f   ftp.Filer
		err error
		//wg  sync.WaitGroup
	)
	for v := range ftpMiner {
		if f, err = v.File, v.Error; err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(f.Name(), bytefmt.ByteSize(uint64(f.Size())))
	}
}

func (c *cmdAVE) prepareJSONs() {

}
