package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

const (
	defaultShowLineNumbers      = true
	defaultHighlightCurrentLine = true
)

// Editor holds all state for the text editor.
type Editor struct {
	lines            [][]rune // Text buffer: each line is a slice of runes
	cursorX, cursorY int      // Cursor position in the buffer
	offsetX, offsetY int      // Viewport offset for scrolling
	currentFilename  string   // Name of the currently loaded file
	inCommandMode    bool     // True if in command mode (like Vim)
	screen           tcell.Screen
	style            tcell.Style
	dirty            bool // True if the buffer or viewport has changed

	highlighter *SyntaxHighlighter

	cmd []rune // Command line input buffer

	status string // Status message to display

	showLineNumbers bool // True if line numbers should be displayed

	highlightCurrentLine bool // True if the current line should be highlighted

	spacesPerTab int // Number of spaces to render for a tab character

	// Add fields for actual and virtual cursor positions
	virtualCursorX int // Virtual cursor position considering tabs
}

// NewEditor initializes a new Editor instance.
// It sets up the text buffer, syntax highlighter, and default settings.
func NewEditor(screen tcell.Screen, style tcell.Style) *Editor {
	highlighter := NewSyntaxHighlighter(style)
	return &Editor{
		lines:                [][]rune{{}}, // Start with one empty line
		cursorX:              0,
		cursorY:              0,
		offsetX:              0,
		offsetY:              0,
		inCommandMode:        false, // Start in edit (insert) mode, not command mode
		screen:               screen,
		style:                style,
		dirty:                true, // Initial state is dirty to trigger a full draw
		highlighter:          highlighter,
		cmd:                  []rune{}, // Initialize command buffer
		showLineNumbers:      defaultShowLineNumbers,
		highlightCurrentLine: defaultHighlightCurrentLine,
		spacesPerTab:         4, // Default to 4 spaces per tab
	}
}

// loadFile loads a file into the editor buffer (entire file in memory).
// It clears the current buffer, reads the file line by line, and updates the syntax highlighter.
func (e *Editor) loadFile(filename string) error {
	filename = filepath.Clean(filename)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", filename, err)
	}
	defer file.Close()

	e.lines = nil // Clear current buffer
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		e.lines = append(e.lines, []rune(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file '%s': %w", filename, err)
	}
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.cursorX, e.cursorY = 0, 0 // Reset cursor
	e.currentFilename = filename
	e.dirty = true // Mark as dirty to trigger a redraw

	// Update highlighter
	e.highlighter.SetFileExtension(filepath.Ext(filename))

	return nil
}

// saveFile saves the buffer to a file (entire file in memory).
// It writes each line of the buffer to the specified file.
func (e *Editor) saveFile(filename string) {
	filename = filepath.Clean(filename)

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

// showStatus updates the status message displayed in the editor.
// It marks the editor as dirty to trigger a redraw.
func (e *Editor) showStatus(msg string) {
	e.status = msg
	e.dirty = true // Mark as dirty to trigger a redraw
}

// draw renders the buffer and cursor to the screen, with Go syntax highlighting using AST.
// It handles line numbers, current line highlighting, and the status/command bar.
func (e *Editor) draw() {
	if !e.dirty {
		return // Skip drawing if nothing has changed
	}

	e.screen.Clear()
	w, h := e.screen.Size()

	// Calculate gutter width once
	gutterWidth := 0
	if e.showLineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(e.lines)))
	}

	// Draw visible lines
	for y := 0; y < h && y+e.offsetY < len(e.lines); y++ {
		// Reserve the last line for the status or command bar only if needed
		if (e.inCommandMode || e.status != "") && y == h-1 {
			break
		}

		lineIndex := y + e.offsetY
		line := e.lines[lineIndex]
		src := string(line)
		highlightMap := e.highlighter.GetHighlightMap(src)

		if e.showLineNumbers {
			// Draw line number gutter
			lineNumber := fmt.Sprintf("%*d ", gutterWidth, lineIndex+1)
			for x, r := range lineNumber {
				if e.highlightCurrentLine && lineIndex == e.cursorY {
					e.screen.SetContent(x, y, r, nil, e.style.Background(tcell.Color18))
				} else {
					e.screen.SetContent(x, y, r, nil, e.style)
				}
			}
		}

		// Adjust starting position for content rendering
		startX := 0
		if e.showLineNumbers {
			startX = gutterWidth + 1
		}

		// Draw line content
		for x, i := startX, e.offsetX; x < w; {
			r := ' ' // Default to space if no character is available
			size := 1
			if i < len(line) {
				r, size = utf8.DecodeRuneInString(src[i:])
				if r == '\t' {
					// Render tab as spaces but treat as one character for layout
					spaces := strings.Repeat(" ", e.spacesPerTab)
					style := highlightMap[i]
					if e.highlightCurrentLine && lineIndex == e.cursorY {
						style = style.Background(tcell.Color18)
					}
					for _, spaceRune := range spaces {
						if x < w {
							e.screen.SetContent(x, y, spaceRune, nil, style)
							x++
						}
					}
					i++ // Move to the next character in the line
					continue
				}
			}
			style := highlightMap[i]
			if e.highlightCurrentLine && lineIndex == e.cursorY {
				style = style.Background(tcell.Color18)
			}
			e.screen.SetContent(x, y, r, nil, style)
			x++
			i += size
		}
	}

	// Draw status or command line
	if e.inCommandMode {
		e.drawCmd(e.cmd)
	} else if e.status != "" {
		e.drawStatus()
	}

	if !e.inCommandMode {
		cursorX := e.virtualCursorX - e.offsetX
		if e.showLineNumbers {
			cursorX += gutterWidth + 1
		}
		e.screen.ShowCursor(cursorX, e.cursorY-e.offsetY)
	}

	e.screen.Show()
	e.dirty = false // Reset dirty flag after drawing
}

