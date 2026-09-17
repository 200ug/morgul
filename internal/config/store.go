package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store loads modules, presets, and the base dockerfile from a single config
// directory (defaults to ~/.config/morgul).
type Store struct {
	Dir string
}

func NewStore(userHome string) *Store {
	return &Store{Dir: filepath.Join(userHome, ".config", "morgul")}
}

func (s *Store) modulesDir() string     { return filepath.Join(s.Dir, "modules") }
func (s *Store) presetsDir() string     { return filepath.Join(s.Dir, "presets") }
func (s *Store) BaseDockerfile() string { return filepath.Join(s.Dir, "Dockerfile.base") }

func (s *Store) LoadModules() ([]Module, error) {
	paths, err := jsonFiles(s.modulesDir())
	if err != nil {
		return nil, err
	}
	modules := make([]Module, 0, len(paths))
	for _, p := range paths {
		m, err := decodeJSON[Module](p)
		if err != nil {
			return nil, err
		}
		modules = append(modules, *m)
	}
	return modules, nil
}

func (s *Store) LoadPresets() ([]Preset, error) {
	paths, err := jsonFiles(s.presetsDir())
	if err != nil {
		return nil, err
	}
	presets := make([]Preset, 0, len(paths))
	for _, p := range paths {
		pr, err := decodeJSON[Preset](p)
		if err != nil {
			return nil, err
		}
		presets = append(presets, *pr)
	}
	return presets, nil
}

func (s *Store) LoadModule(id string) (*Module, error) {
	return decodeJSON[Module](filepath.Join(s.modulesDir(), id+".json"))
}

func (s *Store) LoadPreset(id string) (*Preset, error) {
	return decodeJSON[Preset](filepath.Join(s.presetsDir(), id+".json"))
}

// Returns the sorted list of *.json files in dir, following symlinks so tools
// like stow can be used to manage configs.
func jsonFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			continue
		}
		paths = append(paths, full)
	}
	sort.Strings(paths)
	return paths, nil
}

func decodeJSON[T any](path string) (*T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var v T
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		return nil, err
	}
	return &v, nil
}
