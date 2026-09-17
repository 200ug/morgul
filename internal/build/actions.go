package build

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/2ug/morgul/internal/podman"
	"codeberg.org/2ug/morgul/internal/util"
)

// Returns a container name and hostname that are guaranteed to be unique
// against the currently existing containers.
func UniqueContainerName(client podman.Client, projectName string) (string, string) {
	for {
		hostname := util.RandomHostname()
		name := fmt.Sprintf("%s-%s", projectName, hostname)
		if exists, _ := client.ContainerExists(name); !exists {
			return name, hostname
		}
	}
}

// Builds and runs a brand new container, logging output to build.log. On
// success the spec is persisted to pod_spec.json.
func (s *Spec) Create(client podman.Client) error {
	logFile, err := openLog(s.BuildCtx, "build.log")
	if err != nil {
		return err
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "### CONTAINER BUILD LOG ###\n")
	fmt.Fprintf(logFile, "Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Image: %s\n", s.ImageTag)
	fmt.Fprintf(logFile, "Project: %s\n", s.ProjectPath)

	if err := s.buildImage(client, logFile); err != nil {
		return err
	}
	if err := s.runContainer(client, logFile); err != nil {
		return err
	}
	return s.WriteToDisk()
}

// Stops and removes the old container instance, rebuilding the image only if
// it no longer exists, then runs a fresh container with the updated spec.
func (s *Spec) Recreate(client podman.Client) error {
	logFile, err := openLog(s.BuildCtx, "recreate.log")
	if err != nil {
		return err
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "### CONTAINER RECREATE LOG ###\n")
	fmt.Fprintf(logFile, "Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Image: %s\n", s.ImageTag)
	fmt.Fprintf(logFile, "Project: %s\n", s.ProjectPath)

	if !client.ImageExists(s.ImageTag) {
		if err := s.buildImage(client, logFile); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(logFile, "Reusing existing image %s\n", s.ImageTag)
	}

	if exists, running := client.ContainerExists(s.ContainerName); exists {
		if running {
			if err := client.StopContainer(s.ContainerName, false); err != nil {
				return err
			}
		}
		if err := client.RemoveContainer(s.ContainerName); err != nil {
			return err
		}
	}

	if err := s.runContainer(client, logFile); err != nil {
		return err
	}
	return s.WriteToDisk()
}

// Removes the container, its image, and its scoped .morgul directory.
func (s *Spec) Remove(client podman.Client) error {
	if exists, running := client.ContainerExists(s.ContainerName); exists {
		if running {
			if err := client.StopContainer(s.ContainerName, true); err != nil {
				return err
			}
		}
		if err := client.RemoveContainer(s.ContainerName); err != nil {
			return err
		}
	}
	if err := client.RemoveImage(s.ImageTag); err != nil {
		return err
	}
	return removeContainerDirs(s.ProjectPath, s.ContainerName)
}

// Removes a container and its image using only listing-derived metadata, so it
// works even when the project root (and thus pod_spec.json) has been removed.
func RemoveContainerByInfo(client podman.Client, c podman.Container) error {
	if exists, running := client.ContainerExists(c.Name); exists {
		if running {
			if err := client.StopContainer(c.Name, true); err != nil {
				return err
			}
		}
		if err := client.RemoveContainer(c.Name); err != nil {
			return err
		}
	}
	if err := client.RemoveImage(c.ImageTag); err != nil {
		return err
	}
	return removeContainerDirs(c.ProjectPath, c.Name)
}

// Purges all containers, their images, named volumes, and .morgul
// directories. Containers whose pod_spec.json is missing are still removed,
// using listing-derived metadata.
func Purge(client podman.Client, containers []podman.Container) error {
	volumes := map[string]struct{}{}
	for _, c := range containers {
		spec, err := LoadSpec(c.ProjectPath, c.Name)
		if err != nil {
			if rmErr := RemoveContainerByInfo(client, c); rmErr != nil {
				return rmErr
			}
			continue
		}
		if err := spec.Remove(client); err != nil {
			return err
		}
		for _, v := range spec.VirtualVolumes {
			volumes[v.Name] = struct{}{}
		}
	}
	for name := range volumes {
		if err := client.RemoveVolume(name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Spec) buildImage(client podman.Client, out *os.File) error {
	return client.BuildImage(s.ImageTag, filepath.Join(s.BuildCtx, "Dockerfile.sd"), s.BuildCtx, s.BuildArgs, out)
}

func (s *Spec) runContainer(client podman.Client, out *os.File) error {
	opts := podman.RunOpts{
		Name:     s.ContainerName,
		Hostname: s.Hostname,
		Labels:   s.Labels(),
		Image:    s.ImageTag,
	}
	if !podman.IsDarwin() {
		// map host UID to the same UID inside the container ("dev" instead of
		// "root"); on darwin the VM handles UID/GID mapping transparently.
		opts.Userns = "--userns=keep-id"
	}
	if s.ProjectMount {
		dir := filepath.Base(s.ProjectPath)
		opts.Workdir = fmt.Sprintf("/home/dev/%s", dir)
		opts.Mounts = append(opts.Mounts, fmt.Sprintf("%s:/home/dev/%s", s.ProjectPath, dir))
	}
	for _, v := range s.VirtualVolumes {
		opts.Mounts = append(opts.Mounts, fmt.Sprintf("%s:%s:U", v.Name, v.Path))
	}
	for _, p := range s.Ports {
		opts.Ports = append(opts.Ports, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}
	opts.EnvFiles = s.EnvFiles
	fmt.Fprintf(out, "\n### CONTAINER CREATE LOG ###\n")
	return client.RunContainer(opts, out)
}

func openLog(buildCtx, name string) (*os.File, error) {
	if err := os.MkdirAll(buildCtx, 0755); err != nil {
		return nil, err
	}
	return os.Create(filepath.Join(buildCtx, name))
}

func removeContainerDirs(projectPath, containerName string) error {
	sdDir := filepath.Join(projectPath, ".morgul")
	if err := os.RemoveAll(filepath.Join(sdDir, containerName)); err != nil {
		return err
	}
	entries, err := os.ReadDir(sdDir)
	if err != nil {
		// nothing to clean up if the parent doesn't exist or isn't readable
		return nil
	}
	if len(entries) == 0 {
		return os.Remove(sdDir)
	}
	return nil
}
