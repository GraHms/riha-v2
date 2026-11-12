package sheets

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// One value at column XFD per row would make the reader build 16 384 cells per
// row: such a sheet is refused, at once, before it is read.
func TestSecXLSXWideRowsMemory(t *testing.T) {
	// Streamed, so making the file doesn't itself hold the padded rows.
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2500; i++ {
		if err := sw.SetRow(fmt.Sprintf("XFD%d", i), []any{1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := (Reader{}).Rows("a.xlsx", buf.Bytes()); !errors.Is(err, ErrTooWide) {
		t.Fatalf("got %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %s to refuse", d)
	}
}

// Empty, styled cells far to the right, as spreadsheet programs write them,
// don't stop an import; at most MaxCols columns are kept.
func TestWideButEmptySheetReads(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Nome")
	_ = f.SetCellValue("Sheet1", "BL1", "Última")
	_ = f.SetCellValue("Sheet1", "A2", "Kelvin")
	st, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	_ = f.SetCellStyle("Sheet1", "XFD1", "XFD2", st)
	buf, _ := f.WriteToBuffer()
	rows, err := (Reader{}).Rows("a.xlsx", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "Nome" || len(rows[0]) != MaxCols || rows[0][MaxCols-1] != "Última" {
		t.Fatalf("rows %d, first row %d cells", len(rows), len(rows[0]))
	}
}
