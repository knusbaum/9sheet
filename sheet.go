package main

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var addrRE *regexp.Regexp

type CellAddress struct {
	col string
	row uint32
}

const maxColDigits = 2
const lastCol = "ZZ"

func CellAddr(addr string) (CellAddress, error) {
	var err error
	if addrRE == nil {
		addrRE, err = regexp.Compile("([A-Za-z]+)([0-9]+)")
		if err != nil {
			return CellAddress{}, err
		}
	}
	matches := addrRE.FindStringSubmatch(addr)
	//fmt.Printf("MATCHES: %#v\n", matches)
	if len(matches) != 3 {
		return CellAddress{}, fmt.Errorf("Invalid cell address '%s'", addr)
	}
	colstr := strings.ToUpper(matches[1])
	rowstr := matches[2]

	// Artificially limit number of columns to 26^2 (2 letters)
	if len(colstr) > maxColDigits {
		return CellAddress{}, fmt.Errorf("Invalid cell address '%s': Column address too big", addr)
	}

	row, err := strconv.ParseUint(rowstr, 10, 32)
	if err != nil {
		return CellAddress{}, fmt.Errorf("Invalid cell address '%s': %v", addr, err)
	}

	return CellAddress{colstr, uint32(row)}, nil
}

func (ca CellAddress) LEQCol(ca2 CellAddress) bool {
	if ca.col == ca2.col {
		return true
	}
	return ca.LessCol(ca2)
}

func (ca CellAddress) LessCol(ca2 CellAddress) bool {
	if len(ca.col) < len(ca2.col) {
		return true
	} else if len(ca.col) > len(ca2.col) {
		return false
	}

	for i := range ca.col {
		if ca.col[i] < ca2.col[i] {
			return true
		} else if ca.col[i] > ca2.col[i] {
			return false
		}
	}
	// The column address is the same.
	//return ca.row < ca2.row
	return false
}

func (ca CellAddress) NextCol() (CellAddress, error) {
	if ca.col == lastCol {
		return CellAddress{}, fmt.Errorf("No more columns.")
	}
	ret := ca
	runes := []rune(ret.col)
	activeCol := len(runes) - 1
	for activeCol >= 0 {
		if runes[activeCol] == 'Z' {
			runes[activeCol] = 'A'
			if activeCol == 0 {
				runes = append([]rune{'A'}, runes...)
				break
			} else {
				activeCol -= 1
			}
		} else {
			//fmt.Printf("runes[%d] += 1: %s\n", activeCol, string(runes))
			runes[activeCol] += 1
			break
		}
	}
	ret.col = string(runes)
	return ret, nil
}

func (ca CellAddress) String() string {
	return fmt.Sprintf("%s%d", ca.col, ca.row)
}

const (
	cell_transient = iota
	cell_val
	cell_string
	cell_expr
)

type Cell struct {
	cell_type  int
	content    string
	val        float64
	expstr     string
	exp        *Expression
	expErr     error
	upstream   []*Cell
	downstream []*Cell
}

func (c *Cell) addDownstream(c2 *Cell) {
	c.downstream = append(c.downstream, c2)
}

func (c *Cell) removeDownstream(c2 *Cell) {
	for i := range c.downstream {
		if c.downstream[i] == c2 {
			c.downstream[i] = c.downstream[len(c.downstream)-1]
			c.downstream[len(c.downstream)-1] = nil
			c.downstream = c.downstream[:len(c.downstream)-1]
			return
		}
	}
}

func (c *Cell) editValue() string {
	switch c.cell_type {
	case cell_transient:
		return ""
	case cell_val:
		return fmt.Sprintf("%f", c.val)
	case cell_string:
		return c.content
	case cell_expr:
		return c.expstr
	default:
		return ""
	}
}

