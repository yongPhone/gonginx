package parser

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/tufanbarisyildirim/gonginx/parser/token"
)

//lexer is the main tokenizer
type lexer struct {
	reader *bufio.Reader
	file   string
	line   int
	column int
	Latest token.Token
}

//lex initializes a lexer from string conetnt
func lex(content string) *lexer {
	return newLexer(bytes.NewBuffer([]byte(content)))
}

//newLexer initilizes a lexer from a reader
func newLexer(r io.Reader) *lexer {
	return &lexer{
		line:   1,
		reader: bufio.NewReader(r),
	}
}

//Scan gives you next token
func (s *lexer) scan() (token.Token, error) {
	var err error
	s.Latest, err = s.getNextToken()
	if err != nil {
		return token.Token{}, err
	}
	return s.Latest, nil
}

//All scans all token and returns them as a slice
func (s *lexer) all() (token.Tokens, error) {
	tokens := make([]token.Token, 0)
	for {
		v, err := s.scan()
		if err != nil {
			return nil, err
		}
		if v.Type == token.EOF || v.Type == -1 {
			break
		}
		tokens = append(tokens, v)
	}
	return tokens, nil
}

func (s *lexer) getNextToken() (token.Token, error) {
reToken:
	ch := s.peek()
	switch {
	case isSpace(ch):
		s.skipWhitespace()
		goto reToken
	case isEOF(ch):
		return s.NewToken(token.EOF).Lit(string(s.read())), nil
	case ch == ';':
		return s.NewToken(token.Semicolon).Lit(string(s.read())), nil
	case ch == '{':
		return s.NewToken(token.BlockStart).Lit(string(s.read())), nil
	case ch == '}':
		return s.NewToken(token.BlockEnd).Lit(string(s.read())), nil
	case ch == '#':
		return s.scanComment(), nil
	case ch == '$':
		return s.scanVariable(), nil
	case isQuote(ch):
		return s.scanQuotedString(ch)
	default:
		return s.scanKeyword(), nil
	}
}

//Peek returns nexr rune without consuming it
func (s *lexer) peek() rune {
	r, _, _ := s.reader.ReadRune()
	_ = s.reader.UnreadRune()
	return r
}

type runeCheck func(rune) bool

func (s *lexer) readUntil(until runeCheck) string {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.peek(); isEOF(ch) {
			break
		} else if until(ch) {
			break
		} else {
			buf.WriteRune(s.read())
		}
	}

	return buf.String()
}

//NewToken creates a new Token with its line and column
func (s *lexer) NewToken(tokenType token.Type) token.Token {
	return token.Token{
		Type:   tokenType,
		Line:   s.line,
		Column: s.column,
	}
}

func (s *lexer) readWhile(while runeCheck) string {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.peek(); while(ch) {
			buf.WriteRune(s.read())
		} else {
			break
		}
	}
	// unread the latest char we consume.
	return buf.String()
}

func (s *lexer) skipWhitespace() {
	s.readWhile(isSpace)
}

func (s *lexer) scanComment() token.Token {
	return s.NewToken(token.Comment).Lit(s.readUntil(isEndOfLine))
}

/**
\” – To escape “ within double quoted string.
\\ – To escape the backslash.
\n – To add line breaks between string.
\t – To add tab space.
\r – For carriage return.
*/
func (s *lexer) scanQuotedString(delimiter rune) (token.Token, error) {
	var buf bytes.Buffer
	tok := s.NewToken(token.QuotedString)
	buf.WriteRune(s.read()) //consume delimiter
	for {
		ch := s.read()

		if ch == rune(token.EOF) {
			return token.Token{}, errors.New("unexpected end of file while scanning a string, maybe an unclosed quote? ")
		}

		if ch == '\\' {
			if needsEscape(s.peek(), delimiter) {
				switch s.read() {
				case 'n':
					buf.WriteRune('\n')
				case 'r':
					buf.WriteRune('\r')
				case 't':
					buf.WriteRune('\t')
				case '\\':
					buf.WriteRune('\\')
				case delimiter:
					buf.WriteRune(delimiter)
				}
				continue
			}
		}
		buf.WriteRune(ch)
		if ch == delimiter {
			break
		}
	}

	return tok.Lit(buf.String()), nil
}

func (s *lexer) scanKeyword() token.Token {
	return s.NewToken(token.Keyword).Lit(s.readUntil(isKeywordTerminator))
}

func (s *lexer) scanVariable() token.Token {
	return s.NewToken(token.Variable).Lit(s.readUntil(isKeywordTerminator))
}

func (s *lexer) read() rune {
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		return rune(token.EOF)
	}

	if ch == '\n' {
		s.column = 1
		s.line++
	} else {
		s.column++
	}
	return ch
}

func isQuote(ch rune) bool {
	return ch == '"' || ch == '\'' || ch == '`'
}

func isKeywordTerminator(ch rune) bool {
	return isSpace(ch) || isEndOfLine(ch) || ch == '{' || ch == ';'
}

func needsEscape(ch, delimiter rune) bool {
	return ch == delimiter || ch == 'n' || ch == 't' || ch == '\\' || ch == 'r'
}

func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\t' || isEndOfLine(ch)
}

func isEOF(ch rune) bool {
	return ch == rune(token.EOF)
}

func isEndOfLine(ch rune) bool {
	return ch == '\r' || ch == '\n'
}
