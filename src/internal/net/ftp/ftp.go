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
	Body() io.Reader
	Unzip() (io.ReadCloser, error)
}

type file struct {
	name string
	size int64
	time time.Time
	data []byte
}

func (f file) Name() string {
	return f.name
}

func (f file) Size() int64 {
	return f.size
}

func (f file) Time() time.Time {
	return f.time
}

func (f file) Body() io.Reader {
	return bytes.NewReader(f.data)
}

func (f file) Unzip() (io.ReadCloser, error) {
	zip, err := zip.NewReader(bytes.NewReader(f.data), f.size)
	if err != nil {
		return nil, err
	}

	if len(zip.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return zip.File[0].Open()
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
					name: v.Name,
					size: int64(v.Size),
					time: v.Time,
					data: data,
				},
				err,
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
