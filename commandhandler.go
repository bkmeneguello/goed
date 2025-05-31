package main

import (
	"os"
	"strings"
)

// CommandHandler processes editor commands.
type CommandHandler struct{}

// NewCommandHandler initializes a new CommandHandler.
func NewCommandHandler() *CommandHandler {
	return &CommandHandler{}
}

// HandleCommand processes a command string.
func (ch *CommandHandler) HandleCommand(editor *Editor, command string) {
	switch {
	case command == ":q":
		editor.screen.Fini()
		os.Exit(0)
	case strings.HasPrefix(command, ":e "):
		filename := strings.TrimSpace(command[2:])
		editor.loadFile(filename)
	case strings.HasPrefix(command, ":w "):
		filename := strings.TrimSpace(command[2:])
		editor.saveFile(filename)
	case command == ":w":
		if editor.currentFilename != "" {
			editor.saveFile(editor.currentFilename)
		} else {
			editor.showStatus("No filename specified for :w command")
		}
	default:
		editor.showStatus("Unknown command: " + command)
	}
}
