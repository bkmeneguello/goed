package main

import (
	"bufio"
	"go/scanner"
	"go/token"
	"log"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// Editor holds all state for the text editor.
type Editor struct {
	lines            [][]rune              // Text buffer: each line is a slice of runes
	highlightCache   []map[int]tcell.Style // Cached syntax highlighting for each line
	cursorX, cursorY int                   // Cursor position in the buffer
	offsetX, offsetY int                   // Viewport offset for scrolling
	currentFilename  string                // Name of the currently loaded file
	inCommandMode    bool                  // True if in command mode (like Vim)
	screen           tcell.Screen          // tcell screen for terminal UI
	style            tcell.Style           // Default style for text
	dirty            bool                  // True if the buffer or viewport has changed
}

// NewEditor initializes a new Editor instance.
func NewEditor(screen tcell.Screen, style tcell.Style) *Editor {
	return &Editor{
		lines:          [][]rune{{}}, // Start with one empty line
		highlightCache: []map[int]tcell.Style{},
		cursorX:        0,
		cursorY:        0,
		offsetX:        0,
		offsetY:        0,
		inCommandMode:  false, // Start in edit (insert) mode, not command mode
		screen:         screen,
		style:          style,
		dirty:          true, // Initial state is dirty to trigger a full draw
	}
}

// loadFile loads a file into the editor buffer (entire file in memory).
func (e *Editor) loadFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		e.showStatus("Error opening file: " + err.Error())
		return
	}
	defer file.Close()

	e.lines = nil // Clear current buffer
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		e.lines = append(e.lines, []rune(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		e.showStatus("Error reading file: " + err.Error())
	}
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.cursorX, e.cursorY = 0, 0 // Reset cursor
	e.currentFilename = filename
	e.dirty = true                                               // Mark as dirty to trigger a redraw
	e.highlightCache = make([]map[int]tcell.Style, len(e.lines)) // Reset cache
}

// saveFile saves the buffer to a file (entire file in memory).
func (e *Editor) saveFile(filename string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		e.showStatus("Error opening file: " + err.Error())
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range e.lines {
		_, err := writer.WriteString(string(line) + "\n")
		if err != nil {
			e.showStatus("Error writing to file: " + err.Error())
			return
		}
	}
	writer.Flush()
	e.currentFilename = filename
	e.showStatus("File saved: " + filename)
}

// showStatus displays a message in the status bar (bottom line).
func (e *Editor) showStatus(msg string) {
	w, h := e.screen.Size()
	for x := range w {
		e.screen.SetContent(x, h-1, ' ', nil, e.style)
	}
	for x, ch := range msg {
		if x < w {
			e.screen.SetContent(x, h-1, ch, nil, e.style)
		}
	}
	e.screen.HideCursor() // Ensure the cursor is hidden when showing the status
	e.screen.Show()
}

// getHighlightMap returns a map of rune positions to styles for a given Go source line
func getHighlightMap(src string, baseStyle tcell.Style) map[int]tcell.Style {
	fset := token.NewFileSet()
	var s scanner.Scanner
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, []byte(src), nil, scanner.ScanComments)

	highlight := map[int]tcell.Style{}

	styles := map[token.Token]tcell.Style{
		token.COMMENT: baseStyle.Foreground(tcell.ColorGray),
		token.IDENT:   baseStyle.Foreground(tcell.ColorOrange),
		token.INT:     baseStyle.Foreground(tcell.ColorIndianRed),
		token.FLOAT:   baseStyle.Foreground(tcell.ColorRed),
		token.IMAG:    baseStyle.Foreground(tcell.ColorOrangeRed),
		token.CHAR:    baseStyle.Foreground(tcell.ColorPurple),
		token.STRING:  baseStyle.Foreground(tcell.ColorGreen),
	}

	literalStyle := baseStyle.Foreground(tcell.ColorGreen)
	operatorStyle := baseStyle.Foreground(tcell.ColorBlue)
	keywordStyle := baseStyle.Foreground(tcell.ColorBlue)
	defaultStyle := baseStyle

	for {
		posn, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		start := file.Offset(posn)
		end := start
		if lit != "" {
			end += len(lit)
		} else {
			end += len(tok.String())
		}

		style := defaultStyle
		if s, ok := styles[tok]; ok {
			style = s
		} else if tok.IsLiteral() {
			style = literalStyle
		} else if tok.IsOperator() {
			style = operatorStyle
		} else if tok.IsKeyword() {
			style = keywordStyle
		}

		for i := start; i < end; {
			_, size := utf8.DecodeRuneInString(src[i:])
			if size <= 0 {
				break
			}
			highlight[i] = style
			i += size
		}
	}
	return highlight
}

