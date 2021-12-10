package sheet

import (
	"fmt"
	"io"
)

// Sheet represents a spreadsheet.
type Sheet struct {
	matrix map[string]map[uint32]*Cell
	// OnCellUpdated is a callback that will be called when a cell is updated during
	// recalculations. It is *NOT* called when 	explicitly setting the content of a cell.
	// OnCellUpdated may be set by the user.
	OnCellUpdated func(addr string, c *Cell)
}

// NewSheet creates a new, empty spreadsheet.
func NewSheet() *Sheet {
	return &Sheet{matrix: make(map[string]map[uint32]*Cell)}
}

// SetContent sets the content of the cell at address addr in the sheet.
// If the address is invalid, SetContent returns an error.
func (s *Sheet) SetContent(addr string, content string) error {
	//fmt.Printf("Setting %s -> %s\n", addr, content)
	a, err := CellAddr(addr)
	if err != nil {
		//fmt.Printf("ERROR!!! %#v\n", err)
		return err
	}

	if content == "" {
		cell := s.cellAt(a)
		if cell == nil {
			return nil
		}
	}

	cell := s.cellOrNewAt(a)
	return cell.SetContent(content)
}

// setCellAt puts a Cell into s at address addr.
func (s *Sheet) setCellAt(addr CellAddress, c *Cell) {
	rows := s.matrix[addr.col]
	if rows == nil {
		rows = make(map[uint32]*Cell)
		s.matrix[addr.col] = rows
	}
	rows[addr.row] = c
}

// cellOrNewAt returns the cell at addr, or puts a new cell into s at addr and returns that new
// cell.
func (s *Sheet) cellOrNewAt(addr CellAddress) *Cell {
	rows := s.matrix[addr.col]
	if rows == nil {
		//fmt.Printf("Making new row at %s\n", addr.col)
		rows = make(map[uint32]*Cell)
		s.matrix[addr.col] = rows
	}

	cell, ok := rows[addr.row]
	if !ok {
		cell = NewCell(addr, s)
		rows[addr.row] = cell
	}
	return cell
}

// cellAt returns a cell from addr in s if there is one, or nil if there is none.
func (s *Sheet) cellAt(addr CellAddress) *Cell {
	rows := s.matrix[addr.col]
	if rows != nil {
		return rows[addr.row]
	}
	return nil
}

// ValueAt returns the numeric value present at address addr in s. If the addr is invalid or there
// is not a numeric value available at addr, ValueAt returns an error. Empty cells have an implicit
// numeric value of 0.
func (s *Sheet) ValueAt(addr string) (float64, error) {
	a, err := CellAddr(addr)
	if err != nil {
		return 0, err
	}

	cell := s.cellAt(a)
	if cell == nil {
		// Empty cells have zero value
		return 0, nil
	}
	return cell.Value()
}

// ContentAt will return a human-readable value for a given address, suitable for display. This will
// display the result of any equation.
func (s *Sheet) ContentAt(addr string) (string, error) {
	a, err := CellAddr(addr)
	if err != nil {
		return "", err
	}
	return s.contentAt(a)
}

func (s *Sheet) contentAt(addr CellAddress) (string, error) {
	cell := s.cellAt(addr)
	if cell == nil {
		return "", nil
	}
	return cell.Content()
}

// EditAt returns a human-readable representation of the value of the cell at address addr,
// suitable for editing. This means cells containing equations will return the equation text rather
// that the result of evaluating the equation.
func (s *Sheet) EditAt(addr string) (string, error) {
	a, err := CellAddr(addr)
	if err != nil {
		return "", err
	}

	cell := s.cellAt(a)
	if cell == nil {
		// Empty cells have zero value
		return "", nil
	}
	return cell.EditValue()
}

func (s *Sheet) maxCol() CellAddress {
	max := CellAddress{col: "A", row: 1}
	for k := range s.matrix {
		//fmt.Printf("KEY: %s\n", k)
		addr := CellAddress{col: k, row: 1}
		//fmt.Printf("Comparing %v to %v\n", max, addr)
		if max.LessCol(addr) {
			max = addr
		}
	}
	return max
}

func (s *Sheet) maxRow() uint32 {
	max := uint32(1)
	for _, col := range s.matrix {
		for k := range col {
			if k > max {
				max = k
			}
		}
	}
	return max
}

// WriteCSV writes out a CSV containing the contents of the sheet. This uses the ContentAt function
// to write human-readable values of the cells, including the results of the evaluated equations.
func (s *Sheet) WriteCSV(w io.Writer) {
	for row := uint32(1); row <= s.maxRow(); row++ {
		mc := s.maxCol()
		var err error
		col := CellAddress{col: "A", row: row}
		for {
			c, _ := s.contentAt(col) // We ignore errors
			fmt.Fprintf(w, "%s,", c)
			col, err = col.NextCol()
			if err != nil {
				panic(err)
			}
			if !col.LessCol(mc) {
				break
			}
		}
		fmt.Fprintln(w, "")
	}
}

// WriteRange writes instructions to recreate the cells between the upper left start and bottom right end cells to w.
// The stream written is human-readable and suitable for reading with (*Sheet).Read
func (s *Sheet) WriteRange(start CellAddress, end CellAddress, w io.Writer) error {
	for row := start.row; row <= end.row; row++ {
		col := CellAddress{col: start.col, row: row}
		for ; col.LEQCol(end); col, _ = col.NextCol() {
			//fmt.Printf("COL: %s, end: %s, leq: %v\n", col, end, col.LEQCol(end))

			cell := s.cellAt(col)
			if cell == nil {
				continue
			}

			command, err := cell.EditValue()
			if err != nil {
				return err
			}
			w.Write([]byte(fmt.Sprintf("%s %d %s\n", col, len(command), command)))
		}
	}
	return nil
}

// read parses an instruction (such as those written out by WriteRange) and returns the cell
// address, value, and an error if it could not be parsed.
func read(r io.Reader) (CellAddress, string, error) {
	var addr string
	var clen uint32
	n, err := fmt.Fscanf(r, "%50s %d ", &addr, &clen)
	if err != nil {
		return CellAddress{}, "", err
	}
	if n != 2 {
		return CellAddress{}, "", fmt.Errorf("Expected address and length.")
	}
	if clen > 4096 {
		return CellAddress{}, "", fmt.Errorf("Bad length for content. Must be less than 4096.")
	}
	bs := make([]byte, clen)
	n, err = io.ReadFull(r, bs)
	if err != nil {
		return CellAddress{}, "", err
	}
	_, err = fmt.Fscanf(r, "\n")
	if err != nil {
		return CellAddress{}, "", err
	}
	a, err := CellAddr(addr)
	if err != nil {
		return CellAddress{}, "", err
	}
	return a, string(bs), nil
}

// Read reads a stream of instructions (such as those written by WriteRange) and sets the values in the sheet.
// Instructions are in the form:
//  [address] value\n
// For example, you can set various fields in the sheet by doing the following:
//  A1 10
//  B1 20
//  C1 30
//  D1 =A1+B1+C1
func (s *Sheet) Read(r io.Reader) error {
	a, c, err := read(r)
	if err != nil {
		return err
	}
	//fmt.Printf("SETTING CONTENT: %s\n", a.String())
	return s.SetContent(a.String(), c)
}
