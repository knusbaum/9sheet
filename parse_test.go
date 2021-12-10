package sheet

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextTok(t *testing.T) {
	assert := assert.New(t)
	eqn := "A5+B3+C2/D3-E4+G6*D7"
	p := &parser{r: strings.NewReader(eqn)}
	ss := make([]token, 0)
	for {
		t, err := p.nextTok()
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			break
		}
		ss = append(ss, t)
	}
	expected := []token{
		token{op: ID, val: "A5"},
		token{op: ADD},
		token{op: ID, val: "B3"},
		token{op: ADD},
		token{op: ID, val: "C2"},
		token{op: DIV},
		token{op: ID, val: "D3"},
		token{op: SUB},
		token{op: ID, val: "E4"},
		token{op: ADD},
		token{op: ID, val: "G6"},
		token{op: MUL},
		token{op: ID, val: "D7"},
	}
	assert.Equal(expected, ss)
}

func TestParseExpression(t *testing.T) {
	for name, tt := range map[string]struct {
		parse  string
		expect *Expression
	}{
		"simple/add": {
			parse: "=A1+B1",
			expect: &Expression{op: ADD,
				left:  &Expression{op: ID, val: "A1"},
				right: &Expression{op: ID, val: "B1"},
			},
		},
		"simple/sub": {
			parse: "=ZZ123-BB456",
			expect: &Expression{op: SUB,
				left:  &Expression{op: ID, val: "ZZ123"},
				right: &Expression{op: ID, val: "BB456"},
			},
		},
		"simple/mul": {
			parse: "=C123*D456",
			expect: &Expression{op: MUL,
				left:  &Expression{op: ID, val: "C123"},
				right: &Expression{op: ID, val: "D456"},
			},
		},
		"simple/div": {
			parse: "=E8484/F33",
			expect: &Expression{op: DIV,
				left:  &Expression{op: ID, val: "E8484"},
				right: &Expression{op: ID, val: "F33"},
			},
		},
		"nested/addsub": {
			parse: "=A1+B2-C3+E4",
			expect: &Expression{op: ADD,
				left: &Expression{op: SUB,
					left: &Expression{op: ADD,
						left:  &Expression{op: ID, val: "A1"},
						right: &Expression{op: ID, val: "B2"},
					},
					right: &Expression{op: ID, val: "C3"},
				},
				right: &Expression{op: ID, val: "E4"},
			},
		},
		"nested/addsubmuldiv": {
			parse: "=A1+B2*C3-E4/F5*G6",
			expect: &Expression{op: SUB,
				left: &Expression{op: ADD,
					left: &Expression{op: ID, val: "A1"},
					right: &Expression{op: MUL,
						left:  &Expression{op: ID, val: "B2"},
						right: &Expression{op: ID, val: "C3"},
					},
				},
				right: &Expression{op: MUL,
					left: &Expression{op: DIV,
						left:  &Expression{op: ID, val: "E4"},
						right: &Expression{op: ID, val: "F5"},
					},
					right: &Expression{op: ID, val: "G6"},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			exp, err := ParseExpression(tt.parse)
			if !assert.NoError(err) || !assert.NotNil(exp) {
				return
			}

			assert.Equal(exp, tt.expect)
		})
	}
}
