package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/2ug/morgul/internal/config"
)

func testBlueprint(name string) *config.Blueprint {
	return &config.Blueprint{Name: name, Modules: []string{"base"}}
}

func TestNewSpec_BuildCtxScopedByContainerName(t *testing.T) {
	projectPath := "/tmp/testproject"
	containerName := "myproject-atomics"

	spec, err := NewSpec(testBlueprint("golang"), projectPath, containerName, "atomics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.BuildCtx != filepath.Join(projectPath, ".morgul", containerName) {
		t.Errorf("BuildCtx: got %q, want %q", spec.BuildCtx, filepath.Join(projectPath, ".morgul", containerName))
	}
}

func TestNewSpec_DifferentContainersDifferentPaths(t *testing.T) {
	projectPath := "/tmp/testproject"

	spec1, err := NewSpec(testBlueprint("golang"), projectPath, "myproject-atomics", "atomics")
	if err != nil {
		t.Fatal(err)
	}
	spec2, err := NewSpec(testBlueprint("golang"), projectPath, "myproject-sietch", "sietch")
	if err != nil {
		t.Fatal(err)
	}

	if spec1.BuildCtx == spec2.BuildCtx {
		t.Errorf("two different containers share the same BuildCtx: %q", spec1.BuildCtx)
	}
	if spec1.ContainerName != "myproject-atomics" || spec2.ContainerName != "myproject-sietch" {
		t.Error("container names not preserved")
	}
}

func TestNewSpec_ImageTagIncludesModuleHash(t *testing.T) {
	spec, err := NewSpec(testBlueprint("python"), "/tmp/testproject", "testproj-mentat", "mentat")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(spec.ImageTag, "morgul-python-testproject-") {
		t.Errorf("ImageTag: got %q, want prefix morgul-python-testproject-", spec.ImageTag)
	}
}

func TestNewSpec_PresetAndModules(t *testing.T) {
	bp := &config.Blueprint{Name: "golang", Modules: []string{"base", "nvim", "golang"}}
	spec, err := NewSpec(bp, "/tmp/testproject", "testproj-mentat", "mentat")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Preset != "golang" {
		t.Errorf("Preset: got %q, want %q", spec.Preset, "golang")
	}
	if len(spec.Modules) != 3 || spec.Modules[2] != "golang" {
		t.Errorf("Modules: got %v", spec.Modules)
	}
}

func TestWriteToDisk_WritesToContainerScopedPath(t *testing.T) {
	projectDir := t.TempDir()
	spec, err := NewSpec(testBlueprint("golang"), projectDir, "myproject-atomics", "atomics")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spec.BuildCtx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := spec.WriteToDisk(); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	expected := filepath.Join(projectDir, ".morgul", "myproject-atomics", "pod_spec.json")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("pod_spec.json not found at %q", expected)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".morgul", "pod_spec.json")); err == nil {
		t.Error("pod_spec.json should NOT exist at old unscoped path")
	}
}

func TestLoadSpec_ReadsFromContainerScopedPath(t *testing.T) {
	projectDir := t.TempDir()
	spec, err := NewSpec(testBlueprint("golang"), projectDir, "myproject-sietch", "sietch")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spec.BuildCtx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := spec.WriteToDisk(); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSpec(projectDir, "myproject-sietch")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ContainerName != "myproject-sietch" || loaded.Hostname != "sietch" {
		t.Errorf("got %q/%q", loaded.ContainerName, loaded.Hostname)
	}
	if loaded.ProjectPath != projectDir {
		t.Errorf("ProjectPath: got %q", loaded.ProjectPath)
	}
}

func TestLoadSpec_NotFound(t *testing.T) {
	if _, err := LoadSpec(t.TempDir(), "nonexistent"); err == nil {
		t.Error("expected error for nonexistent spec, got nil")
	}
}

func TestLoadSpec_RequiresContainerName(t *testing.T) {
	if _, err := LoadSpec(t.TempDir(), ""); err == nil {
		t.Error("expected error when containerName is empty, got nil")
	}
}

func TestWriteToDisk_LoadSpec_RoundTrip(t *testing.T) {
	projectDir := t.TempDir()
	spec, err := NewSpec(&config.Blueprint{
		Name:           "golang",
		Modules:        []string{"base", "golang"},
		ProjectMount:   boolPtr(true),
		VirtualVolumes: []config.VirtualVolume{{Name: "myvol", Path: "/data"}},
		Ports:          []config.PortMap{{Host: 8080, Container: 80}},
	}, projectDir, "roundtrip-ghola", "ghola")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spec.BuildCtx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := spec.WriteToDisk(); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSpec(projectDir, "roundtrip-ghola")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ImageTag != spec.ImageTag {
		t.Error("round-trip mismatch on ImageTag")
	}
	if loaded.Preset != "golang" || len(loaded.Modules) != 2 {
		t.Errorf("Preset/Modules: got %q / %v", loaded.Preset, loaded.Modules)
	}
	if len(loaded.VirtualVolumes) != 1 || loaded.VirtualVolumes[0].Name != "myvol" {
		t.Errorf("VirtualVolumes: got %v", loaded.VirtualVolumes)
	}
	if len(loaded.Ports) != 1 || loaded.Ports[0].Host != 8080 {
		t.Errorf("Ports: got %v", loaded.Ports)
	}
}

