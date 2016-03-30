package godbf

type UTF8Decoder struct {
}

func (d *UTF8Decoder) Decode(val []byte) ([]byte, error) {
	return val, nil
}
