package godbf

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnsupportedVersion  = errors.New("unsupported version")
	ErrInvalidHeader       = errors.New("invalid header")
	ErrRecordIsDeleted     = errors.New("record is deleted")
	ErrUnexpectedFlagValue = errors.New("unexpected flag value")
	ErrEOF                 = errors.New("end of file")
)

type DbfReader struct {
	reader  io.ReadSeeker
	header  DbfHeader
	fields  []DbfField
	decoder Decoder
	mutex   *sync.Mutex
}

type DbfRecord map[string]interface{}

func NewReader(reader io.ReadSeeker, decoder Decoder) (*DbfReader, error) {
	if _, err := reader.Seek(0, 0); err != nil {
		return nil, err
	}

	header := &DbfHeader{}

	if err := binary.Read(reader, binary.LittleEndian, header); err != nil {
		return nil, ErrInvalidHeader
	}

	if header.Version != 0x03 {
		return nil, ErrUnsupportedVersion
	}

	if _, err := reader.Seek(32, 0); err != nil {
		return nil, err
	}

	var fields []DbfField

	for i := 0; i < int(header.HeaderLength-1-32)/32; i++ {
		reader.Seek(int64((i*32)+32), 0)

		field := DbfField{}

		binary.Read(reader, binary.LittleEndian, &field)

		fields = append(
			fields,
			field,
		)
	}

	return &DbfReader{
		reader,
		*header,
		fields,
		decoder,
		&sync.Mutex{},
	}, nil
}

func (d *DbfReader) FieldName(index int) string {
	// ASCII-NULL
	return strings.Trim(string(d.fields[index].Name[:]), "\x00")
}

func (d *DbfReader) RecordCount() uint32 {
	return d.header.RecordCount
}

func (d *DbfReader) FieldCount() int {
	return len(d.fields)
}

func (d *DbfReader) Fields() []string {
	fields := make([]string, len(d.fields))

	for i := range d.fields {
		fields[i] = d.FieldName(i)
	}

	return fields
}

func (d *DbfReader) Modified() time.Time {
	year := int(d.header.Year)

	if year > 100 {
		year += 1900
	} else {
		year += 2000
	}

	return time.Date(
		year,
		time.Month(int(d.header.Month)),
		int(d.header.Day),
		0,
		0,
		0,
		0,
		time.Local,
	)
}

func (d *DbfReader) Read(index uint16) (DbfRecord, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if uint32(index+1) > d.header.RecordCount {
		return nil, ErrEOF
	}

	d.reader.Seek(
		int64(d.header.HeaderLength)+(int64(index)*int64(d.header.RecordLength)),
		0,
	)

	var deleted byte

	if err := binary.Read(d.reader, binary.LittleEndian, &deleted); err != nil {
		return nil, err
	}

	if deleted == '*' {
		return nil, ErrRecordIsDeleted
	}

	if deleted != ' ' {
		return nil, ErrUnexpectedFlagValue
	}

	record := DbfRecord{}

	for i, field := range d.fields {
		buf := make([]byte, field.Length)

		if err := binary.Read(d.reader, binary.LittleEndian, &buf); err != nil {
			return nil, err
		}

		val, err := d.parseField(buf, i)

		if err != nil {
			return nil, err
		}

		record[d.FieldName(i)] = val
	}

	return record, nil
}

func (d *DbfReader) parseField(raw []byte, position int) (interface{}, error) {
	switch string(d.fields[position].Type) {
	case "I":
		return int32(binary.LittleEndian.Uint32(raw)), nil
	case "B":
		return math.Float64frombits(binary.LittleEndian.Uint64(raw)), nil
	case "N":
		if d.fields[position].Decimals == 0 {
			value := strings.TrimSpace(string(raw))

			if value == "" {
				return 0, nil
			}

			return strconv.ParseInt(value, 10, 32)
		}
		fallthrough
	case "F":
		return strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	case "D":
		if strings.TrimSpace(string(raw)) == "" {
			return time.Time{}, nil
		}

		return time.Parse("20060102", string(raw))
	case "L":
		switch string(raw) {
		default:
			return false, nil
		case "t", "T", "y", "Y", "1":
			return true, nil
		}
	case "C":
		val, err := d.decoder.Decode(raw)

		if err != nil {
			return string(raw), err
		}

		return string(val), nil
	case "V":
		return raw, nil
	default:
		return strings.TrimSpace(string(raw)), nil
	}
}