func (e *Editor) drawStatusBar(content string) {
	w, h := e.screen.Size()
	for x := range w {
		e.screen.SetContent(x, h-1, ' ', nil, e.style)
	}
	for x, ch := range content {
		if x < w {
			e.screen.SetContent(x, h-1, ch, nil, e.style)
		}
	}
}

// drawStatus draws the status message on the status bar.
func (e *Editor) drawStatus() {
	e.drawStatusBar(e.status)
	e.status = "" // Clear status after drawing
}

// handleCommandMode processes key events in command mode.
// It handles switching to insert mode and processing ':' commands.
func (e *Editor) handleCommandMode(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		// Switch to insert mode
		e.inCommandMode = false
		e.dirty = true // Mark as dirty to trigger a redraw
	case tcell.KeyRune:
		if ev.Rune() == ':' {
			e.handleCommandInput()
		}
	}
}

// drawCmd draws the command line at the bottom.
func (e *Editor) drawCmd(cmd []rune) {
	e.drawStatusBar(string(cmd))
	_, h := e.screen.Size()
	e.screen.ShowCursor(len(cmd), h-1)
}

// toggleShowLineNumbers toggles the display of line numbers.
func (e *Editor) toggleShowLineNumbers() {
	e.showLineNumbers = !e.showLineNumbers
	e.dirty = true // Mark as dirty to trigger a redraw
}

// toggleHighlightCurrentLine toggles the highlighting of the current line.
func (e *Editor) toggleHighlightCurrentLine() {
	e.highlightCurrentLine = !e.highlightCurrentLine
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleCommandInput handles the ':' command line at the bottom.
func (e *Editor) handleCommandInput() {
	e.cmd = []rune{':'}
	e.dirty = true
	for inCmd := true; inCmd; {
		e.draw()
		ev := e.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc:
				// Exit command input, redraw main buffer
				e.cmd = []rune{}
				inCmd = false
				e.dirty = true
				e.inCommandMode = false
			case tcell.KeyEnter:
				// Execute command
				command := string(e.cmd)
				switch {
				case strings.HasPrefix(command, ":e "):
					e.executeEditCommand(command)
				case command == ":e":
					e.executeReloadCommand()
				case strings.HasPrefix(command, ":w "):
					e.executeSaveAsCommand(command)
				case command == ":w":
					e.executeSaveCommand()
				case command == ":q":
					e.executeQuitCommand()
				case command == ":ln":
					e.toggleShowLineNumbers()
				case command == ":hl":
					e.toggleHighlightCurrentLine()
				default:
					e.showStatus("Unknown command: " + command)
				}
				e.cmd = []rune{}
				inCmd = false
				e.dirty = true
				e.inCommandMode = false
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				// Remove last character from command
				if len(e.cmd) > 1 {
					e.cmd = e.cmd[:len(e.cmd)-1]
					e.dirty = true
				}
			case tcell.KeyRune:
				// Add character to command
				e.cmd = append(e.cmd, ev.Rune())
				e.dirty = true
			}
		case *tcell.EventResize:
			e.screen.Sync()
			e.dirty = true
		}
	}
}

func (e *Editor) executeEditCommand(command string) {
	filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
	if filename == "" {
		e.showStatus("No filename specified for :e command")
		return
	}
	e.currentFilename = filename
	if err := e.loadFile(e.currentFilename); err != nil {
		e.showStatus("Error loading file: " + err.Error())
	}
}

func (e *Editor) executeReloadCommand() {
	if e.currentFilename != "" {
		if err := e.loadFile(e.currentFilename); err != nil {
			e.showStatus("Error loading file: " + err.Error())
		}
	} else {
		e.showStatus("No filename specified for :e command")
	}
}

