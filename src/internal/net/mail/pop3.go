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

// Filer is representation for attach in mail message from POP3
type Filer interface {
	io.Reader
	Name() string
	Subj() string
}

type file struct {
	r    io.Reader
	subj string
}

func (f file) Subj() string {
	return f.subj
}

func (f file) Read(b []byte) (int, error) {
	return f.r.Read(b)
}

func connect(addr string) (*pop3.Client, error) {
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

func NewMailChan(addr string, cleanup bool) <-chan struct {
	Mail  io.Reader
	Error error
} {
	pipe := make(chan struct {
		Mail  io.Reader
		Error error
	})

	go func() {
		var (
			c   *pop3.Client
			l   []int
			err error
		)
		defer func() {
			if c != nil {
				_ = c.Quit()
			}
			close(pipe)
		}()

		if c, err = connect(addr); err != nil {
			goto fail
		}

		if l, _, err = c.ListAll(); err != nil {
			goto fail
		}

		for i := range l {
			var m string
			if m, err = c.Retr(l[i]); err != nil {
				goto fail
			}

			if cleanup {
				if err = c.Dele(l[i]); err != nil {
					goto fail
				}
			}

			pipe <- makeMail(strings.NewReader(m), nil)
		}
		return // success
	fail:
		pipe <- makeMail(nil, err)
	}()

	return pipe
}

func makeMail(r io.Reader, err error) struct {
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

func FetchFiles(addr string, cleanup bool) <-chan struct {
	File  Filer
	Error error
} {
	pipe := make(chan struct {
		File  Filer
		Error error
	})

	go func() {
		defer close(pipe)
		var (
			m, r io.Reader
			b    string
			err  error
			vCh  = PullMessages(addr, cleanup)
		)
		for v := range vCh {
			if v.Error != nil {
				goto fail
			}

			if m, err = mail.ReadMessage(v.Mail); err != nil {
				goto fail
			}

			if b, err = findBoundary(msg.Header); err != nil {
				goto fail
			}

			var (
				r = multipart.NewReader(msg.Body, b) // multipart reader
				p *multipart.Part
			)
			for {
				if p, err = r.NextPart(); err != nil {
					if err == io.EOF {
						break
					}
					goto fail
				}

				enc, err := findContentEnc(p.Header)
				if err != nil {
					continue
				}

				gzs = append(gzs, func() (string, io.Reader) {
					return enc, base64.NewDecoder(base64.StdEncoding, p)
				})

			}

		}

		return // success
	fail:
		pipe <- makeResult2(nil, err)
	}()

	return pipe
}

func makeResult2(f Filer, err error) struct {
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
		return "", fmt.Errorf("multipart bot found")
	}

	b := params["boundary"]
	if b == "" {
		return "", fmt.Errorf("boundary not found")
	}

	return b, nil
}

func findContentEnc(header textproto.MIMEHeader) (string, error) {
	foundAttach := strings.Contains(header.Get("Content-Disposition"), "attachment")
	foundBase64 := strings.Contains(header.Get("Content-Transfer-Encoding"), "base64")
	if !(foundAttach && foundBase64) {
		goto fail
	}

	_, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		goto fail
	}

	ext := filepath.Ext(params["name"])
	if !(ext == ".gzip" || ext == ".gz") {
		goto fail
	}

	return "gzip", nil // success
fail:
	return "", fmt.Errorf("gzip not found")
}
