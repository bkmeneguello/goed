package main

import (
	"maps"

	"github.com/gdamore/tcell/v2"
)

// HighlightCache manages syntax highlighting cache for the editor.
type HighlightCache struct {
	cache       map[int]map[int]tcell.Style
	highlighter *SyntaxHighlighter
	retention   int
	lastOffset  int
}

// NewHighlightCache initializes a new HighlightCache.
func NewHighlightCache(highlighter *SyntaxHighlighter, retention int) *HighlightCache {
	return &HighlightCache{
		cache:       make(map[int]map[int]tcell.Style),
		highlighter: highlighter,
		retention:   retention,
		lastOffset:  -1,
	}
}

// Clear clears the highlight cache.
func (hc *HighlightCache) Clear() {
	hc.cache = make(map[int]map[int]tcell.Style)
}

// Update updates the highlight cache based on the current viewport.
func (hc *HighlightCache) Update(offsetY, height int, lines [][]rune) {
	start := offsetY - (hc.retention * height)
	end := offsetY + (hc.retention * height)

	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}

	batch := make(map[int]map[int]tcell.Style)
	for i := start; i < end; i++ {
		if _, exists := hc.cache[i]; !exists {
			batch[i] = hc.highlighter.GetHighlightMap(string(lines[i]))
		}
	}

	// Merge batch updates into the cache
	maps.Copy(hc.cache, batch)
}

// Exists checks if a line index exists in the cache.
func (hc *HighlightCache) Exists(lineIndex int) bool {
	_, exists := hc.cache[lineIndex]
	return exists
}

// UpdateLine updates the highlight for a specific line index.
func (hc *HighlightCache) UpdateLine(lineIndex int, highlight map[int]tcell.Style) {
	hc.cache[lineIndex] = highlight
}

// Get retrieves the highlight for a specific line index.
func (hc *HighlightCache) Get(lineIndex int) map[int]tcell.Style {
	return hc.cache[lineIndex]
}

// ClearLine clears the highlight for a specific line index.
func (hc *HighlightCache) ClearLine(lineIndex int) {
	delete(hc.cache, lineIndex)
}
