package podman

import (
	"io"
	"os/exec"
)

// Abstraction over the podman CLI, so the build and TUI layers can be tested
// against a fake without shelling out.
type Client interface {
	ListContainers() ([]Container, error)

	BuildImage(tag, dockerfile, context string, buildArgs map[string]string, out io.Writer) error
	RunContainer(opts RunOpts, out io.Writer) error

	StartContainer(name string) error
	StopContainer(name string, force bool) error
	RemoveContainer(name string) error
	RemoveImage(tag string) error
	RemoveVolume(name string) error
	AttachCmd(name, shell, detachKeys string) *exec.Cmd

	ImageExists(tag string) bool
	ContainerExists(name string) (exists, running bool)
}
