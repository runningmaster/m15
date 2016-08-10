package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"internal/mailutil"
)

type cmdFoz struct {
	cmdBase
}

func newCmdFoz() *cmdFoz {
	cmd := &cmdFoz{}
	cmd.mustInitBase(cmd, "foz", "download and send to skynet gzip(json) files from email")
	return cmd
}

func (c *cmdFoz) exec() error {
	err := c.failFast()
	if err != nil {
		return err
	}

	return c.downloadAndPushGzips()
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
