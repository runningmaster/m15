package dbf

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
)

func fromReader(reader io.ReadSeeker) (*Table, error) {
	bytes, err := ioutil.ReadAll(reader)

	if err != nil {
		return nil, err
	}

	if _, err := reader.Seek(0, os.SEEK_SET); err != nil {
		return nil, err
	}

	header := &header{}

	if err := binary.Read(reader, binary.LittleEndian, header); err != nil {
		return nil, err
	}

	if int(header.Version) != 3 {
		return nil, ErrUnsupportedVersion
	}

	headerLength := int64(header.HeaderLength)
	fileSize := int64(len(bytes))

	if headerLength > fileSize {
		return nil, ErrUnexpectedHeaderSize
	}

	if (headerLength + (int64(header.RecordCount) * int64(header.RecordByteLength)) - 500) > fileSize {
		return nil, ErrInvalidDBF
	}

	defer func() {
		if header.isFoxPro() {
			reader.Read(make([]byte, 263))
		}

		reader.Seek(int64(header.HeaderLength), os.SEEK_SET)
	}()

	return &Table{
		reader:  reader,
		header:  header,
		columns: readColumns(header, reader),
	}, nil
}
