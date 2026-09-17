package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	content := []byte("hello morgul")
	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	os.WriteFile(srcPath, content, 0644)

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(result), string(content))
	}
}

func TestCopyFileNotFound(t *testing.T) {
	dstDir := t.TempDir()
	if err := CopyFile("/nonexistent/path/file.txt", filepath.Join(dstDir, "dest.txt")); err == nil {
		t.Error("expected error for nonexistent source, got nil")
	}
}

func TestCopyTree(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, "sub", "nested"), 0755)
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "child.txt"), []byte("child"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "nested", "deep.txt"), []byte("deep"), 0644)

	dstPath := filepath.Join(dstDir, "copy")
	if err := CopyTree(srcDir, dstPath, nil); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, want string }{
		{"root.txt", "root"},
		{filepath.Join("sub", "child.txt"), "child"},
		{filepath.Join("sub", "nested", "deep.txt"), "deep"},
	} {
		data, err := os.ReadFile(filepath.Join(dstPath, tc.name))
		if err != nil {
			t.Fatalf("read %s: %v", tc.name, err)
		}
		if string(data) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, string(data), tc.want)
		}
	}
}

func TestCopyTreeExcludeDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, "plugin"), 0755)
	os.WriteFile(filepath.Join(srcDir, "init.lua"), []byte("init"), 0644)
	os.WriteFile(filepath.Join(srcDir, "plugin", "packer.lua"), []byte("compiled"), 0644)

	dstPath := filepath.Join(dstDir, "copy")
	if err := CopyTree(srcDir, dstPath, []string{"plugin"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dstPath, "init.lua")); err != nil {
		t.Errorf("init.lua should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstPath, "plugin")); !os.IsNotExist(err) {
		t.Error("plugin directory should be excluded")
	}
}

func TestCopyTreeExcludeGlob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "keep.lua"), []byte("keep"), 0644)
	os.WriteFile(filepath.Join(srcDir, "skip.log"), []byte("skip"), 0644)

	dstPath := filepath.Join(dstDir, "copy")
	if err := CopyTree(srcDir, dstPath, []string{"*.log"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dstPath, "keep.lua")); err != nil {
		t.Errorf("keep.lua should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstPath, "skip.log")); !os.IsNotExist(err) {
		t.Error("skip.log should be excluded")
	}
}

func TestCopyTreeFollowsSymlinkedFile(t *testing.T) {
	realDir := t.TempDir()
	os.WriteFile(filepath.Join(realDir, "real.txt"), []byte("real content"), 0644)

	srcDir := t.TempDir()
	if err := os.Symlink(filepath.Join(realDir, "real.txt"), filepath.Join(srcDir, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	dstDir := t.TempDir()
	if err := CopyTree(srcDir, filepath.Join(dstDir, "copy"), nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "copy", "link.txt"))
	if err != nil {
		t.Fatalf("copied symlink target not found: %v", err)
	}
	if string(data) != "real content" {
		t.Errorf("got %q, want %q", string(data), "real content")
	}
}

func TestCopyTreeFollowsSymlinkedDir(t *testing.T) {
	realDir := t.TempDir()
	os.WriteFile(filepath.Join(realDir, "inner.txt"), []byte("inner"), 0644)

	srcDir := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(srcDir, "linked-dir")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	dstDir := t.TempDir()
	if err := CopyTree(srcDir, filepath.Join(dstDir, "copy"), nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "copy", "linked-dir", "inner.txt"))
	if err != nil {
		t.Fatalf("nested symlinked dir content not found: %v", err)
	}
	if string(data) != "inner" {
		t.Errorf("got %q, want %q", string(data), "inner")
	}
}
