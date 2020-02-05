package main

import (
	"fmt"
)

func (e *Expression) Eval(s *Sheet) (float64, error) {
	if e.op == ID {
		return s.ValueAt(e.val)
	}

	switch e.op {
	case ADD:
		if e.left == nil {
			return 0, fmt.Errorf("Bad Expression: %#v", e)
		}
		l, err := e.left.Eval(s)
		if err != nil {
			return 0, err
		}

		if e.right == nil {
			return 0, fmt.Errorf("Bad expression: %#v", e)
		}
		r, err := e.right.Eval(s)
		if err != nil {
			return 0, err
		}

		return l + r, nil
	case SUB:
		panic("SUB NOT IMPLEMENTED")
	case MUL:
		panic("MUL NOT IMPLEMENTED")
	case DIV:
		panic("DIV NOT IMPLEMENTED")
	default:
		panic("BAD OP VAL")
	}
	panic("NOT FINISHED IMPLEMENTING")
	return 0, nil
}
