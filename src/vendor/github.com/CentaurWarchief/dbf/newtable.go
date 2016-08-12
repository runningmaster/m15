package dbf

import (
	"io"
	"os"
)

func NewTableFromFile(path string) (*Table, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0777)

	if err != nil {
		return nil, err
	}

	return fromReader(file)
}

func NewTableFromReader(reader io.Reader) (*Table, error) {
	return fromReader(reader.(io.ReadSeeker))
}
