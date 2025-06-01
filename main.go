package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

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
		}
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
			editor.updateScreenSize()
		}

		// Adjust horizontal and vertical offsets if cursor is out of visible area
		editor.adjustOffsets()

		// Redraw the editor if it is marked as dirty
		editor.draw()
	}
}