func (e *Editor) executeSaveAsCommand(command string) {
	filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
	if filename != "" {
		e.saveFile(filename)
	} else {
		e.showStatus("No filename specified for :w command")
	}
}

func (e *Editor) executeSaveCommand() {
	if e.currentFilename != "" {
		e.saveFile(e.currentFilename)
	} else {
		e.showStatus("No filename specified for :w command")
	}
}

func (e *Editor) executeQuitCommand() {
	e.screen.Fini()
	os.Exit(0)
}

// handleInsertMode processes key events in insert mode.
// It handles character insertion, line splitting, and cursor movement.
func (e *Editor) handleInsertMode(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		// Switch to command mode
		e.dirty = true
		e.inCommandMode = true
	case tcell.KeyRune:
		if r := ev.Rune(); r != 0 {
			// Insert character at cursor position
			if e.cursorY >= len(e.lines) {
				e.lines = append(e.lines, []rune{})
			}
			line := e.lines[e.cursorY]
			if e.cursorX > len(line) {
				e.cursorX = len(line)
			}
			newLine := append(line[:e.cursorX], append([]rune{r}, line[e.cursorX:]...)...)
			e.lines[e.cursorY] = newLine
			e.cursorX++
			e.virtualCursorX++ // Adjust virtual cursor for other characters
			e.dirty = true     // Mark as dirty
		}
	case tcell.KeyTab:
		// Insert a tab character
		if e.cursorY >= len(e.lines) {
			e.lines = append(e.lines, []rune{})
		}
		line := e.lines[e.cursorY]
		if e.cursorX > len(line) {
			e.cursorX = len(line)
		}
		newLine := append(line[:e.cursorX], append([]rune{'\t'}, line[e.cursorX:]...)...)
		e.lines[e.cursorY] = newLine
		e.cursorX += e.spacesPerTab
		e.virtualCursorX += e.spacesPerTab
		e.dirty = true // Mark as dirty
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Remove character before cursor or merge lines
		if e.cursorY < len(e.lines) && e.cursorX > 0 {
			line := e.lines[e.cursorY]
			if line[e.cursorX-1] == '\t' {
				// Remove the tab character
				e.lines[e.cursorY] = slices.Delete(line, e.cursorX-1, e.cursorX)
				e.cursorX--
				e.virtualCursorX -= e.spacesPerTab // Adjust virtual cursor position
			} else {
				e.lines[e.cursorY] = slices.Delete(line, e.cursorX-1, e.cursorX)
				e.cursorX--
				e.virtualCursorX-- // Adjust virtual cursor position
			}
			e.dirty = true // Mark as dirty
		} else if e.cursorY > 0 {
			// Merge with previous line
			prevLine := e.lines[e.cursorY-1]
			e.cursorX = len(prevLine) // Set cursor position to the end of the previous line
			e.virtualCursorX = 0
			for _, r := range prevLine {
				if r == '\t' {
					e.virtualCursorX += e.spacesPerTab
				} else {
					e.virtualCursorX++
				}
			}
			e.lines[e.cursorY-1] = append(prevLine, e.lines[e.cursorY]...)
			e.lines = slices.Delete(e.lines, e.cursorY, e.cursorY+1)
			e.cursorY--
			e.dirty = true // Mark as dirty
		}
	case tcell.KeyDelete:
		// Remove character at cursor or merge lines
		if e.cursorY < len(e.lines) && e.cursorX < len(e.lines[e.cursorY]) {
			line := e.lines[e.cursorY]
			e.lines[e.cursorY] = slices.Delete(line, e.cursorX, e.cursorX+1)
			e.dirty = true // Mark as dirty
		} else if e.cursorY < len(e.lines)-1 {
			// Merge with next line
			nextLine := e.lines[e.cursorY+1]
			e.lines[e.cursorY] = append(e.lines[e.cursorY], nextLine...)
			e.lines = slices.Delete(e.lines, e.cursorY+1, e.cursorY+2)
			e.dirty = true // Mark as dirty
		}
	case tcell.KeyEnter:
		// Split the current line at the cursor position
		if e.cursorY < len(e.lines) {
			line := e.lines[e.cursorY]
			newLine := line[e.cursorX:]
			e.lines[e.cursorY] = line[:e.cursorX]
			e.lines = append(e.lines[:e.cursorY+1], append([][]rune{newLine}, e.lines[e.cursorY+1:]...)...)
			e.cursorY++
			e.cursorX = 0
			e.virtualCursorX = 0
			e.dirty = true // Mark as dirty to redraw
		}
	case tcell.KeyLeft:
		if e.cursorX > 0 {
			line := e.lines[e.cursorY]
			if line[e.cursorX-1] == '\t' {
				// Skip over the tab character
				e.virtualCursorX -= e.spacesPerTab
			} else {
				e.virtualCursorX--
			}
			e.cursorX--
		} else if e.cursorY > 0 {
			e.cursorY--
			e.cursorX = len(e.lines[e.cursorY])
			e.virtualCursorX = 0
			for i := range e.lines[e.cursorY] {
				if e.lines[e.cursorY][i] == '\t' {
					e.virtualCursorX += e.spacesPerTab
				} else {
					e.virtualCursorX++
				}
			}
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyRight:
		if e.cursorY < len(e.lines) && e.cursorX < len(e.lines[e.cursorY]) {
			line := e.lines[e.cursorY]
			if line[e.cursorX] == '\t' {
				// Skip over the tab character
				e.virtualCursorX += e.spacesPerTab
			} else {
				e.virtualCursorX++
			}
			e.cursorX++
		} else if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			e.cursorX = 0
			e.virtualCursorX = 0
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyUp:
		if e.cursorY > 0 {
			eol := e.cursorX == len(e.lines[e.cursorY])
			e.cursorY--
			prevLine := e.lines[e.cursorY]
			if e.cursorX > 0 && (eol || e.cursorX > len(prevLine)) {
				e.cursorX = len(prevLine)
			}
			e.virtualCursorX = 0
			for i := range e.cursorX {
				if prevLine[i] == '\t' {
					e.virtualCursorX += e.spacesPerTab
				} else {
					e.virtualCursorX++
				}
			}
		}
		e.dirty = true // Mark as dirty to redraw cursor position
	case tcell.KeyDown:
		if e.cursorY < len(e.lines)-1 {
			eol := e.cursorX == len(e.lines[e.cursorY])
			e.cursorY++
			nextLine := e.lines[e.cursorY]
			if e.cursorX > 0 && (eol || e.cursorX > len(nextLine)) {
				e.cursorX = len(nextLine)
			}
			e.virtualCursorX = 0
			for i := range e.cursorX {
				if nextLine[i] == '\t' {
					e.virtualCursorX += e.spacesPerTab
				} else {
					e.virtualCursorX++
				}
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
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
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
			// Move cursor to the bottom of the screen
			e.cursorY = e.offsetY + h - 1
			if e.cursorY >= len(e.lines) {
				e.cursorY = len(e.lines) - 1
			}
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
			e.dirty = true // Mark as dirty to redraw
		}
	case tcell.KeyHome:
		// Move cursor to the beginning of the current line
		e.cursorX = 0
		e.virtualCursorX = 0
		e.dirty = true // Mark as dirty to redraw
	case tcell.KeyEnd:
		// Move cursor to the end of the current line
		if e.cursorY < len(e.lines) {
			e.cursorX = len(e.lines[e.cursorY])
			e.virtualCursorX = 0
			for _, r := range e.lines[e.cursorY] {
				if r == '\t' {
					e.virtualCursorX += e.spacesPerTab
				} else {
					e.virtualCursorX++
				}
			}
		}
		e.dirty = true // Mark as dirty to redraw
	}
}

// adjustOffsets ensures the cursor is always visible in the viewport.
// It adjusts the horizontal and vertical offsets based on the cursor position.
func (e *Editor) adjustOffsets() {
	w, h := e.screen.Size()

	// Ensure the cursor is visible horizontally
	if e.virtualCursorX < e.offsetX {
		e.offsetX = e.virtualCursorX
		e.dirty = true
	} else if e.virtualCursorX >= e.offsetX+w {
		e.offsetX = e.virtualCursorX - w + 1
		e.dirty = true
	}

	// Ensure the cursor is visible vertically
	if e.cursorY < e.offsetY {
		e.offsetY = e.cursorY
		e.dirty = true
	} else if e.cursorY >= e.offsetY+h-1 {
		e.offsetY = e.cursorY - h + 1
		e.dirty = true
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

	defer screen.Fini() // Ensure cleanup is deferred

	screen.Clear()
	// Set default style: white foreground, black background
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

	editor := NewEditor(screen, style)

	// If a filename is provided as an argument, load it; otherwise, start with an empty buffer
	if len(os.Args) > 1 {
		if err := editor.loadFile(os.Args[1]); err != nil {
			editor.showStatus("Error loading file: " + err.Error())
		} else {
			editor.draw()
		}
	} else {
		editor.draw()
	}

	// Main event loop
	for {
		ev := editor.screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			if editor.inCommandMode {
				editor.handleCommandMode(ev)
			} else {
				editor.handleInsertMode(ev)
			}
		case *tcell.EventResize:
			editor.screen.Sync()
			editor.dirty = true // Mark as dirty to redraw on resize
		}

		// Adjust horizontal and vertical offsets if cursor is out of visible area
		editor.adjustOffsets()

		// Redraw the screen
		editor.draw()
	}
}
