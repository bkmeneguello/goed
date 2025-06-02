package main

import (
	"github.com/gdamore/tcell/v2"
)

// Highlighter defines the interface for syntax highlighters.
type Highlighter interface {
	GetHighlightMap(src []rune) map[int]tcell.Style
}

// SyntaxHighlighter manages different highlighters based on file extensions.
type SyntaxHighlighter struct {
	factories map[string]func() Highlighter
	current   Highlighter
}

// NewSyntaxHighlighter initializes a new SyntaxHighlighter with default styles.
func NewSyntaxHighlighter(baseStyle tcell.Style) *SyntaxHighlighter {
	return &SyntaxHighlighter{
		factories: map[string]func() Highlighter{
			".go": func() Highlighter { return NewGoHighlighter(baseStyle) },
		},
		current: nil,
	}
}

// SetFileExtension sets the current highlighter based on the file extension.
func (sh *SyntaxHighlighter) SetFileExtension(extension string) {
	if factory, ok := sh.factories[extension]; ok {
		// Create a new highlighter using the factory function
		sh.current = factory()
	} else {
		sh.current = nil
	}
}

// GetHighlightMap delegates to the current highlighter or returns an empty style map.
func (sh *SyntaxHighlighter) GetHighlightMap(src []rune) map[int]tcell.Style {
	if sh.current == nil {
		return map[int]tcell.Style{} // Return an empty map if no highlighter is set
	}
	return sh.current.GetHighlightMap(src)
}
