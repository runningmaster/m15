package ziputil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

// ExtractFile extracts first file from zip archive
func ExtractFile(r io.Reader) (io.ReadCloser, error) {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, r)
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(buf.Bytes())
	zip, err := zip.NewReader(br, br.Size())
	if err != nil {
		return nil, err
	}

	if len(zip.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return zip.File[0].Open()
}
