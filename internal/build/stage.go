package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/2ug/morgul/internal/config"
)

// Copies the blueprint's configs from their original locations into the
// container's .morgul/<container>/configs/ staging directory, so they are
// inside the podman build context. Follows symlinks, which allows configs to
// be managed with tools like stow.
func StageConfigs(bp *config.Blueprint, userHome, projectPath, containerName string) error {
	if len(bp.Configs) == 0 {
		return nil
	}
	tmpDir := filepath.Join(projectPath, ".morgul", containerName, "configs")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	for _, cf := range bp.Configs {
		src := strings.Replace(cf.SourcePattern, "~", userHome, 1)
		matches, err := filepath.Glob(src)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("config glob pattern %q returned no matches", cf.SourcePattern)
		}
		for _, match := range matches {
			if err := stageMatch(userHome, tmpDir, match, cf.Exclude); err != nil {
				return err
			}
		}
	}
	return nil
}

func stageMatch(userHome, tmpDir, match string, excludes []string) error {
	info, err := os.Stat(match)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(userHome, match)
	if err != nil {
		return err
	}
	dst := filepath.Join(tmpDir, rel)
	if info.IsDir() {
		return config.CopyTree(match, dst, excludes)
	}
	return config.CopyFile(match, dst)
}

func CleanupStagedConfigs(projectPath, containerName string) error {
	return os.RemoveAll(filepath.Join(projectPath, ".morgul", containerName, "configs"))
}
