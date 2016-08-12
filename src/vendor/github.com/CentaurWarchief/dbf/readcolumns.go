package dbf

import (
	"bytes"
	"encoding/binary"
	"io"
)

func columnName(column [11]byte) string {
	position := bytes.IndexByte(column[:11], 0x00)

	if position == -1 {
		return string(column[:11])
	}

	return string(column[0:position])
}

func readColumns(header *header, reader io.ReadSeeker) []*Column {
	columns := make([]*Column, 0, header.fieldCount())
	position := 1

	for i := 0; i < header.fieldCount(); i++ {
		column := &struct {
			Name    [11]byte
			Type    byte
			Address uint32
			Length  uint8
			_       uint8
			_       [2]byte
			_       uint8
			_       [2]byte
			_       byte
			_       [7]byte
			_       byte
		}{}

		if err := binary.Read(reader, binary.LittleEndian, column); err != nil {
			return nil
		}

		columns = append(columns, &Column{
			Name:     columnName(column.Name),
			Type:     column.Type,
			Address:  column.Address,
			Length:   column.Length,
			Position: position,
		})

		position += int(column.Length)
	}

	return columns
}
