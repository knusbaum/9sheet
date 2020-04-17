package sheet

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCellAddr(t *testing.T) {
	for name, tt := range map[string]struct {
		addr        string
		expectAddr  CellAddress
		expectError bool
	}{
		"good": {
			addr:       "A2",
			expectAddr: CellAddress{col: "A", row: 2},
		},
		"case": {
			addr:       "a2",
			expectAddr: CellAddress{col: "A", row: 2},
		},
		"long/1": {
			addr:       "zz2",
			expectAddr: CellAddress{col: "ZZ", row: 2},
		},
		"long/2": {
			addr:       "zz200000000",
			expectAddr: CellAddress{col: "ZZ", row: 200000000},
		},
		"toobig": {
			addr:        "AAA1",
			expectError: true,
		},
		"toobig/2": {
			addr:        "AA100000000000000",
			expectError: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			addr, err := CellAddr(tt.addr)

			if tt.expectError {
				assert.Error(err)
			} else {
				assert.NoError(err)
				assert.Equal(tt.expectAddr, addr)
			}
		})
	}
}

func TestEquationLoop(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "=A2")
	assert.NoError(err)
	err = sheet.SetContent("A2", "=A3")
	assert.NoError(err)
	err = sheet.SetContent("A3", "=A1")
	assert.NoError(err)

	v, err := sheet.ContentAt("A1")
	assert.NoError(err)
	assert.Equal("A1: Cyclical equations detected.", v)

	v, err = sheet.ContentAt("A2")
	assert.NoError(err)
	assert.Equal("A2: Cyclical equations detected.", v)

	v, err = sheet.ContentAt("A3")
	assert.NoError(err)
	assert.Equal("A3: Cyclical equations detected.", v)
}

func TestSetContent(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "=A2+A3")
	assert.NoError(err)
	v, err := sheet.ValueAt("A1")
	assert.NoError(err)
	assert.Equal(float64(0), v)

	err = sheet.SetContent("A2", "5")
	assert.NoError(err)

	err = sheet.SetContent("A3", "6")
	assert.NoError(err)

	v, err = sheet.ValueAt("A1")
	assert.NoError(err)
	assert.Equal(float64(11), v)
}

func TestSetContent2(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "=A2+A3+A4")
	assert.NoError(err)

	err = sheet.SetContent("A2", "=B2+B3")
	assert.NoError(err)

	err = sheet.SetContent("A3", "=B4+B5")
	assert.NoError(err)

	err = sheet.SetContent("A4", "=B6+B7")
	assert.NoError(err)

	err = sheet.SetContent("B2", "1")
	assert.NoError(err)
	err = sheet.SetContent("B3", "1")
	assert.NoError(err)
	err = sheet.SetContent("B4", "1")
	assert.NoError(err)
	err = sheet.SetContent("B5", "1")
	assert.NoError(err)
	err = sheet.SetContent("B6", "1")
	assert.NoError(err)
	err = sheet.SetContent("B7", "1")
	assert.NoError(err)

	v, err := sheet.ValueAt("A1")
	assert.NoError(err)
	assert.Equal(float64(6), v)

	err = sheet.SetContent("B2", "Some String")
	assert.NoError(err)
	err = sheet.SetContent("B3", "Some Other String")
	assert.NoError(err)
	err = sheet.SetContent("A2", "1")
	assert.NoError(err)

	v, err = sheet.ValueAt("A1")
	assert.NoError(err)
	assert.Equal(float64(5), v)

}

func TestSetContent3(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "=A2+A3")
	assert.NoError(err)
	err = sheet.SetContent("A2", "=A4+A5")
	assert.NoError(err)
	err = sheet.SetContent("A3", "1")
	assert.NoError(err)
	err = sheet.SetContent("A4", "2")
	assert.NoError(err)
	err = sheet.SetContent("A5", "3")
	assert.NoError(err)

	v, err := sheet.ValueAt("A1")
	assert.NoError(err)
	assert.Equal(float64(6), v)

	err = sheet.SetContent("A5", "Some String")
	assert.NoError(err)

	v, err = sheet.ValueAt("A1")
	assert.Error(err)
	assert.Equal(float64(0), v)
}

func TestCellAddress(t *testing.T) {
	t.Run("", func(t *testing.T) {
		assert := assert.New(t)
		ca, err := CellAddr("A1")
		assert.NoError(err)
		ca2, err := CellAddr("B1")
		assert.NoError(err)

		assert.True(ca.LessCol(ca2))
	})

	t.Run("", func(t *testing.T) {
		assert := assert.New(t)
		ca, err := CellAddr("B1")
		assert.NoError(err)
		ca2, err := CellAddr("AZ1")
		assert.NoError(err)

		assert.True(ca.LessCol(ca2))
	})
}

func TestCellAddressNext(t *testing.T) {
	assert := assert.New(t)
	ca, err := CellAddr("A1")
	assert.NoError(err)

	next, err := ca.NextCol()
	assert.NoError(err)

	assert.Equal(CellAddress{col: "B", row: 1}, next)

	ca, err = CellAddr("Z1")
	assert.NoError(err)

	next, err = ca.NextCol()
	assert.NoError(err)

	assert.Equal(CellAddress{col: "AA", row: 1}, next)
}

