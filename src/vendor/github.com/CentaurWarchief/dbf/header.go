package dbf

type header struct {
	Version          uint8
	Year, Month, Day uint8
	RecordCount      uint32
	HeaderLength     uint16
	RecordByteLength uint16
	_                [2]byte
	Transaction      byte
	Encrypted        byte
	_                [4]byte
	_                [8]byte
	MDX              byte
	LanguageCode     byte
	_                [2]byte
}

func (h *header) fieldCount() int {
	if h.isFoxPro() {
		return int((h.HeaderLength - 296)) / 32
	}

	return int((h.HeaderLength - 33)) / 32
}

func (h *header) isFoxPro() bool {
	switch int(h.Version) {
	case 48: // 0x30
	case 49: // 0x31
	case 245: // 0xF5
	case 251: // 0xFB
		return true
	}

	return false
}
