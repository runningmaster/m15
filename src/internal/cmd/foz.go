package cmd

import (
	"encoding/json"
	"flag"
	"path/filepath"
	"strings"

	"internal/log"
	"internal/net/mail"

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
	var err error

	if err = c.failFast(); err != nil {
		goto fail
	}

	if err = c.downloadAndPushGzips(); err != nil {
		goto fail
	}

	return subcommands.ExitSuccess

fail:
	log.Println(err)
	if err = c.sendError(err); err != nil {
		log.Println(err)
	}
	return subcommands.ExitFailure
}

func (c *cmdFoz) downloadAndPushGzips() error {
	vCh := mail.NewFileChan(
		c.flagPOP,
		func(name string) bool {
			return strings.HasPrefix(strings.ToLower(filepath.Ext(name)), ".gz")
		},
		true,
	)

	var err error
	for v := range vCh {
		if v.Error != nil {
			return v.Error
		}

		if key, tag, ok := extractKeyTag(v.File.Subj()); ok {
			c.flagKey, c.flagTag = key, tag
		}

		if err = c.pushGzipV1(v.File); err != nil {
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

	if err := json.NewDecoder(strings.NewReader(s)).Decode(&subj); err != nil {
		return "", "", false
	}

	return subj.Key, subj.Tag, subj.Key != "" && subj.Tag != ""
}
