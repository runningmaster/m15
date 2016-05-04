package subcmd

import (
	"encoding/json"
	"flag"
	"log"
	"path/filepath"
	"strings"

	"internal/net/mailutil"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

type cmdFoz struct {
	cmdBase
}

func newCmdFoz() *cmdFoz {
	cmd := &cmdFoz{}
	cmd.initBase("foz", "download and send to skynet gzip(json) files from email")
	return cmd
}

// Execute executes the command and returns an ExitStatus.
func (c *cmdFoz) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	err := c.failFast()
	if err != nil {
		goto fail
	}

	err = c.downloadAndPushGzips()
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

func (c *cmdFoz) downloadAndPushGzips() error {
	vCh := mailutil.NewFileChan(
		c.flagSRC,
		func(name string) bool {
			return strings.HasPrefix(strings.ToLower(filepath.Ext(name)), ".gz")
		},
		true,
	)

	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}

		if key, tag, ok := extractKeyTag(v.File.Subj()); ok {
			c.flagKey, c.flagTag = key, tag
		}

		err := c.pushGzipV1(v.File)
		if err != nil {
			return err
		}
	}

	return nil
}

// Util func
func extractKeyTag(s string) (key string, tag string, ok bool) {
	subj := struct {
		Key string `json:"key"`
		Tag string `json:"tag"`
	}{}

	err := json.NewDecoder(strings.NewReader(s)).Decode(&subj)
	if err != nil {
		return "", "", false
	}

	return subj.Key, subj.Tag, subj.Key != "" && subj.Tag != ""
}
