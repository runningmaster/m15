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

		u, err := url.Parse(addr)
		if err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		if u.User == nil {
			pipe <- makeResult(nil, fmt.Errorf("ftp: user must be defined"))
			return
		}

		user := u.User.Username()
		pass, _ := u.User.Password()

		c, err := ftp.Connect(u.Host)
		if err != nil {
			pipe <- makeResult(nil, err)
			return
		}
		defer func() { _ = c.Quit() }()

		if err = c.Login(user, pass); err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		list, err := c.List(".")
		if err != nil {
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

			body, err := c.Retr(v.Name)
			if err != nil {
				pipe <- makeResult(nil, err)
				return
			}

			data, err := ioutil.ReadAll(body)
			if err != nil {
				pipe <- makeResult(nil, err)
				return
			}

			if err = body.Close(); err != nil {
				pipe <- makeResult(nil, err)
				return
			}

			if cleanup {
				if err = c.Delete(v.Name); err != nil {
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
