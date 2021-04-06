package parser

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser/token"
)

//Parser is an nginx config parser
type Parser struct {
	lexer             *lexer
	currentToken      token.Token
	followingToken    token.Token
	statementParsers  map[string]func() gonginx.IDirective
	blockWrappers     map[string]func(*gonginx.Directive) (gonginx.IDirective, error)
	directiveWrappers map[string]func(*gonginx.Directive) (gonginx.IDirective, error)
}

//NewStringParser parses nginx conf from string
func NewStringParser(str string) (*Parser, error) {
	return NewParserFromLexer(lex(str))
}

//NewParser create new parser
func NewParser(filePath string) (*Parser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	l := newLexer(bufio.NewReader(f))
	l.file = filePath
	p, err := NewParserFromLexer(l)
	if err != nil {
		return nil, err
	}
	return p, nil
}

//NewParserFromLexer initilizes a new Parser
func NewParserFromLexer(lexer *lexer) (*Parser, error) {
	parser := &Parser{
		lexer: lexer,
	}
	err := parser.nextToken()
	if err != nil {
		return nil, err
	}
	err = parser.nextToken()
	if err != nil {
		return nil, err
	}

	parser.blockWrappers = map[string]func(*gonginx.Directive) (gonginx.IDirective, error) {
		"http": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.wrapHttp(directive), nil
		},
		"server": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.wrapServer(directive), nil
		},
		"location": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.wrapLocation(directive)
		},
		"upstream": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.wrapUpstream(directive), nil
		},
	}

	parser.directiveWrappers = map[string]func(*gonginx.Directive) (gonginx.IDirective, error) {
		"server": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.parseUpstreamServer(directive), nil
		},
		"include": func(directive *gonginx.Directive) (gonginx.IDirective, error) {
			return parser.parseInclude(directive)
		},
	}

	return parser, nil
}

func (p *Parser) nextToken() error {
	p.currentToken = p.followingToken
	var err error
	p.followingToken, err = p.lexer.scan()
	if err != nil {
		return err
	}
	return err
}

func (p *Parser) curTokenIs(t token.Type) bool {
	return p.currentToken.Type == t
}

func (p *Parser) followingTokenIs(t token.Type) bool {
	return p.followingToken.Type == t
}

//Parse the gonginx.
func (p *Parser) Parse() (*gonginx.Config, error) {
	block, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &gonginx.Config{
		FilePath: p.lexer.file, //TODO: set filepath here,
		Block:    block,
	}, nil
}

//ParseBlock parse a block statement
func (p *Parser) parseBlock() (*gonginx.Block, error) {

	context := &gonginx.Block{
		Directives: make([]gonginx.IDirective, 0),
	}

parsingloop:
	for {
		switch {
		case p.curTokenIs(token.EOF) || p.curTokenIs(token.BlockEnd):
			break parsingloop
		case p.curTokenIs(token.Keyword):
			di, err :=  p.parseStatement()
			if err != nil {
				return nil, err
			}
			context.Directives = append(context.Directives, di)
			break
		}
		err := p.nextToken()
		if err != nil {
			return nil, err
		}
	}

	return context, nil
}

func (p *Parser) parseStatement() (gonginx.IDirective, error) {
	d := &gonginx.Directive{
		Name: p.currentToken.Literal,
	}

	//if we have a special parser for the directive, we use it.
	if sp, ok := p.statementParsers[d.Name]; ok {
		return sp(), nil
	}

	//parse parameters until the end.
	if err := p.nextToken(); err != nil {
		return nil, err
	}
	for p.currentToken.IsParameterEligible() {
		d.Parameters = append(d.Parameters, p.currentToken.Literal)
		if err := p.nextToken(); err != nil {
			return nil, err
		}
	}

	//if we find a semicolon it is a directive, we will check directive converters
	if p.curTokenIs(token.Semicolon) {
		if dw, ok := p.directiveWrappers[d.Name]; ok {
			return dw(d)
		}
		return d, nil
	}

	//ok, it does not end with a semicolon but a block starts, we will convert that block if we have a converter
	if p.curTokenIs(token.BlockStart) {
		var err error
		d.Block, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
		if bw, ok := p.blockWrappers[d.Name]; ok {
			return bw(d)
		}
		return d, nil
	}

	return nil, fmt.Errorf("unexpected token %s (%s) on line %d, column %d", p.currentToken.Type.String(), p.currentToken.Literal, p.currentToken.Line, p.currentToken.Column)
}

//TODO: move this into gonginx.Include
func (p *Parser) parseInclude(directive *gonginx.Directive) (*gonginx.Include, error) {
	include := &gonginx.Include{
		Directive:   directive,
		IncludePath: directive.Parameters[0],
	}

	if len(directive.Parameters) > 1 {
		return nil, errors.New("include directive can not have multiple parameters")
	}

	if directive.Block != nil {
		return nil, errors.New("include can not have a block, or missing semicolon at the end of include statement")
	}

	return include, nil
}

//TODO: move this into gonginx.Location
func (p *Parser) wrapLocation(directive *gonginx.Directive) (*gonginx.Location, error) {
	location := &gonginx.Location{
		Modifier:  "",
		Match:     "",
		Directive: directive,
	}

	if len(directive.Parameters) == 0 {
		return nil, errors.New("no enough parameter for location")
	}

	if len(directive.Parameters) == 1 {
		location.Match = directive.Parameters[0]
		return location, nil
	} else if len(directive.Parameters) == 2 {
		location.Modifier = directive.Parameters[0]
		location.Match = directive.Parameters[1]
		return location, nil
	}

	return nil, errors.New("too many arguments for location directive")
}

func (p *Parser) wrapServer(directive *gonginx.Directive) *gonginx.Server {
	s, _ := gonginx.NewServer(directive)
	return s
}

func (p *Parser) wrapUpstream(directive *gonginx.Directive) *gonginx.Upstream {
	s, _ := gonginx.NewUpstream(directive)
	return s
}

func (p *Parser) wrapHttp(directive *gonginx.Directive) *gonginx.Http {
	h, _ := gonginx.NewHttp(directive)
	return h
}

func (p *Parser) parseUpstreamServer(directive *gonginx.Directive) *gonginx.UpstreamServer {
	return gonginx.NewUpstreamServer(directive)
}