// draw renders the buffer and cursor to the screen, with Go syntax highlighting using AST.
func (e *Editor) draw() {
	if !e.dirty {
		return // Skip drawing if nothing has changed
	}

	e.screen.Clear()
	w, h := e.screen.Size()

	// Draw visible lines
	for y := 0; y < h && y+e.offsetY < len(e.lines); y++ {
		// Reserve the last line for the command bar only if in command mode
		if e.inCommandMode && y == h-1 {
			break
		}

		lineIndex := y + e.offsetY
		line := e.lines[lineIndex]
		src := string(line)

		// Update highlight cache if necessary
		if e.highlightCache[lineIndex] == nil {
			e.highlightCache[lineIndex] = getHighlightMap(src, e.style)
		}
		highlight := e.highlightCache[lineIndex]

		x := 0
		for i := 0; i < len(line) && x < w; {
			r, size := utf8.DecodeRuneInString(src[i:])
			style, ok := highlight[i]
			if !ok {
				style = e.style
			}
			e.screen.SetContent(x, y, r, nil, style)
			i += size
			x++
		}
	}

	// Show cursor only in insert mode
	if e.inCommandMode {
		e.screen.ShowCursor(0, h-1)
	} else {
		e.screen.ShowCursor(e.cursorX-e.offsetX, e.cursorY-e.offsetY)
	}

	e.screen.Show()
	e.dirty = false // Reset dirty flag after drawing
}

// handleCommandMode processes key events in command mode.
func (e *Editor) handleCommandMode(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		// Switch to insert mode
		e.inCommandMode = false
		e.dirty = true // Mark as dirty to trigger a redraw
		e.draw()       // Redraw the screen to restore the cursor
	case tcell.KeyRune:
		if ev.Rune() == ':' {
			e.handleCommandInput()
			e.inCommandMode = false
		}
	}
	// Redraw only once after handling the event
	e.draw()
}

// handleCommandInput handles the ':' command line at the bottom.
func (e *Editor) handleCommandInput() {
	cmd := []rune{':'}
	drawCmd := func() {
		w, h := e.screen.Size()
		for x := range w {
			e.screen.SetContent(x, h-1, ' ', nil, e.style)
		}
		for x, ch := range cmd {
			if x < w {
				e.screen.SetContent(x, h-1, ch, nil, e.style)
			}
		}
		e.screen.ShowCursor(len(cmd), h-1)
		e.screen.Show()
	}
	drawCmd()
	for inCmd := true; inCmd; {
		ev := e.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc:
				// Exit command input, redraw main buffer
				cmd = []rune{}
				inCmd = false
				e.dirty = true
				e.inCommandMode = false
			case tcell.KeyEnter:
				// Execute command
				command := string(cmd)
				switch {
				case strings.HasPrefix(command, ":e "):
					filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
					if filename == "" {
						e.showStatus("No filename specified for :e command")
						break
					}
					e.currentFilename = filename
					fallthrough
				case command == ":e":
					// Reload current file
					if e.currentFilename != "" {
						e.loadFile(e.currentFilename)
					} else {
						e.showStatus("No filename specified for :e command")
					}
				case strings.HasPrefix(command, ":w "):
					// Save a copy
					filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
					if filename == "" {
						e.showStatus("No filename specified for :w command")
						break
					}
					e.saveFile(filename)
				case command == ":w":
					// Save current file
					if e.currentFilename != "" {
						e.saveFile(e.currentFilename)
					} else {
						e.showStatus("No filename specified for :w command")
					}
				case command == ":q":
					// Quit editor
					e.screen.Fini()
					os.Exit(0)
				default:
					e.showStatus("Unknown command: " + command)
				}
				cmd = []rune{}
				inCmd = false
				e.dirty = true
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				// Remove last character from command
				if len(cmd) > 1 {
					cmd = cmd[:len(cmd)-1]
				}
				drawCmd()
			case tcell.KeyRune:
				// Add character to command
				cmd = append(cmd, ev.Rune())
				drawCmd()
			}
		case *tcell.EventResize:
			e.screen.Sync()
			drawCmd()
		}
	}
	// Redraw main buffer after exiting command input
	e.draw()
}

