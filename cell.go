package main

import (
	"fmt"
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
	addr       CellAddress
	sheet      *Sheet
	content    string
	val        float64
	expstr     string
	exp        *Expression
	expErr     error
	upstream   []*Cell
	downstream []*Cell
	recalculating bool
	errCycle bool
}

func NewCell(a CellAddress, s *Sheet) *Cell {
	return &Cell{addr: a, sheet: s}
}

func (c *Cell) addDownstream(c2 *Cell) {
	c.downstream = append(c.downstream, c2)
}

func (c *Cell) deleteSelfIfNecessary() {
	if c.cell_type != cell_transient {
		return
	}
	if len(c.downstream) == 0 {
		delete(c.sheet.mtx[c.addr.col], c.addr.row)
	}
}

func (c *Cell) removeDownstream(c2 *Cell) {
	defer c.deleteSelfIfNecessary()
	for i := range c.downstream {
		if c.downstream[i] == c2 {
			c.downstream[i] = c.downstream[len(c.downstream)-1]
			c.downstream[len(c.downstream)-1] = nil
			c.downstream = c.downstream[:len(c.downstream)-1]
			return
		}
	}
}

func (c *Cell) EditValue() (string, error) {
	switch c.cell_type {
	case cell_transient:
		return "", nil
	case cell_val:
		return fmt.Sprintf("%f", c.val), nil
	case cell_string:
		return c.content, nil
	case cell_expr:
		return c.expstr, nil
	default:
		return "", fmt.Errorf("Invalid cell type.")
	}
}

func (c *Cell) Value() (float64, error) {
	switch c.cell_type {
	case cell_transient:
		return 0, nil
	case cell_string:
		return 0, fmt.Errorf("Cannot get numeric value from %s", c.addr)
	case cell_val:
		return c.val, nil
	case cell_expr:
		if c.expErr != nil {
			return 0, fmt.Errorf("%s: %v", c.addr, c.expErr)
		}
		return c.val, nil
	default:
		return 0, fmt.Errorf("Invalid cell type.")
	}
}

func (c *Cell) Content() (string, error) {
	switch c.cell_type {
	case cell_transient:
		return "", nil
	case cell_string:
		return c.content, nil
	case cell_val:
		return fmt.Sprintf("%f", c.val), nil
	case cell_expr:
		if c.expErr != nil {
			return fmt.Sprintf("%s: %v", c.addr, c.expErr), nil
		}
		return fmt.Sprintf("%f", c.val), nil
	default:
		//panic(fmt.Sprintf("Invalid cell type %d", c.cell_type))
		return "##ERROR", fmt.Errorf("Invalid cell type.")
	}
}

func (c *Cell) Recalculate() {
	if c.recalculating {
		// We've hit a cycle. 
		if c.cell_type == cell_expr {
			//fmt.Printf("CYCLICAL EQUATIONS\n")
			c.expErr = fmt.Errorf("Cyclical equations detected.")
		}
		c.content = "##ERROR"
		if !c.errCycle {
			// If this is the first round through the cycle, continue and populate errors.
			c.errCycle = true
			defer func() { c.errCycle = false }()
			for i := range c.upstream {
				c.upstream[i].Recalculate()
			}
		}
		return
	}
	c.recalculating = true
	defer func() { c.recalculating = false }()
	if c.sheet.OnCellUpdated != nil {
		defer c.sheet.OnCellUpdated(c.addr.String(), c)
	}
	defer func() {
		for i := range c.downstream {
			c.downstream[i].Recalculate()
		}
	}()
	if c.cell_type != cell_expr || c.exp == nil {
		return
	}

	//fmt.Printf("RECALCULATING CELL @ %s -> ", c.addr)
	f, err := c.exp.Eval(c.sheet)
	if err != nil {
		//fmt.Println("ERROR")
		c.expErr = err
		c.content = "##ERROR"
		return
	}
	c.expErr = nil
	c.val = f
	c.content = fmt.Sprintf("%f", f)
	//fmt.Printf("%s\n", c.content)
}

func (c *Cell) SetContent(content string) error {
	defer c.deleteSelfIfNecessary()
	defer c.Recalculate()
	if len(c.upstream) > 0 {
		for i := range c.upstream {
			c.upstream[i].removeDownstream(c)
		}
		c.upstream = nil
	}
	if content == "" {
		*c = Cell{sheet: c.sheet, addr: c.addr, cell_type: cell_transient, downstream: c.downstream}
		return nil
	}
	if strings.HasPrefix(content, "=") {
		c.cell_type = cell_expr
		c.expstr = content
		c.exp = nil
		expr, err := ParseExpression(content)
		if err != nil {
			c.expErr = err
			c.content = "##ERROR"
			return nil
		}
		upAddrs, err := expr.upstreamAddrs()
		if err != nil {
			c.expErr = err
			c.content = "##ERROR"
			return nil
		}

		c.upstream = make([]*Cell, len(upAddrs))
		for i := range upAddrs {
			c.upstream[i] = c.sheet.cellOrNewAt(upAddrs[i])
			c.upstream[i].addDownstream(c)
		}

		c.exp = expr
	} else if f, err := strconv.ParseFloat(content, 64); err == nil {
		c.cell_type = cell_val
		c.val = f
		c.content = fmt.Sprintf("%f", f)
	} else {
		c.cell_type = cell_string
		c.content = content
	}
	return nil
}