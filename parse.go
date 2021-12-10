package sheet

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type op int

const (
	NONE op = iota
	ADD  op = iota
	SUB  op = iota
	MUL  op = iota
	DIV  op = iota
	LP   op = iota
	RP   op = iota
	ID   op = iota
)

type token struct {
	op  op
	val string
}

// Expression is an equation that can be evaluated on a Sheet.
type Expression struct {
	op    op
	left  *Expression
	right *Expression
	val   string
}

// upstreamAddrs returns a list of CellAddresses that are used in this equation.
func (e *Expression) upstreamAddrs() ([]CellAddress, error) {
	if e.op == ID {
		addr, err := CellAddr(e.val)
		if err != nil {
			return nil, err
		}
		return []CellAddress{addr}, nil
	}

	addrs := make([]CellAddress, 0)
	if e.left != nil {
		leftas, err := e.left.upstreamAddrs()
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, leftas...)
	}

	if e.right != nil {
		rightas, err := e.right.upstreamAddrs()
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, rightas...)
	}
	return addrs, nil
}

// parser parses an Equation. See: ParseExpression
type parser struct {
	r    *strings.Reader
	look token
}

// unreadToken returns a token onto the front of the parser's token stream.
// Only one token can be unread at a time, or an error will occur.
func (p *parser) unreadToken(tok token) error {
	if p.look.op != NONE {
		return fmt.Errorf("Cannot unread more than one token.")
	}
	p.look = tok
	return nil
}

// nextTok returns the next token from the token stream, or an error if there is an invalid token.
// err is io.EOF when the end of the stream is reached.
func (p *parser) nextTok() (tok token, err error) {
	if p.look.op != NONE {
		tok = p.look
		p.look.op = NONE
		p.look.val = ""
		return
	}
	rn, _, err := p.r.ReadRune()
	if err != nil {
		return token{}, err
	}

	switch rn {
	case rune('-'):
		return token{op: SUB}, nil
	case rune('+'):
		return token{op: ADD}, nil
	case rune('*'):
		return token{op: MUL}, nil
	case rune('/'):
		return token{op: DIV}, nil
	case rune('('):
		return token{op: LP}, nil
	case rune(')'):
		return token{op: RP}, nil
	}

	if !(unicode.IsLetter(rn) || unicode.IsDigit(rn)) {
		ret := string([]rune{rn})
		return token{}, fmt.Errorf("Unexpected rune %s", ret)
	}

	var rs []rune
	for err == nil && (unicode.IsLetter(rn) || unicode.IsDigit(rn)) {
		rs = append(rs, rn)
		rn, _, err = p.r.ReadRune()
	}
	if err != io.EOF {
		p.r.UnreadRune()
	}

	return token{op: ID, val: string(rs)}, nil
}

// expectTok is used to consume an expected token, t, from the stream. if the next token is not ==
// t or there are no more tokens, expectTok returns an error.
func (p *parser) expectTok(t token) error {
	tok, err := p.nextTok()
	if err != nil {
		return err
	}
	if tok != t {
		return fmt.Errorf("Expected %#v, but have %#v", t, tok)
	}
	return nil
}

// SUBEXP = LP EXP RP | ID
func (p *parser) parseSUBEXP() (*Expression, error) {
	tok, err := p.nextTok()
	if err != nil {
		return nil, err
	}

	switch tok.op {
	case LP:
		exp, err := p.parseEXP()
		if err != nil {
			return nil, err
		}
		err = p.expectTok(token{op: RP})
		if err != nil {
			return nil, err
		}
		return exp, nil
	case ID:
		return &Expression{op: ID, val: tok.val}, nil
	}
	return nil, fmt.Errorf("Expected a SUBEXPR, but got token %#v", tok)
}

// MDEXP = MUL SUBEXP MDEXP | DIV SUBEXP MDEXP | END
func (p *parser) parseMDEXP(left *Expression) (*Expression, error) {
	tok, err := p.nextTok()
	if err == io.EOF {
		// We are at the end of the epression.
		return left, nil
	}
	switch tok.op {
	case MUL:
		fallthrough
	case DIV:
		ex, err := p.parseSUBEXP()
		if err != nil {
			return nil, err
		}
		exp := &Expression{op: tok.op, left: left, right: ex}
		return p.parseMDEXP(exp)
	}
	// not EOF and not MUL or DIV, so not part of this production.
	// We want to unread the token to not lose it.
	err = p.unreadToken(tok)
	if err != nil {
		return nil, err
	}
	return left, nil
}

// MDSEXP = SUBEXP MDEXP
func (p *parser) parseMDSEXP() (*Expression, error) {
	exp, err := p.parseSUBEXP()
	if err != nil {
		return nil, err
	}
	return p.parseMDEXP(exp)
}

// PMSEXP = ADD MDSEXP PMSEXP | SUB MDSEXP PMSEXP | END
func (p *parser) parsePMSEXP(left *Expression) (*Expression, error) {
	tok, err := p.nextTok()
	if err == io.EOF {
		// We are at the end of the epression.
		return left, nil
	}
	switch tok.op {
	case ADD:
		fallthrough
	case SUB:
		ex, err := p.parseMDSEXP()
		if err != nil {
			return nil, err
		}
		exp := &Expression{op: tok.op, left: left, right: ex}
		return p.parsePMSEXP(exp)
	}
	// not EOF and not ADD or SUB, so not part of this production.
	// We want to unread the token to not lose it.
	err = p.unreadToken(tok)
	if err != nil {
		return nil, err
	}
	return left, nil
}

//  EXP = MDSEXP PMSEXP
func (p *parser) parseEXP() (*Expression, error) {
	exp, err := p.parseMDSEXP()
	if err != nil {
		return nil, err
	}
	return p.parsePMSEXP(exp)
}

func expectStr(r *strings.Reader, s string) error {
	bs := make([]byte, len(s))
	_, err := r.Read(bs)
	if err != nil {
		return err
	}
	if s != string(bs) {
		return fmt.Errorf("Expected %s, got %s\n", s, bs)
	}
	return nil
}

// ParseExpression parses an EXP according to the below grammar. ParseExpression is implemented as
// a hand-written recursive descent parse.
//
//  EXP = MDSEXP PMSEXP
//  PMSEXP = ADD MDSEXP PMSEXP | SUB MDSEXP PMSEXP | END
//  MDSEXP = SUBEXP MDEXP
//  MDEXP = MUL SUBEXP MDEXP | DIV SUBEXP MDEXP | END
//  SUBEXP = LP EXP RP | ID

//  ID = '[a-zA-Z0-9]+'
//  ADD = '+'
//  SUB = '-'
//  MUL = '*'
//  DIV = '/'
//  LP = '('
//  RP = ')'
//  OP = [+-*/]
func ParseExpression(eqn string) (*Expression, error) {
	r := strings.NewReader(eqn)
	expectStr(r, "=")
	prs := parser{r: r}
	p := &prs
	e, err := p.parseEXP()
	if err != nil {
		return nil, err
	}
	return e, nil
}
