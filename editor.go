package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

const (
	defaultShowLineNumbers      = true
	defaultHighlightCurrentLine = true
	defaultSpacesPerTab         = 4
)

// Editor holds all state for the text editor.
// This struct encapsulates the text buffer, cursor positions, viewport offsets, and other editor settings.
// It also manages the syntax highlighter and command input buffer.
type Editor struct {
	// Text buffer and cursor positions
	lines            [][]rune // Text buffer: each line is a slice of runes
	cursorX, cursorY int      // Cursor position in the buffer
	cursorOffsetX    int      // Virtual cursor position considering tabs
	offsetX, offsetY int      // Viewport offset for scrolling

	// Screen and rendering
	screen tcell.Screen
	style  tcell.Style
	w, h   int // Screen dimensions (width and height)

	// File management
	currentFilename string // Name of the currently loaded file
	dirty           bool   // True if the buffer or viewport has changed

	// Command mode
	inCommandMode bool   // True if in command mode (like Vim)
	cmd           []rune // Command line input buffer

	// Status and settings
	status               string // Status message to display
	showLineNumbers      bool   // True if line numbers should be displayed
	highlightCurrentLine bool   // True if the current line should be highlighted
	spacesPerTab         int    // Number of spaces to render for a tab character

	// Syntax highlighting
	highlighter *SyntaxHighlighter
}

// NewEditor initializes a new Editor instance.
// It sets up the text buffer, syntax highlighter, and default settings.
// Parameters:
// - screen: The tcell screen instance for rendering.
// - style: The default style for the editor.
// Returns: A pointer to the newly created Editor instance.
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
		spacesPerTab:         defaultSpacesPerTab, // Default to 4 spaces per tab
	}
}

// adjustOffsets ensures the cursor is always visible in the viewport.
// It adjusts the horizontal and vertical offsets based on the cursor position.
func (e *Editor) adjustOffsets() {
	// Ensure the cursor is visible horizontally
	if cursorX := e.cursorX + e.cursorOffsetX; cursorX < e.offsetX {
		e.offsetX = cursorX
		e.dirty = true // Mark as dirty to trigger a redraw
	} else if cursorX := e.cursorX + e.cursorOffsetX; cursorX >= e.offsetX+e.w {
		e.offsetX = cursorX - e.w + 1
		e.dirty = true // Mark as dirty to trigger a redraw
	}

	// Ensure the cursor is visible vertically
	if e.cursorY < e.offsetY {
		e.offsetY = e.cursorY
		e.dirty = true // Mark as dirty to trigger a redraw
	} else if e.cursorY >= e.offsetY+e.h-1 {
		e.offsetY = e.cursorY - e.h + 1
		e.dirty = true // Mark as dirty to trigger a redraw
	}
}

// draw renders the buffer and cursor to the screen, with Go syntax highlighting using AST.
// It handles line numbers, current line highlighting, and the status/command bar.
// This function skips rendering if the editor is not marked as dirty.
func (e *Editor) draw() {
	if !e.dirty {
		return // Skip drawing if nothing has changed
	}

	e.screen.Clear()

	// Calculate gutter width once
	gutterWidth := 0
	if e.showLineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(e.lines)))
	}

	// Draw visible lines
	for y := 0; y < e.h && y+e.offsetY < len(e.lines); y++ {
		// Reserve the last line for the status or command bar only if needed
		if (e.inCommandMode || e.status != "") && y == e.h-1 {
			break
		}

		lineIndex := y + e.offsetY
		line := e.lines[lineIndex]
		highlightMap := e.highlighter.GetHighlightMap(string(line))

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
		for x, i, r := startX, e.offsetX, ' '; x < e.w; r = ' ' {
			if i < len(line) {
				r = line[i]
			}
			style := highlightMap[i]
			if e.highlightCurrentLine && lineIndex == e.cursorY {
				style = style.Background(tcell.Color18)
			}
			if r == '\t' {
				// Render tab as spaces but treat as one character for layout
				for range e.spacesPerTab {
					if x < e.w {
						e.screen.SetContent(x, y, ' ', nil, style)
						x++
					}
				}
				i++ // Move to the next character in the line
				continue
			}
			e.screen.SetContent(x, y, r, nil, style)
			x++
			i++
		}
	}

	// Draw status or command line
	if e.inCommandMode {
		e.drawCmd(e.cmd)
	} else {
		e.drawStatus()

		cursorX := e.cursorX + e.cursorOffsetX - e.offsetX
		if e.showLineNumbers {
			cursorX += gutterWidth + 1
		}
		e.screen.ShowCursor(cursorX, e.cursorY-e.offsetY)
	}

	e.screen.Show()
	e.dirty = false // Reset dirty flag after drawing
}

