package sheet

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var addrRE *regexp.Regexp

// CellAddress is the address of a cell in a sheet.
type CellAddress struct {
	col string
	row uint32
}

const maxColDigits = 2
const lastCol = "ZZ"

// CellAddr creates a new CellAddress by parsing an address string, addr. addr must be of the
// format [A-Za-z]+[0-9]+, where the alphabetic characters are the column and the number is the
// row, as in a traditional spreadsheet. Currently, an most 2 alphabetic characters are specified
// for a maximum of 26^2 (676) columns. The number of rows is bounded to math.MaxUint32.
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

// LEQCol returns true if ca's column is less or equal to ca2's column.
func (ca CellAddress) LEQCol(ca2 CellAddress) bool {
	if ca.col == ca2.col {
		return true
	}
	return ca.LessCol(ca2)
}

// LessCol returns true if ca's column is strictly less than ca2's column.
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

// NextCol returns the next column after ca's column. It returns error if ca has the last column
// possible.
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

// String returns a human-readable representation of ca. This value can also be parsed by CellAddr.
func (ca CellAddress) String() string {
	return fmt.Sprintf("%s%d", ca.col, ca.row)
}

// These types describe what kind of value is in a cell.
// transient is when a cell is blank, but still exists - this is useful when we want to keep a cell
// in the sheet even though it is blank - for instance when other Cells have equations that
// reference the cell.
// val is when there is a numeric value in the cell that can be used for calculations.
// string is when there is a string value in the cell.
// expr is when there is an expression in the cell that will yield a value.
const (
	cell_transient = iota
	cell_val
	cell_string
	cell_expr
)

// Cell is the basic unit of storage and computation for a spreadsheet. A Cell has an address and
// can store numbers, text, or compute its value based on an equation, consuming values from other
// Cells.
type Cell struct {
	cell_type int
	addr      CellAddress
	sheet     *Sheet
	content   string
	val       float64

	// These variables hold the expression string, the parsed expression, and any error that
	// occurs during the parsing or computation of an expression, for instance when there are cyclical
	// dependencies (A1 = B1 + C1, C1 = A1)
	expstr string
	exp    *Expression
	expErr error

	// upstream is a list of cells that are used to calculate the result of this cell.
	// downstream is a list of cells that use the value of this cell to calculate their results.
	// Together, these lists form a graph of cells whose values depend on each other. This graph is
	// used to perform recalculations necessary when some value in the sheet changes.
	upstream   []*Cell
	downstream []*Cell

	// Recalculating is used during graph traversal to detect cycles.
	recalculating bool
	// ErrCycle is set when a cyclical computation error is detected.
	errCycle bool
}

// Create a new cell at CellAddress a in Sheet s
func NewCell(a CellAddress, s *Sheet) *Cell {
	return &Cell{addr: a, sheet: s}
}

// Add cell c2 to c's downstream dependents.
func (c *Cell) addDownstream(c2 *Cell) {
	c.downstream = append(c.downstream, c2)
}

// deleteSelfIfNecessary prunes a Cell from its Sheet if it is blank and no
// other cells depend on it.
func (c *Cell) deleteSelfIfNecessary() {
	if c.cell_type != cell_transient {
		return
	}
	if len(c.downstream) == 0 {
		delete(c.sheet.matrix[c.addr.col], c.addr.row)
	}
}

// Remove c2 from c's downstream dependents.
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

// EditValue returns the value to show when "editing" the cell. This means it will return the
// string, value or equation that is in the cell and not return the evaluated result of an
// equation.
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

// Value returns the numeric value present in the Cell, including the value resulting from the
// evaluation of an equation, or an error if there is no numeric value available.
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

// Content returns a string representation of the value of the cell. This will be a string
// representation of a number if the cell is numeric or has as equation that returns a result. It
// will be an error message if an equation results in an error, or it will be a string if text was
// entered into the cell.
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

// Recalculate recalculates the value of this cell and any downstream cells that would be affected
// by this cell's value. It will detect any dependency cycles present and set error messages on the
// affected cells.
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

// SetContent puts some value into the Cell, c. SetContent detects whether an equation, number, or
// text was entered and recalculates the sheet accordingly.
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
