package tui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbletea"

	"codeberg.org/2ug/morgul/internal/build"
	"codeberg.org/2ug/morgul/internal/config"
	"codeberg.org/2ug/morgul/internal/podman"
)

type containersMsg struct {
	containers []podman.Container
	err        error
}

type opResultMsg struct {
	err   error
	notif string
	warn  string
}

type refreshMsg struct{}
type notifTickMsg struct{}
type flushJMsg struct{}
type attachDoneMsg struct {
	err error
}

func listContainersCmd(client podman.Client) tea.Cmd {
	return func() tea.Msg {
		cs, err := client.ListContainers()
		return containersMsg{containers: cs, err: err}
	}
}

// Recurring tick that drives container-list refreshes.
func refreshLoop() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(5 * time.Second)
		return refreshMsg{}
	}
}

func notifTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return notifTickMsg{} })
}

func createCmd(client podman.Client, bp *config.Blueprint, baseDockerfile, userHome, projectPath string) tea.Cmd {
	return func() tea.Msg {
		containerName, hostname := build.UniqueContainerName(client, filepath.Base(projectPath))
		if err := build.SubstituteDockerfile(bp, baseDockerfile, projectPath, containerName); err != nil {
			return opResultMsg{err: err}
		}
		if err := build.StageConfigs(bp, userHome, projectPath, containerName); err != nil {
			return opResultMsg{err: err}
		}
		defer build.CleanupStagedConfigs(projectPath, containerName)

		spec, err := build.NewSpec(bp, projectPath, containerName, hostname)
		if err != nil {
			return opResultMsg{err: err}
		}
		if err := spec.Create(client); err != nil {
			return opResultMsg{err: err}
		}
		return opResultMsg{notif: "Container created successfully"}
	}
}

func recreateCmd(client podman.Client, spec *build.Spec) tea.Cmd {
	return func() tea.Msg {
		if err := spec.Recreate(client); err != nil {
			return opResultMsg{err: err}
		}
		return opResultMsg{notif: "Container recreated successfully"}
	}
}

func removeCmd(client podman.Client, c podman.Container) tea.Cmd {
	return func() tea.Msg {
		spec, err := build.LoadSpec(c.ProjectPath, c.Name)
		if err != nil {
			// project root (and thus pod_spec.json) may be gone; infer from listing
			if rmErr := build.RemoveContainerByInfo(client, c); rmErr != nil {
				return opResultMsg{err: rmErr}
			}
			return opResultMsg{notif: "Container removed successfully", warn: "spec missing; inferred from listing"}
		}
		if err := spec.Remove(client); err != nil {
			return opResultMsg{err: err}
		}
		return opResultMsg{notif: "Container removed successfully"}
	}
}

func stopCmd(client podman.Client, name string, force bool) tea.Cmd {
	return func() tea.Msg {
		if err := client.StopContainer(name, force); err != nil {
			return opResultMsg{err: err}
		}
		if force {
			return opResultMsg{notif: "Container killed successfully"}
		}
		return opResultMsg{notif: "Container stopped successfully"}
	}
}

func purgeCmd(client podman.Client, containers []podman.Container) tea.Cmd {
	return func() tea.Msg {
		if err := build.Purge(client, containers); err != nil {
			return opResultMsg{err: err}
		}
		return opResultMsg{notif: fmt.Sprintf("Purged %d containers", len(containers))}
	}
}

func attachCmd(client podman.Client, c podman.Container) tea.Cmd {
	return func() tea.Msg {
		shell := config.DefaultShell
		if spec, err := build.LoadSpec(c.ProjectPath, c.Name); err == nil && spec.Shell != "" {
			shell = spec.Shell
		}
		if exists, running := client.ContainerExists(c.Name); exists && !running {
			if err := client.StartContainer(c.Name); err != nil {
				return attachDoneMsg{err: err}
			}
		}
		return tea.ExecProcess(client.AttachCmd(c.Name, shell, podman.DetachKeys), func(err error) tea.Msg {
			return attachDoneMsg{err: err}
		})()
	}
}
