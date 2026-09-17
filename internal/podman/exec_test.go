package podman

import "testing"

func TestNewContainer_ParsesNameLabelsAndModules(t *testing.T) {
	ps := psContainer{
		ID:    "abc123def456",
		Names: []string{"proj-atomics"},
		State: "running",
		Image: "morgul-golang-proj",
		Labels: map[string]string{
			LabelProjectPath: "/tmp/proj",
			LabelPreset:      "golang",
			LabelModules:     "base, nvim, golang",
		},
	}

	c := newContainer(ps)
	if c.Name != "proj-atomics" {
		t.Errorf("Name: got %q", c.Name)
	}
	if c.ImageTag != "morgul-golang-proj" {
		t.Errorf("ImageTag: got %q", c.ImageTag)
	}
	if c.ProjectPath != "/tmp/proj" {
		t.Errorf("ProjectPath: got %q", c.ProjectPath)
	}
	if c.Preset != "golang" {
		t.Errorf("Preset: got %q", c.Preset)
	}
	if len(c.Modules) != 3 || c.Modules[2] != "golang" {
		t.Errorf("Modules: got %v", c.Modules)
	}
	if !c.Running {
		t.Error("expected running state")
	}
}

func TestNewContainer_FallsBackToIDWithoutNames(t *testing.T) {
	ps := psContainer{ID: "abc123def456", State: "exited"}

	c := newContainer(ps)
	if c.Name != "abc123def456" {
		t.Errorf("Name: got %q, want ID fallback", c.Name)
	}
	if c.Running {
		t.Error("expected down state for exited container")
	}
}

func TestNewContainer_IgnoresEmptyModules(t *testing.T) {
	ps := psContainer{
		ID:     "abc123",
		Labels: map[string]string{LabelModules: " base, ,nvim "},
	}

	c := newContainer(ps)
	if len(c.Modules) != 2 || c.Modules[0] != "base" || c.Modules[1] != "nvim" {
		t.Errorf("Modules: got %v", c.Modules)
	}
}