func TestWriteToDisk_MultipleContainersNoConflict(t *testing.T) {
	projectDir := t.TempDir()

	spec1, _ := NewSpec(testBlueprint("golang"), projectDir, "proj-atomics", "atomics")
	spec2, _ := NewSpec(testBlueprint("golang"), projectDir, "proj-sietch", "sietch")
	os.MkdirAll(spec1.BuildCtx, 0755)
	os.MkdirAll(spec2.BuildCtx, 0755)
	if err := spec1.WriteToDisk(); err != nil {
		t.Fatal(err)
	}
	if err := spec2.WriteToDisk(); err != nil {
		t.Fatal(err)
	}

	var loaded1, loaded2 Spec
	data1, _ := os.ReadFile(filepath.Join(projectDir, ".morgul", "proj-atomics", "pod_spec.json"))
	data2, _ := os.ReadFile(filepath.Join(projectDir, ".morgul", "proj-sietch", "pod_spec.json"))
	json.Unmarshal(data1, &loaded1)
	json.Unmarshal(data2, &loaded2)

	if loaded1.ContainerName == loaded2.ContainerName {
		t.Error("two containers sharing a project should not overwrite each other's specs")
	}
	if loaded1.Hostname != "atomics" || loaded2.Hostname != "sietch" {
		t.Error("hostnames not preserved independently")
	}
}

func TestCleanupStagedConfigs_RemovesContainerScopedDir(t *testing.T) {
	projectDir := t.TempDir()
	configsDir := filepath.Join(projectDir, ".morgul", "cleanup-ghola", "configs")
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := CleanupStagedConfigs(projectDir, "cleanup-ghola"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configsDir); err == nil {
		t.Error("configs dir should have been removed")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".morgul", "cleanup-ghola")); os.IsNotExist(err) {
		t.Error("container-scoped dir should still exist after config cleanup")
	}
}

func TestSubstituteDockerfile_WritesToContainerScopedPath(t *testing.T) {
	projectDir := t.TempDir()
	configsDir := t.TempDir()
	base := filepath.Join(configsDir, "Dockerfile.base")
	content := "# t\n# {{PROFILE_PKGS}}\n# {{PROFILE_INSTALLERS}}\n# {{PROFILE_CONFIGS}}\n# {{PROFILE_DIRS}}\n"
	os.WriteFile(base, []byte(content), 0644)

	bp := testBlueprint("golang")
	bp.ProjectMount = boolPtr(true)
	if err := SubstituteDockerfile(bp, base, projectDir, "proj-mentat"); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(projectDir, ".morgul", "proj-mentat", "Dockerfile.sd")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("Dockerfile.sd not found at %q", expected)
	}
}

func TestSubstituteDockerfile_FollowsSymlink(t *testing.T) {
	projectDir := t.TempDir()
	real := t.TempDir()
	content := "# t\n# {{PROFILE_PKGS}}\n# {{PROFILE_INSTALLERS}}\n# {{PROFILE_CONFIGS}}\n# {{PROFILE_DIRS}}\n"
	os.WriteFile(filepath.Join(real, "Dockerfile.base"), []byte(content), 0644)

	link := filepath.Join(projectDir, "Dockerfile.base")
	if err := os.Symlink(filepath.Join(real, "Dockerfile.base"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if err := SubstituteDockerfile(testBlueprint("golang"), link, projectDir, "proj-mentat"); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(projectDir, ".morgul", "proj-mentat", "Dockerfile.sd")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("Dockerfile.sd not found at %q", expected)
	}
}

func TestStageConfigs_StagesToContainerScopedPath(t *testing.T) {
	projectDir := t.TempDir()
	userHome := t.TempDir()
	configDir := filepath.Join(userHome, ".config", "nvim")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "init.lua"), []byte("vim.cmd('set nu')"), 0644)

	bp := testBlueprint("golang")
	bp.Configs = []config.Config{{SourcePattern: "~/.config/nvim", DestinationPath: "~/.config/nvim"}}

	if err := StageConfigs(bp, userHome, projectDir, "proj-arrakeen"); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(projectDir, ".morgul", "proj-arrakeen", "configs", ".config", "nvim", "init.lua")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("staged config not found at %q", expected)
	}
}

func TestRemoveContainerDirs_RemovesParentWhenLast(t *testing.T) {
	projectDir := t.TempDir()
	sdDir := filepath.Join(projectDir, ".morgul")
	containerDir := filepath.Join(sdDir, "proj-atomics")
	os.MkdirAll(containerDir, 0755)
	os.WriteFile(filepath.Join(containerDir, "pod_spec.json"), []byte("{}"), 0644)

	if err := removeContainerDirs(projectDir, "proj-atomics"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sdDir); err == nil {
		t.Error("parent .morgul dir should be removed when empty")
	}
}

func TestRemoveContainerDirs_KeepsParentWithOtherContainers(t *testing.T) {
	projectDir := t.TempDir()
	os.MkdirAll(filepath.Join(projectDir, ".morgul", "proj-atomics"), 0755)
	os.MkdirAll(filepath.Join(projectDir, ".morgul", "proj-sietch"), 0755)

	if err := removeContainerDirs(projectDir, "proj-atomics"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".morgul", "proj-sietch")); err != nil {
		t.Error("other container dir should remain")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".morgul")); err != nil {
		t.Error("parent .morgul dir should remain")
	}
}

func TestRemoveContainerDirs_MissingDirsIsNoop(t *testing.T) {
	if err := removeContainerDirs(t.TempDir(), "does-not-exist"); err != nil {
		t.Fatalf("should be a no-op, got: %v", err)
	}
}

func boolPtr(b bool) *bool {
	return &b
}
