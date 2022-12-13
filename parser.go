package css

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/scanner"
)

type tokenEntry struct {
	value string
	pos   scanner.Position
}

type tokenizer struct {
	s *scanner.Scanner
}

type tokenType int
type Rule string

const (
	tokenFirstToken tokenType = iota - 1
	tokenBlockStart
	tokenBlockEnd
	tokenRuleName
	tokenValue
	tokenSelector
	tokenStyleSeparator
	tokenStatementEnd
)

func (rule Rule) Type() string {
	if strings.HasPrefix(string(rule), ".") {
		return "class"
	} else if strings.HasPrefix(string(rule), "#") {
		return "id"
	} else {
		return "tag"
	}
}

func (e tokenEntry) typ() tokenType {
	return newTokenType(e.value)
}

func newTokenType(typ string) tokenType {
	switch typ {
	case "{":
		return tokenBlockStart
	case "}":
		return tokenBlockEnd
	case ":":
		return tokenStyleSeparator
	case ";":
		return tokenStatementEnd
	case ".", "#":
		return tokenSelector
	}

	return tokenValue
}

func newTokenizer(r io.Reader) *tokenizer {
	s := &scanner.Scanner{}
	s.Init(r)
	return &tokenizer{
		s,
	}
}

func (t tokenType) String() string {
	switch t {
	case tokenBlockStart:
		return "BLOCK_START"
	case tokenBlockEnd:
		return "BLOCK_END"
	case tokenRuleName:
		return "RULE_NAME"
	case tokenSelector:
		return "SELECTOR"
	case tokenStyleSeparator:
		return "STYLE_SEPARATOR"
	case tokenStatementEnd:
		return "STATEMENT_END"
	}
	return "VALUE"
}

func (t *tokenizer) next() (tokenEntry, error) {
	token := t.s.Scan()
	if token == scanner.EOF {
		return tokenEntry{}, errors.New("EOF")
	}
	value := t.s.TokenText()
	pos := t.s.Pos()
	if newTokenType(value).String() == "STYLE_SEPARATOR" {
		t.s.IsIdentRune = func(ch rune, i int) bool {
			if ch == -1 || ch == '\n' || ch == '\t' || ch == ':' || ch == ';' {
				return false
			}
			return true
		}
	} else {
		t.s.IsIdentRune = func(ch rune, i int) bool {
			if ch == -1 || ch == '#' || ch == '.' || ch == '\n' || ch == '\t' || ch == ' ' || ch == ':' || ch == ';' {
				return false
			}
			return true
		}
	}

	return tokenEntry{
		value,
		pos,
	}, nil
}

func parse(l *list.List) (map[Rule]map[string]string, error) {
	var (
		rule      []string
		style     string
		value     string
		selector  string
		isBlock   bool
		css       = make(map[Rule]map[string]string)
		styles    = make(map[string]string)
		prevToken = tokenType(tokenFirstToken)
	)
	for e := l.Front(); e != nil; e = l.Front() {
		token := e.Value.(tokenEntry)
		l.Remove(e)
		switch token.typ() {
		case tokenValue:
			switch prevToken {
			case tokenFirstToken:
				rule = append(rule, token.value)
			case tokenSelector:
				rule = append(rule, selector+token.value)
			case tokenBlockStart:
				style = token.value
			case tokenStyleSeparator:
				value = token.value
			case tokenValue:
				rule = append(rule, token.value)
			default:
				return css, fmt.Errorf("line %d: unexpected token %s", token.pos.Line, token.value)
			}
		case tokenSelector:
			selector = token.value
		case tokenBlockStart:
			if prevToken != tokenValue {
				return css, fmt.Errorf("line %d: unexpected token %s", token.pos.Line, token.value)
			}
			isBlock = true
		case tokenStatementEnd:
			if prevToken != tokenValue || style == "" || value == "" {
				return css, fmt.Errorf("line %d: unexpected token %s", token.pos.Line, token.value)
			}
			styles[style] = value
		case tokenBlockEnd:
			if !isBlock {
				return css, fmt.Errorf("line %d: unexpected token %s", token.pos.Line, token.value)
			}

			for i := range rule {
				r := Rule(rule[i])
				oldRule, ok := css[r]
				if ok {
					for style, value := range oldRule {
						if _, ok := styles[style]; !ok {
							styles[style] = value
						}
					}
					continue
				}
				css[r] = styles
			}

			styles = map[string]string{}
			style, value = "", ""
			isBlock = false
		}

		prevToken = token.typ()
	}

	return css, nil
}

func buildList(r io.Reader) *list.List {
	l := list.New()
	t := newTokenizer(r)
	for {
		token, err := t.next()
		if err != nil {
			break
		}
		l.PushBack(token)
	}
	return l
}

func Unmarshal(b []byte) (map[Rule]map[string]string, error) {
	return parse(buildList(bytes.NewReader(b)))
}
