package config

import (
	"io"
	"os"
	"path/filepath"
)

// Copies a single file, following symlinks (the target's content is copied).
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// Recursively copies src to dst, following symlinks and skipping any path
// whose relative path matches an exclusion pattern.
func CopyTree(src, dst string, excludes []string) error {
	return copyTree(src, src, dst, excludes, map[string]bool{})
}

func copyTree(base, src, dst string, excludes []string, visited map[string]bool) error {
	info, err := os.Stat(src) // follows symlinks
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(src); err == nil {
		abs, _ := filepath.Abs(resolved)
		if visited[abs] {
			return nil // symlink cycle
		}
		visited[abs] = true
		defer delete(visited, abs)
	}
	if rel, err := filepath.Rel(base, src); err == nil && rel != "" && rel != "." {
		for _, pattern := range excludes {
			if matched, _ := filepath.Match(pattern, rel); matched {
				return nil
			}
		}
	}
	if !info.IsDir() {
		return CopyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(base, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), excludes, visited); err != nil {
			return err
		}
	}
	return nil
}
