package paths

import (
	"os"
	"path/filepath"
)

func DeckHome() string {
	if v := os.Getenv("CLAUDE_DECK_HOME"); v != "" {
		return v
	}
	return filepath.Join(userHome(), ".config", "claude-deck")
}

func ClaudeHome() string {
	if v := os.Getenv("CLAUDE_HOME"); v != "" {
		return v
	}
	return filepath.Join(userHome(), ".claude")
}

func Socket() string {
	if v := os.Getenv("CLAUDE_DECK_SOCKET"); v != "" {
		return v
	}
	return "claude-deck"
}

func ClaudeCommand() string {
	if v := os.Getenv("CLAUDE_DECK_CLAUDE"); v != "" {
		return v
	}
	return "claude"
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
