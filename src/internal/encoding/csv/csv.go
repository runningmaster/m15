package csv

import (
	"encoding/csv"
	"io"
)

func MineRecords(f io.Reader, comma rune) <-chan struct {
	Record []string
	Error  error
} {
	pipe := make(chan struct {
		Record []string
		Error  error
	})

	go func() {
		defer close(pipe)

		r := csv.NewReader(f)
		r.Comma = comma

		var rec []string
		var err error
		for {
			if rec, err = r.Read(); err == io.EOF {
				break
			} else if err != nil {
				pipe <- makeRecord(nil, err)
				continue
			}
			pipe <- makeRecord(rec, nil)
		}
	}()

	return pipe
}

func makeRecord(rec []string, err error) struct {
	Record []string
	Error  error
} {
	return struct {
		rec []string
		err error
	}{
		rec,
		err,
	}
}
