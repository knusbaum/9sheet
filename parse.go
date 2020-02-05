package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type Op int

const (
	NONE Op = iota
	ADD  Op = iota
	SUB  Op = iota
	MUL  Op = iota
	DIV  Op = iota
	LP   Op = iota
	RP   Op = iota
	ID   Op = iota
)

type Token struct {
	op  Op
	val string
}

type Expression struct {
	op    Op
	left  *Expression
	right *Expression
	val   string
}

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

type Parser struct {
	r    *strings.Reader
	look Token
}

func (p *Parser) unreadToken(tok Token) error {
	if p.look.op != NONE {
		return fmt.Errorf("Cannot unread more than one token.")
	}
	p.look = tok
	return nil
}

func (p *Parser) nextTok() (tok Token, err error) {
	if p.look.op != NONE {
		tok = p.look
		p.look.op = NONE
		p.look.val = ""
		return
	}
	rn, _, err := p.r.ReadRune()
	if err != nil {
		return Token{}, err
	}

	switch rn {
	case rune('-'):
		return Token{op: SUB}, nil
	case rune('+'):
		return Token{op: ADD}, nil
	case rune('*'):
		return Token{op: MUL}, nil
	case rune('/'):
		return Token{op: DIV}, nil
	case rune('('):
		return Token{op: LP}, nil
	case rune(')'):
		return Token{op: RP}, nil
	}

	if !(unicode.IsLetter(rn) || unicode.IsDigit(rn)) {
		ret := string([]rune{rn})
		return Token{}, fmt.Errorf("Unexpected rune %s", ret)
	}

	var rs []rune
	for err == nil && (unicode.IsLetter(rn) || unicode.IsDigit(rn)) {
		rs = append(rs, rn)
		rn, _, err = p.r.ReadRune()
	}
	if err != io.EOF {
		p.r.UnreadRune()
	}

	return Token{op: ID, val: string(rs)}, nil
}

// EXP = MDSEXP PMSEXP
// PMSEXP = ADD MDSEXP PMSEXP | SUB MDSEXP PMSEXP | END
// MDSEXP = SUBEXP MDEXP
// MDEXP = MUL SUBEXP MDEXP | DIV SUBEXP MDEXP | END
// SUBEXP = LP EXP RP | ID

// ID = '[a-zA-Z0-9]+'
// ADD = '+'
// SUB = '-'
// MUL = '*'
// DIV = '/'
// LP = '('
// RP = ')'
// OP = [+-*/]

func (p *Parser) expectTok(t Token) error {
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
func (p *Parser) parseSUBEXP() (*Expression, error) {
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
		err = p.expectTok(Token{op: RP})
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
func (p *Parser) parseMDEXP(left *Expression) (*Expression, error) {
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
func (p *Parser) parseMDSEXP() (*Expression, error) {
	exp, err := p.parseSUBEXP()
	if err != nil {
		return nil, err
	}
	return p.parseMDEXP(exp)
}

// PMSEXP = ADD MDSEXP PMSEXP | SUB MDSEXP PMSEXP | END
func (p *Parser) parsePMSEXP(left *Expression) (*Expression, error) {
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

// EXP = MDSEXP PMSEXP
func (p *Parser) parseEXP() (*Expression, error) {
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

func ParseExpression(eqn string) (*Expression, error) {
	r := strings.NewReader(eqn)
	expectStr(r, "=")
	prs := Parser{r: r}
	p := &prs
	e, err := p.parseEXP()
	if err != nil {
		return nil, err
	}
	return e, nil
}
