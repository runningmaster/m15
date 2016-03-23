package ftp

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/jlaffaye/ftp"
)

type Filer interface {
	io.Reader
	Name() string
	Size() int64
	Time() time.Time
	Unzip() (io.ReadCloser, error)
}

type file struct {
	r    *bytes.Reader
	name string
	time time.Time
}

func (f file) Name() string {
	return f.name
}

func (f file) Size() int64 {
	return f.r.Size()
}

func (f file) Time() time.Time {
	return f.time
}

func (f file) Read(b []byte) (int, error) {
	return f.r.Read(b)
}

func (f file) Unzip() (io.ReadCloser, error) {
	z, err := zip.NewReader(f.r, f.r.Size())
	if err != nil {
		return nil, err
	}

	if len(z.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return z.File[0].Open()
}

func MineFiles(addr string, nameOK func(string) bool, cleanup bool) <-chan struct {
	File  Filer
	Error error
} {
	pipe := make(chan struct {
		File  Filer
		Error error
	})

	go func() {
		defer close(pipe)

		var u *url.URL
		var err error
		if u, err = url.Parse(addr); err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		var user, pass string
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}

		var c *ftp.ServerConn
		if c, err = ftp.Connect(u.Host); err != nil {
			pipe <- makeResult(nil, err)
			return
		}
		defer func() { _ = c.Quit() }()

		if err = c.Login(user, pass); err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		var list []*ftp.Entry
		if list, err = c.List("."); err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		var v *ftp.Entry
		var badType, badSize, badName bool
		for _, v = range list {
			badType = v.Type != ftp.EntryTypeFile
			badSize = v.Size <= 0
			badName = nameOK != nil && !nameOK(v.Name)
			if badType || badSize || badName {
				continue
			}

			var body io.ReadCloser
			if body, err = c.Retr(v.Name); err != nil {
				pipe <- makeResult(nil, err)
				return
			}

			var data []byte
			if data, err = ioutil.ReadAll(body); err != nil {
				pipe <- makeResult(nil, err)
				return
			}

			if body != nil {
				if err = body.Close(); err != nil {
					pipe <- makeResult(nil, err)
					return
				}
			}

			pipe <- makeResult(
				file{
					r:    bytes.NewReader(data),
					name: v.Name,
					time: v.Time,
				},
				nil,
			)

			if !cleanup {
				continue
			}

			if err = c.Delete(v.Name); err != nil {
				pipe <- makeResult(nil, err)
				return
			}
		}
	}()

	return pipe
}

func makeResult(f Filer, err error) struct {
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