type Sheet struct {
	mtx map[string]map[uint32]*Cell
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
	cell := s.cellOrNewAt(a)
	//fmt.Printf("Setting content for %s@%p: %s.\n", addr, cell, content)

	if len(cell.upstream) > 0 {
		for i := range cell.upstream {
			cell.upstream[i].removeDownstream(cell)
		}
		cell.upstream = nil
	}

	if strings.HasPrefix(content, "=") {
		cell.cell_type = cell_expr
		cell.expstr = content
		cell.exp = nil
		expr, err := ParseExpression(content)
		if err != nil {
			cell.expErr = err
			cell.content = "##ERROR"
			return nil
		}
		upAddrs, err := expr.upstreamAddrs()
		if err != nil {
			cell.expErr = err
			cell.content = "##ERROR"
			return nil
		}

		cell.upstream = make([]*Cell, len(upAddrs))
		for i := range upAddrs {
			cell.upstream[i] = s.cellOrNewAt(upAddrs[i])
			cell.upstream[i].addDownstream(cell)
		}

		cell.exp = expr
	} else if f, err := strconv.ParseFloat(content, 64); err == nil {
		cell.cell_type = cell_val
		cell.val = f
		cell.content = fmt.Sprintf("%f", f)
	} else {
		cell.cell_type = cell_string
		cell.content = content
	}
	s.recalculateCell(cell)
	return nil
}

func (s *Sheet) recalculateCell(cell *Cell) {
	defer func() {
		for i := range cell.downstream {
			s.recalculateCell(cell.downstream[i])
		}
	}()
	if cell.cell_type != cell_expr || cell.exp == nil {
		return
	}

	//fmt.Printf("RECALCULATING CELL @ %p -> ", cell)
	f, err := cell.exp.Eval(s)
	if err != nil {
		//fmt.Println("ERROR")
		cell.expErr = err
		cell.content = "##ERROR"
		return
	}
	cell.expErr = nil
	cell.val = f
	//fmt.Printf("%f\n", f)
	cell.content = fmt.Sprintf("%f", f)
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
		cell = new(Cell)
		rows[addr.row] = cell
		//s.setCellAt(addr, cell)
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

	switch cell.cell_type {
	case cell_transient:
		return 0, nil
	case cell_string:
		return 0, fmt.Errorf("Cannot get numeric value from %s", addr)
	case cell_val:
		return cell.val, nil
	case cell_expr:
		if cell.expErr != nil {
			return 0, fmt.Errorf("%s: %v", addr, cell.expErr)
		}
		return cell.val, nil
	default:
		return 0, fmt.Errorf("Bad Cell")
	}
}

func (s *Sheet) ContentAt(addr string) (string, error) {
	a, err := CellAddr(addr)
	if err != nil {
		return "", err
	}
	return s.contentAt(a), nil
}

func (s *Sheet) contentAt(addr CellAddress) string {
	cell := s.cellAt(addr)
	if cell == nil {
		return ""
	}

	switch cell.cell_type {
	case cell_transient:
		return ""
	case cell_string:
		return cell.content
	case cell_val:
		return fmt.Sprintf("%f", cell.val)
	case cell_expr:
		if cell.expErr != nil {
			return fmt.Sprintf("%s%d: %v", addr.col, addr.row, cell.expErr)
		}
		return fmt.Sprintf("%f", cell.val)
	default:
		panic(fmt.Sprintf("Invalid cell type %d", cell.cell_type))
	}

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
			c := s.contentAt(col)
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

func (s *Sheet) WriteRange(start CellAddress, end CellAddress, w io.Writer) {
	for row := start.row; row <= end.row; row++ {
		col := CellAddress{col: start.col, row: row}
		for ; col.LEQCol(end); col, _ = col.NextCol() {
			//fmt.Printf("COL: %s, end: %s, leq: %v\n", col, end, col.LEQCol(end))

			cell := s.cellAt(col)
			if cell == nil {
				continue
			}

			command := cell.editValue()
			w.Write([]byte(fmt.Sprintf("%s %d %s\n", col, len(command), command)))
		}
	}
}

func Read(r io.Reader) (CellAddress, string, error) {
	var addr string
	var clen uint32
	n, err := fmt.Fscanf(r, "%50s%d ", &addr, &clen)
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
	//fmt.Printf("Setting cell at [%s] to [%s]\n", addr, string(bs))
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
	return s.SetContent(a.String(), c)
}

//
//	err = s.SetContent(addr, string(bs))
//	if err != nil {
//		return err
//	}
//	return nil
//}
