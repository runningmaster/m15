package dbf

import (
	"strings"
	"time"
)

func parseField(fieldType byte, data []byte) interface{} {
	switch string(fieldType) {
	case "D":
		if len(strings.TrimSpace(string(data))) == 8 {
			value, _ := time.Parse("20060102", string(data))
			return value
		}

		return nil
	case "L":
		switch string(data) {
		case "t", "T", "y", "Y", "n", "N", "1":
			return true
		default:
			return false
		}
	default:
		return strings.TrimSpace(string(data))
	}
}
