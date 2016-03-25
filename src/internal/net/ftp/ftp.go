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

// Filer is representation for file from FTP
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

func connect(addr string) (*ftp.ServerConn, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	if u.User == nil {
		return nil, fmt.Errorf("ftp: user must be defined")
	}

	user := u.User.Username()
	pass, _ := u.User.Password()

	c, err := ftp.Connect(u.Host)
	if err != nil {
		return nil, err
	}

	if err = c.Login(user, pass); err != nil {
		return nil, err
	}

	return c, nil
}

func skipFile(e *ftp.Entry, nameOK func(string) bool) bool {
	badType := e.Type != ftp.EntryTypeFile
	badSize := e.Size <= 0
	badName := nameOK != nil && !nameOK(e.Name)
	return badType || badSize || badName
}

func readFile(c *ftp.ServerConn, name string) ([]byte, error) {
	body, err := c.Retr(name)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	if err = body.Close(); err != nil {
		return nil, err
	}

	return data, nil
}

// KillFiles deletes files
func KillFiles(addr string, name ...string) error {
	c, err := connect(addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Quit() }()

	for i := range name {
		if err = c.Delete(name[i]); err != nil {
			return err
		}
	}

	return nil
}

// MineFiles allows to work with files in a pipe style
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

		c, err := connect(addr)
		if err != nil {
			pipe <- makeResult(nil, err)
			return
		}
		defer func() { _ = c.Quit() }()

		list, err := c.List(".")
		if err != nil {
			pipe <- makeResult(nil, err)
			return
		}

		var v *ftp.Entry
		for _, v = range list {
			if skipFile(v, nameOK) {
				continue
			}

			data, err := readFile(c, v.Name)
			if err != nil {
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