func generateAddrs() []string {
	letters := []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K",
		"L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V",
		"W", "X", "Y", "Z",
	}

	var strs []string
	for i := range letters {
		strs = append(strs, fmt.Sprintf("%s1", letters[i]))
	}
	for i := range letters {
		for j := range letters {
			strs = append(strs, fmt.Sprintf("%s%s1", letters[i], letters[j]))
		}
	}
	//	for i := range letters {
	//		for j := range letters {
	//			for k := range letters {
	//				strs = append(strs, fmt.Sprintf("%s%s%s1", letters[i], letters[j], letters[k]))
	//			}
	//		}
	//	}
	return strs
}

func TestCellAddressNext2(t *testing.T) {
	assert := assert.New(t)
	cellStr := generateAddrs()

	addrSeq := make([]CellAddress, len(cellStr))
	for i := range cellStr {
		var err error
		addrSeq[i], err = CellAddr(cellStr[i])
		if !assert.NoError(err) {
			return
		}
	}

	it, err := CellAddr("A1")
	if !assert.NoError(err) {
		return
	}
	last := CellAddress{col: lastCol, row: 1}
	for it.LessCol(last) {
		if !assert.Equal(addrSeq[0], it) {
			return
		}
		it, err = it.NextCol()
		if !assert.NoError(err) {
			return
		}
		addrSeq = addrSeq[1:]
	}
}

func TestMaxCol(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "1")
	assert.NoError(err)
	err = sheet.SetContent("B2", "1")
	assert.NoError(err)
	err = sheet.SetContent("C3", "1")
	assert.NoError(err)

	a := sheet.maxCol()
	assert.Equal(CellAddress{col: "C", row: 1}, a)

	//fmt.Println("BREAK")
	err = sheet.SetContent("FT1", "1")
	assert.NoError(err)

	a = sheet.maxCol()
	assert.Equal(CellAddress{col: "FT", row: 1}, a)
}

func TestMaxRow(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()

	err := sheet.SetContent("A1", "1")
	assert.NoError(err)
	err = sheet.SetContent("B23", "1")
	assert.NoError(err)
	err = sheet.SetContent("C10", "1")
	assert.NoError(err)

	row := sheet.maxRow()
	assert.Equal(uint32(23), row)

	err = sheet.SetContent("ZZ2991", "1")
	assert.NoError(err)

	row = sheet.maxRow()
	assert.Equal(uint32(2991), row)
}

//func TestCSV(t *testing.T) {
//	//assert := assert.New(t)
//	sheet := NewSheet()
//
//	sheet.SetContent("A1", "Count")
//	sheet.SetContent("A2", "1")
//	sheet.SetContent("A3", "2")
//	sheet.SetContent("A4", "3")
//	sheet.SetContent("A5", "4")
//	sheet.SetContent("A6", "5")
//	//sheet.SetContent("B8", "Some thing.")
//
//	sheet.SetContent("A7", "=A2+A3+A4+A5+A6")
//	sheet.WriteCSV(os.Stdout)
//}

func TestWriteRange(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()
	sheet.SetContent("A1", "Count")
	sheet.SetContent("B1", "1")
	sheet.SetContent("C1", "2")
	sheet.SetContent("D1", "3")
	sheet.SetContent("E1", "4")
	sheet.SetContent("F1", "5")
	sheet.SetContent("F2", "Total")
	sheet.SetContent("F3", "=B1+C1+D1+E1+F1")

	start, _ := CellAddr("A1")
	end, _ := CellAddr("F3")
	b := &strings.Builder{}
	sheet.WriteRange(start, end, b)
	out := b.String()
	expected := `A1 5 Count
B1 8 1.000000
C1 8 2.000000
D1 8 3.000000
E1 8 4.000000
F1 8 5.000000
F2 5 Total
F3 15 =B1+C1+D1+E1+F1
`
	assert.Equal(expected, out)
}

func TestRead(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()
	sheet.SetContent("A1", "Count")
	sheet.SetContent("B1", "1")
	sheet.SetContent("C1", "2")
	sheet.SetContent("D1", "3")
	sheet.SetContent("E1", "4")
	sheet.SetContent("F1", "5")
	sheet.SetContent("F2", "Total")
	sheet.SetContent("F3", "=B1+C1+D1+E1+F1")

	start, _ := CellAddr("A1")
	end, _ := CellAddr("F3")
	b := &strings.Builder{}
	sheet.WriteRange(start, end, b)
	s := b.String()

	sheet = NewSheet()
	sr := strings.NewReader(s)
	for {
		err := sheet.Read(sr)
		if err != nil {
			if !assert.Equal(io.EOF, err) {
				return
			}
			break
		}
	}
	
	v, err := sheet.ValueAt("F3")
	assert.NoError(err)
	assert.Equal(float64(15), v)
}

func TestTransient(t *testing.T) {
	assert := assert.New(t)
	sheet := NewSheet()
	sheet.SetContent("A1", "Count")
	sheet.SetContent("B1", "1")
	sheet.SetContent("C1", "2")
	sheet.SetContent("D1", "3")
	sheet.SetContent("E1", "4")
	sheet.SetContent("F1", "5")
	sheet.SetContent("F2", "Total")
	sheet.SetContent("F3", "=B1+C1+D1+E1")

	sheet.SetContent("F1", "")
	assert.Nil(sheet.cellAt(CellAddress{col: "F", row: 1}))

	sheet.SetContent("B1", "")
	if !assert.NotNil(sheet.cellAt(CellAddress{col: "B", row: 1})) {
		return
	}
	assert.Equal(cell_transient, sheet.cellAt(CellAddress{col: "B", row: 1}).cell_type)

	sheet.SetContent("F3", "")
	assert.Nil(sheet.cellAt(CellAddress{col: "F", row: 3}))
	assert.Nil(sheet.cellAt(CellAddress{col: "B", row: 1}))
}