// drawCmd draws the command line at the bottom of the screen.
// Parameters:
// - cmd: The command input buffer as a slice of runes.
func (e *Editor) drawCmd(cmd []rune) {
	e.drawStatusBar(string(cmd))
	e.screen.ShowCursor(len(cmd), e.h-1)
}

// drawStatus draws the status message on the status bar.
// It clears the status message after rendering.
func (e *Editor) drawStatus() {
	if e.status != "" {
		e.drawStatusBar(e.status)
		e.status = "" // Clear status after drawing
	}
}

func (e *Editor) drawStatusBar(content string) {
	for x := range e.w {
		e.screen.SetContent(x, e.h-1, ' ', nil, e.style)
	}
	for x, ch := range content {
		if x < e.w {
			e.screen.SetContent(x, e.h-1, ch, nil, e.style)
		}
	}
}

// executeEditCommand processes the :e command to load a new file.
// Parameters:
// - command: The full command string, including the filename.
func (e *Editor) executeEditCommand(command string) {
	filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
	if filename == "" {
		e.showStatus("No filename specified for :e command")
		return
	}
	if err := e.loadFile(filename); err != nil {
		e.showStatus("Error loading file: " + err.Error())
	}
}

// executeQuitCommand exits the editor and cleans up resources.
func (e *Editor) executeQuitCommand() {
	e.screen.Fini()
	os.Exit(0)
}

// executeReloadCommand reloads the currently loaded file.
// If no file is loaded, it displays an error message.
func (e *Editor) executeReloadCommand() {
	if e.currentFilename != "" {
		if err := e.loadFile(e.currentFilename); err != nil {
			e.showStatus("Error loading file: " + err.Error())
		}
	} else {
		e.showStatus("No filename specified for :e command")
	}
}

// executeSaveAsCommand processes the :w command to save the buffer to a new file.
// Parameters:
// - command: The full command string, including the filename.
func (e *Editor) executeSaveAsCommand(command string) {
	filename := strings.Trim(strings.TrimSpace(command[2:]), "\"")
	if filename != "" {
		e.saveFile(filename)
	} else {
		e.showStatus("No filename specified for :w command")
	}
}

// executeSaveCommand saves the buffer to the currently loaded file.
// If no file is loaded, it displays an error message.
func (e *Editor) executeSaveCommand() {
	if e.currentFilename != "" {
		e.saveFile(e.currentFilename)
	} else {
		e.showStatus("No filename specified for :w command")
	}
}

// handleBackspace removes the character before the cursor position.
// If the cursor is at the beginning of the line, it merges the current line with the previous line.
func (e *Editor) handleBackspace() {
	if e.cursorY < len(e.lines) && e.cursorX > 0 {
		line := e.lines[e.cursorY]
		if line[e.cursorX-1] == '\t' {
			// Remove the tab character
			e.lines[e.cursorY] = slices.Delete(line, e.cursorX-1, e.cursorX)
			e.cursorX--
			e.cursorOffsetX -= e.spacesPerTab - 1 // Adjust virtual cursor position
		} else {
			e.lines[e.cursorY] = slices.Delete(line, e.cursorX-1, e.cursorX)
			e.cursorX--
		}
		e.dirty = true // Mark as dirty
	} else if e.cursorY > 0 {
		// Merge with previous line
		prevLine := e.lines[e.cursorY-1]
		e.cursorX = len(prevLine) // Set cursor position to the end of the previous line
		e.cursorOffsetX = 0
		for _, r := range prevLine {
			if r == '\t' {
				e.cursorOffsetX += e.spacesPerTab - 1
			}
		}
		e.lines[e.cursorY-1] = append(prevLine, e.lines[e.cursorY]...)
		e.lines = slices.Delete(e.lines, e.cursorY, e.cursorY+1)
		e.cursorY--
		e.dirty = true // Mark as dirty
	}
}

