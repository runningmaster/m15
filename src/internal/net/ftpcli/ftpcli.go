package ftpcli

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/jlaffaye/ftp"
)

// Filer is representation for file from FTP
type Filer interface {
	io.Reader
	Name() string
	Time() time.Time
}

type file struct {
	r    io.Reader
	name string
	time time.Time
}

func (f file) Name() string {
	return f.name
}

func (f file) Time() time.Time {
	return f.time
}

func (f file) Read(p []byte) (int, error) {
	return f.r.Read(p)
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
	c.DisableEPSV = true // passive mode

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

func copyFileAndClose(dst io.Writer, src io.ReadCloser) error {
	defer func() { _ = src.Close() }()
	_, err := io.Copy(dst, src)
	return err
}

// Delete deletes files
func Delete(addr string, name ...string) error {
	c, err := newFTP(addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Quit() }()

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
		defer func() { close(pipe) }()

		var (
			c   *ftp.ServerConn
			l   []*ftp.Entry
			r   io.ReadCloser
			b   *bytes.Buffer
			err error
		)

		c, err = newFTP(addr)
		if err != nil {
			goto fail
		}
		defer func() { _ = c.Quit() }()

		l, err = c.List(".")
		if err != nil {
			goto fail
		}

		for _, v := range l {
			if skipFile(v, nameOK) {
				continue
			}

			r, err = c.Retr(v.Name)
			if err != nil {
				goto fail
			}

			b = new(bytes.Buffer)
			err = copyFileAndClose(b, r)
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
					r:    b,
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
