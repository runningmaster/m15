package godbf

type DbfField struct {
	Name     [11]byte
	Type     byte
	Position uint32
	Length   uint8
	Decimals uint8
	Flags    byte
	Next     uint32
	Step     uint16
	Reserved [8]byte
}