// handleCommandInput handles the ':' command line at the bottom.
// It processes user input and executes commands like :e, :w, and :q.
func (e *Editor) handleCommandInput() {
	e.cmd = []rune{':'}
	e.dirty = true // Mark as dirty to trigger a redraw
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
				e.dirty = true // Mark as dirty to trigger a redraw
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
				e.dirty = true // Mark as dirty to trigger a redraw
				e.inCommandMode = false
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				// Remove last character from command
				if len(e.cmd) > 1 {
					e.cmd = e.cmd[:len(e.cmd)-1]
					e.dirty = true // Mark as dirty to trigger a redraw
				}
			case tcell.KeyRune:
				// Add character to command
				e.cmd = append(e.cmd, ev.Rune())
				e.dirty = true // Mark as dirty to trigger a redraw
			}
		case *tcell.EventResize:
			e.updateScreenSize()
		}
	}
}

// handleCommandMode processes key events in command mode.
// It handles switching to insert mode and processing ':' commands.
// Parameters:
// - ev: The key event to process.
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

// handleDelete removes the character at the cursor position.
// If the cursor is at the end of the line, it merges the current line with the next line.
func (e *Editor) handleDelete() {
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
}

// handleEnter splits the current line at the cursor position.
// The text after the cursor is moved to a new line.
func (e *Editor) handleEnter() {
	if e.cursorY < len(e.lines) {
		line := e.lines[e.cursorY]
		newLine := line[e.cursorX:]
		e.lines[e.cursorY] = line[:e.cursorX]
		e.lines = append(e.lines[:e.cursorY+1], append([][]rune{newLine}, e.lines[e.cursorY+1:]...)...)
		e.cursorY++
		e.cursorX = 0
		e.cursorOffsetX = 0
		e.dirty = true // Mark as dirty to redraw
	}
}

// handleExitInsertMode switches the editor from insert mode to command mode.
func (e *Editor) handleExitInsertMode() {
	e.inCommandMode = true
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleInsertMode processes key events in insert mode.
// It handles character insertion, line splitting, and cursor movement.
// Parameters:
// - ev: The key event to process.
func (e *Editor) handleInsertMode(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		// Switch to command mode
		e.handleExitInsertMode()
	case tcell.KeyRune:
		if r := ev.Rune(); r != 0 {
			e.handleInsertRune(r)
		}
	case tcell.KeyTab:
		// Insert a tab character
		e.handleTab()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Remove character before cursor or merge lines
		e.handleBackspace()
	case tcell.KeyDelete:
		// Remove character at cursor or merge lines
		e.handleDelete()
	case tcell.KeyEnter:
		// Split the current line at the cursor position
		e.handleEnter()
	case tcell.KeyLeft:
		e.handleMoveLeft() // Mark as dirty to redraw cursor position
	case tcell.KeyRight:
		e.handleMoveRight() // Mark as dirty to redraw cursor position
	case tcell.KeyUp:
		e.handleMoveUp() // Mark as dirty to redraw cursor position
	case tcell.KeyDown:
		e.handleMoveDown() // Mark as dirty to redraw cursor position
	case tcell.KeyPgUp:
		// Scroll up one page minus one row
		e.handlePageUp()
	case tcell.KeyPgDn:
		// Scroll down one page minus one row
		e.handlePageDown()
	case tcell.KeyHome:
		// Move cursor to the beginning of the current line
		e.handleMoveToStart() // Mark as dirty to redraw
	case tcell.KeyEnd:
		// Move cursor to the end of the current line
		e.handleMoveToEnd() // Mark as dirty to redraw
	}
}

