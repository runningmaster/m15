package ftputil

import (
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
	io.Reader
	io.ReaderAt
	Name() string
	Size() int64
	Time() time.Time
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

func (f file) ReadAt(b []byte, off int64) (int, error) {
	return f.r.ReadAt(b, off)
}

func newFTP(addr string) (*ftp.ServerConn, error) {
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

	err = c.Login(user, pass)
	if err != nil {
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
	defer func() {
		if body != nil {
			_ = body.Close()
		}
	}()

	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Delete deletes files
func Delete(addr string, name ...string) error {
	c, err := newFTP(addr)
	if err != nil {
		return err
	}
	defer func() {
		if c != nil {
			_ = c.Quit()
		}
	}()

	for i := range name {
		err = c.Delete(name[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// NewFileChan allows to work with files from FTP server in a pipe style
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
		var (
			c   *ftp.ServerConn
			l   []*ftp.Entry
			err error
		)
		defer func() {
			if c != nil {
				_ = c.Quit()
			}
			close(pipe)
		}()

		c, err = newFTP(addr)
		if err != nil {
			goto fail
		}

		l, err = c.List(".")
		if err != nil {
			goto fail
		}

		for _, v := range l {
			if skipFile(v, nameOK) {
				continue
			}

			var b []byte
			b, err = readFile(c, v.Name)
			if err != nil {
				goto fail
			}

			if cleanup {
				err = c.Delete(v.Name)
				if err != nil {
					goto fail
				}
			}

			pipe <- makeResult(
				file{
					r:    bytes.NewReader(b),
					name: v.Name,
					time: v.Time,
				},
				nil,
			)
		}

		return // success
	fail:
		pipe <- makeResult(nil, err)
	}()

	return pipe
}
