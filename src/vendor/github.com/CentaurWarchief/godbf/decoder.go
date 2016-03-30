package godbf

type Decoder interface {
	Decode(val []byte) ([]byte, error)
}
