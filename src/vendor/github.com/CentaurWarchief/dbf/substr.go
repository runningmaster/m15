package dbf

func substr(data []byte, position, length int) []byte {
	l := position + length

	if l > len(data) {
		l = len(data)
	}

	return data[position:l]
}
