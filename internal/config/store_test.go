package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONFilesFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := t.TempDir()

	os.WriteFile(filepath.Join(dir, "regular.json"), []byte(`{"id":"regular"}`), 0644)
	os.WriteFile(filepath.Join(real, "linked.json"), []byte(`{"id":"linked"}`), 0644)
	if err := os.Symlink(filepath.Join(real, "linked.json"), filepath.Join(dir, "linked.json")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	// a symlink resolving to a directory named *.json must be skipped
	os.MkdirAll(filepath.Join(real, "dirlink.json"), 0755)
	if err := os.Symlink(filepath.Join(real, "dirlink.json"), filepath.Join(dir, "dirlink.json")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	paths, err := jsonFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("got %d paths, want 2: %v", len(paths), paths)
	}
	got := map[string]bool{}
	for _, p := range paths {
		got[filepath.Base(p)] = true
	}
	if !got["regular.json"] || !got["linked.json"] {
		t.Errorf("expected regular.json and linked.json, got %v", got)
	}
	if got["dirlink.json"] {
		t.Error("dirlink.json (symlink to dir) should have been skipped")
	}
}

func TestLoadModulesFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "modules"), 0755)
	os.WriteFile(filepath.Join(real, "mod.json"), []byte(`{"id":"mod","packages":["x"]}`), 0644)
	if err := os.Symlink(filepath.Join(real, "mod.json"), filepath.Join(dir, "modules", "mod.json")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	store := &Store{Dir: dir}
	mods, err := store.LoadModules()
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].ID != "mod" {
		t.Fatalf("got %v", mods)
	}
}