// handleInsertRune inserts a single character at the cursor position.
// Parameters:
// - r: The rune to insert.
func (e *Editor) handleInsertRune(r rune) {
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
	e.dirty = true // Mark as dirty
}

// handleMoveDown moves the cursor down by one line.
// It adjusts the cursor position to the end of the line if necessary.
func (e *Editor) handleMoveDown() {
	if e.cursorY < len(e.lines)-1 {
		eol := e.cursorX == len(e.lines[e.cursorY])
		e.cursorY++
		nextLine := e.lines[e.cursorY]
		if e.cursorX > 0 && (eol || e.cursorX > len(nextLine)) {
			e.cursorX = len(nextLine)
		}
		e.cursorOffsetX = 0
		for i := range e.cursorX {
			if nextLine[i] == '\t' {
				e.cursorOffsetX += e.spacesPerTab - 1
			}
		}
	}
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleMoveLeft moves the cursor one character to the left.
// If the cursor is at the beginning of the line, it moves to the end of the previous line.
func (e *Editor) handleMoveLeft() {
	if e.cursorX > 0 {
		line := e.lines[e.cursorY]
		if line[e.cursorX-1] == '\t' {
			// Skip over the tab character
			e.cursorOffsetX -= e.spacesPerTab - 1
		}
		e.cursorX--
	} else if e.cursorY > 0 {
		e.cursorY--
		e.cursorX = len(e.lines[e.cursorY])
		e.cursorOffsetX = 0
		for i := range e.lines[e.cursorY] {
			if e.lines[e.cursorY][i] == '\t' {
				e.cursorOffsetX += e.spacesPerTab - 1
			}
		}
	}
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleMoveRight moves the cursor one character to the right.
// If the cursor is at the end of the line, it moves to the beginning of the next line.
func (e *Editor) handleMoveRight() {
	if e.cursorY < len(e.lines) && e.cursorX < len(e.lines[e.cursorY]) {
		line := e.lines[e.cursorY]
		if line[e.cursorX] == '\t' {
			// Skip over the tab character
			e.cursorOffsetX += e.spacesPerTab - 1
		}
		e.cursorX++
	} else if e.cursorY < len(e.lines)-1 {
		e.cursorY++
		e.cursorX = 0
		e.cursorOffsetX = 0
	}
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleMoveToEnd moves the cursor to the end of the current line.
// It adjusts the virtual cursor position to account for tab characters.
func (e *Editor) handleMoveToEnd() {
	if e.cursorY < len(e.lines) {
		e.cursorX = len(e.lines[e.cursorY])
		e.cursorOffsetX = 0
		for _, r := range e.lines[e.cursorY] {
			if r == '\t' {
				e.cursorOffsetX += e.spacesPerTab - 1
			}
		}
	}
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleMoveToStart moves the cursor to the beginning of the current line.
func (e *Editor) handleMoveToStart() {
	e.cursorX = 0
	e.cursorOffsetX = 0
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handleMoveUp moves the cursor up by one line.
// It adjusts the cursor position to the end of the line if necessary.
func (e *Editor) handleMoveUp() {
	if e.cursorY > 0 {
		eol := e.cursorX == len(e.lines[e.cursorY])
		e.cursorY--
		prevLine := e.lines[e.cursorY]
		if e.cursorX > 0 && (eol || e.cursorX > len(prevLine)) {
			e.cursorX = len(prevLine)
		}
		e.cursorOffsetX = 0
		for i := range e.cursorX {
			if prevLine[i] == '\t' {
				e.cursorOffsetX += e.spacesPerTab - 1
			}
		}
	}
	e.dirty = true // Mark as dirty to trigger a redraw
}

// handlePageDown scrolls down one page minus one row.
// It adjusts the cursor position to stay within the visible area.
func (e *Editor) handlePageDown() {
	if e.offsetY < len(e.lines)-1 {
		e.offsetY += e.h - 1
		if e.offsetY > len(e.lines)-1 {
			e.offsetY = len(e.lines) - 1
		}
		// Move cursor to the bottom of the screen
		e.cursorY = e.offsetY + e.h - 1
		if e.cursorY >= len(e.lines) {
			e.cursorY = len(e.lines) - 1
		}
		if e.cursorX > len(e.lines[e.cursorY]) {
			e.cursorX = len(e.lines[e.cursorY])
		}
		e.dirty = true // Mark as dirty to redraw
	}
}

// handlePageUp scrolls up one page minus one row.
// It adjusts the cursor position to stay within the visible area.
func (e *Editor) handlePageUp() {
	if e.offsetY > 0 {
		e.offsetY -= e.h - 1
		if e.offsetY < 0 {
			e.offsetY = 0
		}
		e.cursorY = e.offsetY
		if e.cursorX > len(e.lines[e.cursorY]) {
			e.cursorX = len(e.lines[e.cursorY])
		}
		e.dirty = true // Mark as dirty to trigger a redraw
	}
}

// handleTab inserts a tab character at the cursor position.
// It adjusts the virtual cursor position to account for the tab width.
func (e *Editor) handleTab() {
	if e.cursorY >= len(e.lines) {
		e.lines = append(e.lines, []rune{})
	}
	line := e.lines[e.cursorY]
	if e.cursorX > len(line) {
		e.cursorX = len(line)
	}
	newLine := append(line[:e.cursorX], append([]rune{'\t'}, line[e.cursorX:]...)...)
	e.lines[e.cursorY] = newLine
	e.cursorX++
	e.cursorOffsetX += e.spacesPerTab - 1
	e.dirty = true // Mark as dirty to trigger a redraw
}

// loadFile loads a file into the editor buffer (entire file in memory).
// It clears the current buffer, reads the file line by line, and updates the syntax highlighter.
// Parameters:
// - filename: The path to the file to be loaded.
// Returns:
// - error: An error if the file cannot be opened or read.
func (e *Editor) loadFile(filename string) error {
	// Parse line and column from filename
	var line, col int
	parts := strings.Split(filename, ":")
	filename = filepath.Clean(parts[0])
	if len(parts) > 1 {
		line, _ = strconv.Atoi(parts[1])
		line-- // Convert to zero-based index
	}
	if len(parts) > 2 {
		col, _ = strconv.Atoi(parts[2])
		col-- // Convert to zero-based index
	}

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
	if line >= 0 && line < len(e.lines) {
		e.cursorY = line
		if col >= 0 && col < len(e.lines[line]) {
			e.cursorX = col
		}
	}
	// Update highlighter
	e.highlighter.SetFileExtension(filepath.Ext(filename))
	e.currentFilename = filename

	return nil
}

// saveFile saves the buffer to a file (entire file in memory).
// It writes each line of the buffer to the specified file.
// Parameters:
// - filename: The path to the file where the buffer will be saved.
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
// Parameters:
// - msg: The status message to display.
func (e *Editor) showStatus(msg string) {
	e.status = msg
	e.dirty = true // Mark as dirty to trigger a redraw
}

// toggleHighlightCurrentLine toggles the highlighting of the current line.
// This function marks the editor as dirty to trigger a redraw.
func (e *Editor) toggleHighlightCurrentLine() {
	e.highlightCurrentLine = !e.highlightCurrentLine
	e.dirty = true // Mark as dirty to trigger a redraw
}

// toggleShowLineNumbers toggles the display of line numbers.
// This function marks the editor as dirty to trigger a redraw.
func (e *Editor) toggleShowLineNumbers() {
	e.showLineNumbers = !e.showLineNumbers
	e.dirty = true // Mark as dirty to trigger a redraw
}

func (e *Editor) updateScreenSize() {
	e.screen.Sync()
	w, h := e.screen.Size()
	if w != e.w || h != e.h {
		e.w, e.h = w, h
		e.dirty = true // Mark as dirty to redraw on resize
	}
}
