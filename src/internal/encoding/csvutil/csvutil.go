package csvutil

import (
	"encoding/csv"
	"io"
)

// NewRecordChan allows to work with cvs records in a pipe style
func NewRecordChan(f io.Reader, comma rune, lquotes bool, skip int) <-chan struct {
	Record []string
	Error  error
} {
	var (
		pipe = make(chan struct {
			Record []string
			Error  error
		})
		makeResult = func(rec []string, err error) struct {
			Record []string
			Error  error
		} {
			return struct {
				Record []string
				Error  error
			}{
				rec,
				err,
			}
		}
	)

	go func() {
		defer func() { close(pipe) }()

		r := csv.NewReader(f)
		r.Comma = comma
		r.LazyQuotes = lquotes

		var n int
		for {
			rec, err := r.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				pipe <- makeResult(nil, err)
				continue
			}

			n++
			if n <= skip {
				continue
			}
			pipe <- makeResult(rec, nil)
		}
	}()

	return pipe
}
