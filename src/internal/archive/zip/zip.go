package zip

import (
	"archive/zip"
	"fmt"
	"io"
)

// ExtractFile extracts first file from zip archive
func ExtractFile(r io.ReaderAt, size int64) (io.ReadCloser, error) {
	zip, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	if len(zip.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return zip.File[0].Open()
}
