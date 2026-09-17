package build

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeberg.org/2ug/morgul/internal/config"
	"codeberg.org/2ug/morgul/internal/podman"
)

type fakeClient struct {
	exists  bool
	running bool

	stopped []string
	removed []string
	images  []string
	volumes []string
}

func (f *fakeClient) ListContainers() ([]podman.Container, error) { return nil, nil }
func (f *fakeClient) BuildImage(string, string, string, map[string]string, io.Writer) error {
	return nil
}
func (f *fakeClient) RunContainer(podman.RunOpts, io.Writer) error { return nil }
func (f *fakeClient) StartContainer(string) error                  { return nil }
func (f *fakeClient) StopContainer(name string, force bool) error {
	f.stopped = append(f.stopped, name)
	return nil
}
func (f *fakeClient) RemoveContainer(name string) error {
	f.removed = append(f.removed, name)
	return nil
}
func (f *fakeClient) RemoveImage(tag string) error {
	f.images = append(f.images, tag)
	return nil
}
func (f *fakeClient) RemoveVolume(name string) error {
	f.volumes = append(f.volumes, name)
	return nil
}
func (f *fakeClient) AttachCmd(string, string, string) *exec.Cmd { return nil }
func (f *fakeClient) ImageExists(string) bool                    { return false }
func (f *fakeClient) ContainerExists(string) (bool, bool)        { return f.exists, f.running }

func TestRemoveContainerByInfo(t *testing.T) {
	projectDir := t.TempDir()
	name := "proj-atomics"
	os.MkdirAll(filepath.Join(projectDir, ".morgul", name), 0755)

	fc := &fakeClient{exists: true, running: true}
	c := podman.Container{Name: name, ImageTag: "localhost/morgul-golang-proj-abc", ProjectPath: projectDir}

	if err := RemoveContainerByInfo(fc, c); err != nil {
		t.Fatal(err)
	}

	if len(fc.stopped) != 1 || fc.stopped[0] != name {
		t.Errorf("stop calls: got %v", fc.stopped)
	}
	if len(fc.removed) != 1 || fc.removed[0] != name {
		t.Errorf("remove calls: got %v", fc.removed)
	}
	if len(fc.images) != 1 || fc.images[0] != c.ImageTag {
		t.Errorf("image removals: got %v", fc.images)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".morgul")); !os.IsNotExist(err) {
		t.Error(".morgul dir should be removed when its last container is gone")
	}
}

func TestPurge_FallsBackWhenSpecMissing(t *testing.T) {
	fc := &fakeClient{exists: true, running: false}
	containers := []podman.Container{
		{Name: "a-missing", ImageTag: "img-a", ProjectPath: filepath.Join(t.TempDir(), "gone")},
	}

	if err := Purge(fc, containers); err != nil {
		t.Fatal(err)
	}

	if len(fc.removed) != 1 || fc.removed[0] != "a-missing" {
		t.Errorf("remove calls: got %v", fc.removed)
	}
	if len(fc.images) != 1 || fc.images[0] != "img-a" {
		t.Errorf("image removals: got %v", fc.images)
	}
	if len(fc.stopped) != 0 {
		t.Errorf("should not stop a down container, got %v", fc.stopped)
	}
}

func TestPurge_CollectsVolumesFromSpecs(t *testing.T) {
	projectDir := t.TempDir()
	bp := &config.Blueprint{
		Name:           "golang",
		Modules:        []string{"golang"},
		VirtualVolumes: []config.VirtualVolume{{Name: "vol1", Path: "/x"}},
	}
	spec, err := NewSpec(bp, projectDir, "proj-ghola", "ghola")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spec.BuildCtx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := spec.WriteToDisk(); err != nil {
		t.Fatal(err)
	}

	fc := &fakeClient{exists: true, running: false}
	containers := []podman.Container{
		{Name: "proj-ghola", ImageTag: "img", ProjectPath: projectDir},
	}

	if err := Purge(fc, containers); err != nil {
		t.Fatal(err)
	}

	if len(fc.volumes) != 1 || fc.volumes[0] != "vol1" {
		t.Errorf("volume removals: got %v", fc.volumes)
	}
}
