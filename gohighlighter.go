package main

import (
	"go/scanner"
	"go/token"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// GoHighlighter implements syntax highlighting for Go files using go/token.
type GoHighlighter struct {
	styles        map[token.Token]tcell.Style
	literalStyle  tcell.Style
	operatorStyle tcell.Style
	keywordStyle  tcell.Style
	defaultStyle  tcell.Style
}

// NewGoHighlighter initializes a new GoHighlighter with default styles.
func NewGoHighlighter(baseStyle tcell.Style) *GoHighlighter {
	return &GoHighlighter{
		styles: map[token.Token]tcell.Style{
			token.COMMENT: baseStyle.Foreground(tcell.ColorGray),
			token.IDENT:   baseStyle.Foreground(tcell.ColorOrange),
			token.INT:     baseStyle.Foreground(tcell.ColorIndianRed),
			token.FLOAT:   baseStyle.Foreground(tcell.ColorRed),
			token.IMAG:    baseStyle.Foreground(tcell.ColorOrangeRed),
			token.CHAR:    baseStyle.Foreground(tcell.ColorPurple),
			token.STRING:  baseStyle.Foreground(tcell.ColorGreen),
		},
		literalStyle:  baseStyle.Foreground(tcell.ColorGreen),
		operatorStyle: baseStyle.Foreground(tcell.ColorBlue),
		keywordStyle:  baseStyle.Foreground(tcell.ColorBlue),
		defaultStyle:  baseStyle,
	}
}

// GetHighlightMap returns a map of rune positions to styles for a given Go source line.
func (gh *GoHighlighter) GetHighlightMap(src []rune) map[int]tcell.Style {
	fset := token.NewFileSet()
	var s scanner.Scanner
	srcBytes := gh.runesToBytes(src) // Convert []rune to []byte efficiently
	file := fset.AddFile("", fset.Base(), len(srcBytes))
	s.Init(file, srcBytes, nil, scanner.ScanComments)

	highlight := map[int]tcell.Style{}

	for {
		posn, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		start := file.Offset(posn)
		end := start
		if lit == "" {
			end += len(tok.String())
		} else {
			end += len(lit)
		}

		// Determine style based on token type
		style := gh.defaultStyle
		if s, ok := gh.styles[tok]; ok {
			style = s
		} else if tok.IsLiteral() {
			style = gh.literalStyle
		} else if tok.IsOperator() {
			style = gh.operatorStyle
		} else if tok.IsKeyword() {
			style = gh.keywordStyle
		}

		for i := start; i < end; i++ {
			highlight[i] = style
		}
	}
	return highlight
}

func (gh *GoHighlighter) runesToBytes(src []rune) []byte {
	// Allocate enough space: max 4 bytes per rune
	buf := make([]byte, 0, len(src)*utf8.UTFMax)

	for _, r := range src {
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		buf = append(buf, tmp[:n]...)
	}

	return buf
}
