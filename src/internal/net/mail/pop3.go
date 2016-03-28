package mail

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	pop3 "github.com/bytbox/go-pop3"
)

func PullMessages(rawurl string) <-chan func() (io.Reader, error) {
	pipe := make(chan func() (io.Reader, error))
	go func() {
		defer close(pipe)
		pushError := func(err error) bool {
			if err != nil {
				pipe <- func() (io.Reader, error) {
					return nil, err
				}
				return true
			}
			return false
		}

		u, err := url.Parse(rawurl)
		if pushError(err) {
			return
		}
		var user, pass string
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}

		c, err := pop3.Dial(u.Host)
		if pushError(err) {
			return
		}
		defer func() {
			if c == nil {
				return
			}
			err := c.Quit()
			if pushError(err) {
				return
			}
		}()

		err = c.Auth(user, pass)
		if pushError(err) {
			return
		}

		list, _, err := c.ListAll()
		if pushError(err) {
			return
		}

		for i := range list {
			n := list[i]
			msg, err := c.Retr(n)
			if pushError(err) {
				return
			}
			pipe <- func() (io.Reader, error) {
				return strings.NewReader(msg), nil

			}
			err = c.Dele(n)
			if pushError(err) {
				return
			}
		}
	}()

	return pipe
}

// FIXME
func MineAttachGZ(body io.Reader, n int) (sbj string, gzs []func() (string, io.Reader), err error) {
	errFunc := func(err error) (string, []func() (string, io.Reader), error) {
		return "", nil, err
	}

	if n == 0 {
		return errFunc(io.EOF)
	}

	msg, err := mail.ReadMessage(body)
	if err != nil {
		return errFunc(err)
	}

	bnd, err := findBoundary(msg.Header)
	if err != nil {
		return errFunc(err)
	}

	mpr := multipart.NewReader(msg.Body, bnd)      // multipart reader
	gzs = make([]func() (string, io.Reader), 0, 1) // output
	for {
		p, err := mpr.NextPart()
		if err != nil {
			return errFunc(err)
		}

		enc, err := findContentEnc(p.Header)
		if err != nil {
			continue
		}

		gzs = append(gzs, func() (string, io.Reader) {
			return enc, base64.NewDecoder(base64.StdEncoding, p)
		})
		if len(gzs) == n {
			break
		}
	}

	sbj = msg.Header.Get("Subject")
	return sbj, gzs, nil
}

func findBoundary(header mail.Header) (string, error) {
	mtype, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(mtype, "multipart/") {
		return "", err
	}

	boundary := params["boundary"]
	if boundary == "" {
		return "", fmt.Errorf("boundary not found")
	}

	return boundary, nil
}

func findContentEnc(header textproto.MIMEHeader) (string, error) {
	notFound := func() (string, error) {
		return "", fmt.Errorf("gzip not found")
	}

	foundAttach := strings.Contains(header.Get("Content-Disposition"), "attachment")
	foundBase64 := strings.Contains(header.Get("Content-Transfer-Encoding"), "base64")
	if !(foundAttach && foundBase64) {
		return notFound()
	}

	_, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return notFound()
	}

	ext := filepath.Ext(params["name"])
	if !(ext == ".gzip" || ext == ".gz") {
		return notFound()
	}

	return "gzip", nil
}

/*
A send to a nil channel blocks forever
A receive from a nil channel blocks forever
A send to a closed channel panics
A receive from a closed channel returns the zero value immediately

1. skynet@morion.ua

2. 1 email => 1 pharmacy (drugstore) => 1 attachment => GZIP*(JSON, UTF-8)

3. Subj as JSON => {"key":"<key_value>","tag":"<tag_value>"}

GZIP RFC 1952 https://www.ietf.org/rfc/rfc1952.txt
www.7-zip.org
www.gzip.org

*/
