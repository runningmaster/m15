package dbf

import (
	"io"
	"time"
)

type Table struct {
	reader  io.ReadSeeker
	header  *header
	columns []*Column
}

func (t *Table) LastModifiedAt() time.Time {
	year := int(t.header.Year)

	if year > 70 {
		year += 1900
	} else {
		year += 2000
	}

	return time.Date(year, time.Month(int(t.header.Month)), int(t.header.Day), 0, 0, 0, 0, time.Local)
}

func (t *Table) NumberOfRecords() int {
	return int(t.header.RecordCount)
}

func (t *Table) NumberOfColumns() int {
	return len(t.columns)
}

func (t *Table) ColumnNames() []string {
	names := make([]string, 0, len(t.columns))

	for _, column := range t.columns {
		names = append(names, column.Name)
	}

	return names
}

func (t *Table) ReadAll() []map[string]interface{} {
	var records []map[string]interface{}

	for i := 0; i < int(t.header.RecordCount); i++ {
		byColumn := make(map[string]interface{})

		record := make([]byte, t.header.RecordByteLength)

		if _, err := t.reader.Read(record); err != nil {
			continue
		}

		for j := 0; j < int(t.header.fieldCount()); j++ {
			column := t.columns[j]

			byColumn[column.Name] = parseField(
				column.Type,
				substr(record, int(column.Position), int(column.Length)),
			)
		}

		records = append(records, byColumn)
	}

	return records
}
