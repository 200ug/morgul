package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// TUI color palette, overridable via colors.json in the config directory.
type Colors struct {
	Accent   string `json:"accent"`
	Dim      string `json:"dim"`
	Success  string `json:"success"`
	Error    string `json:"error"`
	Warning  string `json:"warning"`
	Text     string `json:"text"`
	OnAccent string `json:"on_accent"`
}

// Returns the built-in palette, used as the fallback for any color not set in
// the config file.
func DefaultColors() Colors {
	return Colors{
		Accent:   "#E8DB7D",
		Dim:      "#6B6B6B",
		Success:  "#5B9279",
		Error:    "#F95738",
		Warning:  "#84732B",
		Text:     "#EFF7FF",
		OnAccent: "#1A1A1A",
	}
}

// Loads colors.json from the config directory, falling back to the default
// palette when the file is absent or omits individual keys.
func (s *Store) LoadColors() (Colors, error) {
	colors := DefaultColors()
	raw, err := os.ReadFile(filepath.Join(s.Dir, "colors.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return colors, nil
		}
		return colors, err
	}
	if err := json.Unmarshal(raw, &colors); err != nil {
		return colors, err
	}
	return colors, nil
}
