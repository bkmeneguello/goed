package main

import (
	"os"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEditorLoadFile(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "testfile.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.WriteString("Line1\nLine2\nLine3")
	tempFile.Close()

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()

	style := tcell.StyleDefault
	editor := NewEditor(screen, style)
	editor.loadFile(tempFile.Name())

	if len(editor.lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(editor.lines))
	}
	if string(editor.lines[0]) != "Line1" {
		t.Errorf("Expected 'Line1', got '%s'", string(editor.lines[0]))
	}
}

func TestEditorSaveFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()

	style := tcell.StyleDefault
	editor := NewEditor(screen, style)
	editor.lines = [][]rune{
		[]rune("Line1"),
		[]rune("Line2"),
	}

	editor.saveFile(tempFile.Name())

	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	expected := "Line1\nLine2\n"
	if string(content) != expected {
		t.Errorf("Expected file content '%s', got '%s'", expected, string(content))
	}
}

func TestEditorShowStatus(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()

	style := tcell.StyleDefault
	editor := NewEditor(screen, style)

	msg := "Test Status"
	editor.showStatus(msg)

	// Verify the status message is displayed (mock screen content check needed)
}

func TestEditorAdjustOffsets(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()

	style := tcell.StyleDefault
	editor := NewEditor(screen, style)

	editor.cursorX = 100
	editor.cursorY = 50
	editor.adjustOffsets()

	if editor.offsetX > editor.cursorX || editor.offsetY > editor.cursorY {
		t.Errorf("Offsets not adjusted correctly")
	}
}

func TestSyntaxHighlighterSetFileExtension(t *testing.T) {
	style := tcell.StyleDefault
	highlighter := NewSyntaxHighlighter(style)

	highlighter.SetFileExtension(".go")
	if _, ok := highlighter.factories[".go"]; !ok {
		t.Errorf("Expected .go highlighter to be set")
	}
}

func TestSyntaxHighlighterUnsupportedExtension(t *testing.T) {
	style := tcell.StyleDefault
	highlighter := NewSyntaxHighlighter(style)

	highlighter.SetFileExtension(".unsupported")
	if highlighter.current != nil {
		t.Errorf("Expected no highlighter for unsupported extension")
	}
}

func TestGoHighlighterGetHighlightMap(t *testing.T) {
	style := tcell.StyleDefault
	goHighlighter := NewGoHighlighter(style)

	src := "package main"
	highlightMap := goHighlighter.GetHighlightMap(src)
	if len(highlightMap) == 0 {
		t.Errorf("Expected highlight map to have entries")
	}
}

func TestGoHighlighterComplexSyntax(t *testing.T) {
	style := tcell.StyleDefault
	goHighlighter := NewGoHighlighter(style)

	src := "func main() { var x = 42 }"
	highlightMap := goHighlighter.GetHighlightMap(src)

	if len(highlightMap) == 0 {
		t.Errorf("Expected highlight map to have entries for complex syntax")
	}
}
