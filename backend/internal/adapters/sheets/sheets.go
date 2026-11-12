package sheets

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"riha/internal/core/ports"
)

const (
	MaxRows = 2000
	// MaxCols is the widest sheet read: an import has a handful of columns.
	MaxCols = 64
)

var ErrFormat = errors.New("formato não suportado")

// ErrTooWide is a sheet with data past column MaxCols: reading it would take
// the memory of thousands of empty cells per row.
var ErrTooWide = ports.ErrSheetTooWide

type Reader struct{}

func (Reader) Rows(name string, data []byte) ([][]string, error) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".xlsx") && bytes.HasPrefix(data, []byte("PK")):
		if wide, err := tooWide(data); err != nil {
			return nil, ErrFormat
		} else if wide {
			return nil, ErrTooWide
		}
		f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 16 << 20})
		if err != nil {
			return nil, ErrFormat
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, ErrFormat
		}
		// Row by row, never the whole sheet at once: a file with one cell
		// far to the right on each row would otherwise be held as thousands
		// of empty cells per row.
		it, err := f.Rows(sheets[0])
		if err != nil {
			return nil, ErrFormat
		}
		defer it.Close()
		rows := [][]string{}
		for len(rows) <= MaxRows && it.Next() {
			cols, err := it.Columns()
			if err != nil {
				return nil, ErrFormat
			}
			if len(cols) > MaxCols {
				cols = cols[:MaxCols:MaxCols]
			}
			rows = append(rows, cols)
		}
		if it.Error() != nil {
			return nil, ErrFormat
		}
		return rows, nil
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
	for i := range rows {
		if len(rows[i]) > MaxCols {
			rows[i] = rows[i][:MaxCols:MaxCols]
		}
	}
	return rows
}

// tooWide tells, reading the sheets' XML as it streams, whether a cell past
// column MaxCols holds a value or a formula. Empty, styled cells far to the
// right, which spreadsheet programs write, don't count.
func tooWide(data []byte) (bool, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false, err
	}
	var read int64
	for _, zf := range zr.File {
		if !strings.HasPrefix(zf.Name, "xl/worksheets/") || !strings.HasSuffix(zf.Name, ".xml") {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return false, err
		}
		lr := &io.LimitedReader{R: rc, N: 32<<20 - read}
		wide, err := wideIn(xml.NewDecoder(lr))
		read = 32<<20 - lr.N
		_ = rc.Close()
		if err != nil || wide {
			return wide, err
		}
	}
	return false, nil
}

func wideIn(d *xml.Decoder) (bool, error) {
	col, cellCol, inCell := 0, 0, false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "row":
				col = 0
			case "c":
				col++
				for _, at := range e.Attr {
					if at.Name.Local == "r" {
						if c, _, err := excelize.CellNameToCoordinates(at.Value); err == nil {
							col = c
						}
					}
				}
				cellCol, inCell = col, true
			case "v", "is", "f":
				if inCell && cellCol > MaxCols {
					return true, nil
				}
			}
		case xml.EndElement:
			if e.Name.Local == "c" {
				inCell = false
			}
		}
	}
}