// handleInsertMode processes key events in insert mode.
func (e *Editor) handleInsertMode(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		// Switch to command mode
		e.dirty = true
		e.inCommandMode = true
	case tcell.KeyRune:
		if ev.Rune() != 0 {
			// Insert character at cursor position
			if e.cursorY >= len(e.lines) {
				e.lines = append(e.lines, []rune{})
			}
			line := e.lines[e.cursorY]
			r := ev.Rune()
			if e.cursorX > len(line) {
				e.cursorX = len(line)
			}
			newLine := append(line[:e.cursorX], append([]rune{r}, line[e.cursorX:]...)...)
			e.lines[e.cursorY] = newLine
			e.cursorX++
			e.dirty = true                    // Mark as dirty
			e.highlightCache[e.cursorY] = nil // Invalidate cache for the modified line
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Remove character before cursor or merge lines
		if e.cursorY < len(e.lines) && e.cursorX > 0 {
			line := e.lines[e.cursorY]
			e.lines[e.cursorY] = slices.Delete(line, e.cursorX-1, e.cursorX)
			e.cursorX--
			e.dirty = true                    // Mark as dirty
			e.highlightCache[e.cursorY] = nil // Invalidate cache for the modified line
		} else if e.cursorY > 0 {
			// Merge with previous line
			prevLine := e.lines[e.cursorY-1]
			e.cursorX = len(prevLine) // Set cursor position to the end of the previous line
			e.lines[e.cursorY-1] = append(prevLine, e.lines[e.cursorY]...)
			e.lines = slices.Delete(e.lines, e.cursorY, e.cursorY+1)
			e.cursorY--
			e.dirty = true                                               // Mark as dirty
			e.highlightCache = make([]map[int]tcell.Style, len(e.lines)) // Reset cache
		}
	case tcell.KeyEnter:
		// Split line at cursor
		if e.cursorY < len(e.lines) {
			line := e.lines[e.cursorY]
			newLine := slices.Clone(line[e.cursorX:])
			e.lines[e.cursorY] = line[:e.cursorX]
			e.lines = append(e.lines[:e.cursorY+1], append([][]rune{newLine}, e.lines[e.cursorY+1:]...)...)
			e.cursorY++
			e.cursorX = 0
			e.dirty = true                                               // Mark as dirty
			e.highlightCache = make([]map[int]tcell.Style, len(e.lines)) // Reset cache
		}
	case tcell.KeyLeft:
		if e.cursorX > 0 {
			e.cursorX--
		} else if e.cursorY > 0 {
			e.cursorY--
			e.cursorX = len(e.lines[e.cursorY])
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyRight:
		if e.cursorY < len(e.lines) && e.cursorX < len(e.lines[e.cursorY]) {
			e.cursorX++
		} else if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			e.cursorX = 0
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyUp:
		if e.cursorY > 0 {
			e.cursorY--
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyDown:
		if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyPgUp:
		// Scroll up one page minus one row
		_, h := e.screen.Size()
		if e.offsetY > 0 {
			e.offsetY -= h - 1
			if e.offsetY < 0 {
				e.offsetY = 0
			}
			e.cursorY = e.offsetY
			e.dirty = true // Mark as dirty to redraw
		}
	case tcell.KeyPgDn:
		// Scroll down one page minus one row
		_, h := e.screen.Size()
		if e.offsetY < len(e.lines)-1 {
			e.offsetY += h - 1
			if e.offsetY > len(e.lines)-1 {
				e.offsetY = len(e.lines) - 1
			}
			e.cursorY = e.offsetY
			e.dirty = true // Mark as dirty to redraw
		}
	}
	// Redraw only once after handling the event
	e.draw()
}

// adjustOffsets ensures the cursor is always visible in the viewport.
func (e *Editor) adjustOffsets() {
	w, h := e.screen.Size()
	if e.cursorX < e.offsetX {
		e.offsetX = e.cursorX
		e.dirty = true // Mark as dirty
	} else if e.cursorX >= e.offsetX+w {
		e.offsetX = e.cursorX - w + 1
		e.dirty = true // Mark as dirty
	}
	if e.cursorY < e.offsetY {
		e.offsetY = e.cursorY
		e.dirty = true // Mark as dirty
	} else if e.cursorY >= e.offsetY+h-1 {
		e.offsetY = e.cursorY - h + 2 // -1 for status/cmd bar
		e.dirty = true                // Mark as dirty
	}
}

func main() {
	// Initialize tcell screen for terminal UI
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}
	defer screen.Fini()

	screen.Clear()
	// Set default style: white foreground, black background
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

	editor := NewEditor(screen, style)

	// If a filename is provided as an argument, load it; otherwise, start with an empty buffer
	if len(os.Args) > 1 {
		editor.loadFile(os.Args[1])
	}

	editor.draw()

	// Main event loop
	for {
		ev := editor.screen.PollEvent()

		// Adjust horizontal and vertical offsets if cursor is out of visible area
		editor.adjustOffsets()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			if editor.inCommandMode {
				editor.handleCommandMode(ev)
			} else {
				editor.handleInsertMode(ev)
			}
		case *tcell.EventResize:
			editor.screen.Sync()
			editor.draw()
		}
	}
}
