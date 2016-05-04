package mailutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"

	pop3 "github.com/bytbox/go-pop3"
)

// Filer is representation for attach in mail message from POP3
type Filer interface {
	io.Reader
	Name() string
	Subj() string
}

type file struct {
	r    *bytes.Reader
	name string
	subj string
}

func (f file) Name() string {
	return f.name
}

func (f file) Subj() string {
	return f.subj
}

func (f file) Read(b []byte) (int, error) {
	return f.r.Read(b)
}

func newPOP3(addr string) (*pop3.Client, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	if u.User == nil {
		return nil, fmt.Errorf("pop3: user must be defined")
	}

	user := u.User.Username()
	pass, _ := u.User.Password()

	c, err := pop3.Dial(u.Host)
	if err != nil {
		return nil, err
	}

	err = c.Auth(user, pass)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// NewMailChan allows to work with messages from POP3 server in a pipe style
func NewMailChan(addr string, cleanup bool) <-chan struct {
	Mail  io.Reader
	Error error
} {
	var (
		pipe = make(chan struct {
			Mail  io.Reader
			Error error
		})
		makeResult = func(r io.Reader, err error) struct {
			Mail  io.Reader
			Error error
		} {
			return struct {
				Mail  io.Reader
				Error error
			}{
				r,
				err,
			}
		}
	)
	go func() {
		defer func() { close(pipe) }()

		var (
			c   *pop3.Client
			l   []int
			m   string
			err error
		)

		c, err = newPOP3(addr)
		if err != nil {
			goto fail
		}
		defer func() { _ = c.Quit() }()

		l, _, err = c.ListAll()
		if err != nil {
			goto fail
		}

		for i := range l {
			m, err = c.Retr(l[i])
			if err != nil {
				goto fail
			}

			if cleanup {
				err = c.Dele(l[i])
				if err != nil {
					goto fail
				}
			}

			pipe <- makeResult(strings.NewReader(m), nil)
		}

		return // success
	fail:
		pipe <- makeResult(nil, err)
	}()

	return pipe
}

func skipFile(h textproto.MIMEHeader, name string, nameOK func(string) bool) bool {
	badBase := !foundBase64Attach(h)
	badName := nameOK != nil && !nameOK(name)
	return badBase || badName
}

func readFile(p io.ReadCloser) ([]byte, error) {
	defer func() { _ = p.Close() }()
	return ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, p))
}

// NewFileChan allows to work with files (attachments) from POP3 server in a pipe style
func NewFileChan(addr string, nameOK func(string) bool, cleanup bool) <-chan struct {
	File  Filer
	Error error
} {
	var (
		pipe = make(chan struct {
			File  Filer
			Error error
		})
		makeResult = func(f Filer, err error) struct {
			File  Filer
			Error error
		} {
			return struct {
				File  Filer
				Error error
			}{
				f,
				err,
			}
		}
	)
	go func() {
		defer func() { close(pipe) }()

		var (
			m   *mail.Message
			s   string
			err error
			vCh = NewMailChan(addr, cleanup)
		)

		for v := range vCh {
			if v.Error != nil {
				goto fail
			}

			m, err = mail.ReadMessage(v.Mail)
			if err != nil {
				goto fail
			}

			s, err = findBoundary(m.Header)
			if err != nil {
				goto fail
			}

			var (
				r = multipart.NewReader(m.Body, s) // multipart reader
				p *multipart.Part
				b []byte
			)
			for {
				p, err = r.NextPart()
				if err != nil {
					if err == io.EOF {
						break
					}
					goto fail
				}

				s, _ = findFileName(p.Header)

				if skipFile(p.Header, s, nameOK) {
					continue
				}

				b, err = readFile(p)
				if err != nil {
					goto fail
				}

				pipe <- makeResult(
					file{
						r:    bytes.NewReader(b),
						name: s,
						subj: m.Header.Get("Subject"),
					},
					nil,
				)
			}

		}

		return // success
	fail:
		pipe <- makeResult(nil, err)
	}()

	return pipe
}

func findBoundary(h mail.Header) (string, error) {
	t, p, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(t, "multipart/") {
		return "", fmt.Errorf("multipart not found")
	}

	if v := p["boundary"]; v != "" {
		return v, nil
	}

	return "", fmt.Errorf("boundary not found")
}

func findFileName(h textproto.MIMEHeader) (string, error) {
	_, p, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return "", err
	}

	if v := p["name"]; v != "" {
		return v, nil
	}

	return "", fmt.Errorf("file name not found")
}

func foundBase64Attach(h textproto.MIMEHeader) bool {
	foundAttach := strings.Contains(h.Get("Content-Disposition"), "attachment")
	foundBase64 := strings.Contains(h.Get("Content-Transfer-Encoding"), "base64")
	return foundAttach && foundBase64
}
