package godbf

type DbfHeader struct {
	Version                    byte
	Year, Month, Day           uint8
	RecordCount                uint32
	HeaderLength, RecordLength uint16
	_                          [16]byte
	TableFlags                 byte
	CodePage                   byte
}
