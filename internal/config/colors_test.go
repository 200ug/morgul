package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadColors_DefaultsWhenMissing(t *testing.T) {
	store := &Store{Dir: t.TempDir()}

	colors, err := store.LoadColors()
	if err != nil {
		t.Fatal(err)
	}
	if colors != DefaultColors() {
		t.Errorf("got %+v, want defaults %+v", colors, DefaultColors())
	}
}

func TestLoadColors_OverridesPartial(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "colors.json"), []byte(`{"accent":"#ff0000"}`), 0644); err != nil {
		t.Fatal(err)
	}

	colors, err := (&Store{Dir: dir}).LoadColors()
	if err != nil {
		t.Fatal(err)
	}
	if colors.Accent != "#ff0000" {
		t.Errorf("accent: got %q, want #ff0000", colors.Accent)
	}
	if colors.Dim != DefaultColors().Dim {
		t.Errorf("dim should fall back to default, got %q", colors.Dim)
	}
}

func TestLoadColors_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "colors.json"), []byte(`{not json`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := (&Store{Dir: dir}).LoadColors(); err == nil {
		t.Error("expected error for malformed colors.json, got nil")
	}
}
