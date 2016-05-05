package ziputil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
)

// ExtractFile extracts first file from zip archive
func ExtractFile(r io.Reader) (io.ReadCloser, error) {
	var b []byte
	var err error

	switch r := r.(type) {
	case *bytes.Buffer:
		b = r.Bytes()
	default:
		b, err = ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
	}

	br := bytes.NewReader(b)
	zip, err := zip.NewReader(br, br.Size())
	if err != nil {
		return nil, err
	}

	if len(zip.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return zip.File[0].Open()
}
