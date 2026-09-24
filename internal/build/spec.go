package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/2ug/morgul/internal/config"
	"codeberg.org/2ug/morgul/internal/podman"
)

// Spec is the persisted configuration for a single container, stored as
// pod_spec.json inside the project's .morgul/ directory.
type Spec struct {
	ContainerName  string                 `json:"container_name"`
	Hostname       string                 `json:"hostname"`
	Preset         string                 `json:"preset"`
	Modules        []string               `json:"modules"`
	ImageTag       string                 `json:"image_tag"`
	ProjectPath    string                 `json:"project_path"`
	ProjectMount   bool                   `json:"project_mount"`
	BuildCtx       string                 `json:"-"`
	ConfigsDir     string                 `json:"-"`
	BuildArgs      map[string]string      `json:"build_args"`
	VirtualVolumes []config.VirtualVolume `json:"virtual_volumes"`
	Ports          []config.PortMap       `json:"ports"`
	EnvFiles       []string               `json:"env_files"`
}

func NewSpec(bp *config.Blueprint, projectPath, containerName, hostname string) (*Spec, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectName := filepath.Base(projectPath)

	spec := &Spec{
		ContainerName: containerName,
		Hostname:      hostname,
		Preset:        bp.Name,
		Modules:       bp.Modules,
		ImageTag:      fmt.Sprintf("morgul-%s-%s-%s", bp.Name, projectName, moduleHash(bp.Modules)),
		ProjectPath:   projectPath,
		ProjectMount:  bp.ProjectMount == nil || *bp.ProjectMount,
		// NOTE: scoping with containerName is necessary to support multiple
		//       containers sharing the same project (.morgul) directory.
		BuildCtx:   filepath.Join(projectPath, ".morgul", containerName),
		ConfigsDir: filepath.Join(projectPath, ".morgul", containerName, "configs"),
		BuildArgs: map[string]string{
			"UID": strconv.Itoa(os.Getuid()),
			"GID": strconv.Itoa(os.Getgid()),
		},
		VirtualVolumes: bp.VirtualVolumes,
		Ports:          bp.Ports,
	}
	for _, envFile := range bp.EnvFiles {
		expPath := os.ExpandEnv(strings.Replace(envFile, "~", userHome, 1))
		if _, err := os.Stat(expPath); err != nil {
			return nil, err
		}
		spec.EnvFiles = append(spec.EnvFiles, expPath)
	}
	return spec, nil
}

func LoadSpec(projectPath, containerName string) (*Spec, error) {
	if containerName == "" {
		return nil, fmt.Errorf("container name is required")
	}
	specPath := filepath.Join(projectPath, ".morgul", containerName, "pod_spec.json")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	// recompute derived paths to support container recreation
	s.BuildCtx = filepath.Join(s.ProjectPath, ".morgul", s.ContainerName)
	s.ConfigsDir = filepath.Join(s.BuildCtx, "configs")
	return &s, nil
}

// Writes the spec as formatted JSON to the container's .morgul directory.
func (s *Spec) WriteToDisk() error {
	formatted, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.BuildCtx, "pod_spec.json"), formatted, 0644)
}

func (s *Spec) Labels() map[string]string {
	return map[string]string{
		podman.LabelRoot:        "",
		podman.LabelProjectPath: s.ProjectPath,
		podman.LabelPreset:      s.Preset,
		podman.LabelModules:     strings.Join(s.Modules, ","),
	}
}

func moduleHash(modules []string) string {
	sum := sha256.Sum256([]byte(strings.Join(modules, ",")))
	return hex.EncodeToString(sum[:])[:6]
}
