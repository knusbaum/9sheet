package main

import (
	"fmt"
	"io"
)

type Sheet struct {
	mtx           map[string]map[uint32]*Cell
	OnCellUpdated func(addr string, c *Cell)
}

func NewSheet() *Sheet {
	return &Sheet{mtx: make(map[string]map[uint32]*Cell)}
}

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

func (s *Sheet) setCellAt(addr CellAddress, c *Cell) {
	rows := s.mtx[addr.col]
	if rows == nil {
		rows = make(map[uint32]*Cell)
		s.mtx[addr.col] = rows
	}
	rows[addr.row] = c
}

func (s *Sheet) cellOrNewAt(addr CellAddress) *Cell {
	rows := s.mtx[addr.col]
	if rows == nil {
		//fmt.Printf("Making new row at %s\n", addr.col)
		rows = make(map[uint32]*Cell)
		s.mtx[addr.col] = rows
	}

	cell, ok := rows[addr.row]
	if !ok {
		cell = NewCell(addr, s)
		rows[addr.row] = cell
	}
	return cell
}

func (s *Sheet) cellAt(addr CellAddress) *Cell {
	rows := s.mtx[addr.col]
	if rows != nil {
		return rows[addr.row]
	}
	return nil
}

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
	for k := range s.mtx {
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
	for _, col := range s.mtx {
		for k := range col {
			if k > max {
				max = k
			}
		}
	}
	return max
}

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

func Read(r io.Reader) (CellAddress, string, error) {
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

func (s *Sheet) Read(r io.Reader) error {
	a, c, err := Read(r)
	if err != nil {
		return err
	}
	//fmt.Printf("SETTING CONTENT: %s\n", a.String())
	return s.SetContent(a.String(), c)
}

//
//	err = s.SetContent(addr, string(bs))
//	if err != nil {
//		return err
//	}
//	return nil
//}
