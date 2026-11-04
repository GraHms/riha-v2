package sheets

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"

	"github.com/xuri/excelize/v2"
)

const MaxRows = 2000

var ErrFormat = errors.New("formato não suportado")

type Reader struct{}

func (Reader) Rows(name string, data []byte) ([][]string, error) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".xlsx") && bytes.HasPrefix(data, []byte("PK")):
		f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 16 << 20})
		if err != nil {
			return nil, ErrFormat
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, ErrFormat
		}
		rows, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, ErrFormat
		}
		return limit(rows), nil
	case strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".txt"):
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		sep := ','
		if first, _, _ := bytes.Cut(data, []byte("\n")); bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
			sep = ';'
		}
		r := csv.NewReader(bytes.NewReader(data))
		r.Comma, r.FieldsPerRecord, r.LazyQuotes = sep, -1, true
		rows, err := r.ReadAll()
		if err != nil {
			return nil, ErrFormat
		}
		return limit(rows), nil
	}
	return nil, ErrFormat
}

func limit(rows [][]string) [][]string {
	if len(rows) > MaxRows+1 {
		rows = rows[:MaxRows+1]
	}
	return rows
}
