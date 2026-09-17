package config

const DefaultShell = "/bin/zsh"

// A host file/dir to copy into the image at build time.
type Config struct {
	SourcePattern   string   `json:"src"`
	DestinationPath string   `json:"dst"`
	Exclude         []string `json:"exclude"`
}

// A host-to-container port forwarding.
type PortMap struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

// A named podman volume mounted at a container-local path.
type VirtualVolume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// A composable unit of tooling: packages, installers, configs, and runtime
// options that can be mixed into a container blueprint.
type Module struct {
	ID             string          `json:"id"`
	Description    string          `json:"description"`
	Packages       []string        `json:"packages"`
	Installers     []string        `json:"installers"`
	Configs        []Config        `json:"configs"`
	Ports          []PortMap       `json:"ports"`
	VirtualVolumes []VirtualVolume `json:"virtual_volumes"`
	EnvFiles       []string        `json:"env_files"`
}

// A named, ordered selection of modules plus project-level defaults.
type Preset struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Shell        string   `json:"shell"`
	ProjectMount *bool    `json:"project_mount"`
	Modules      []string `json:"modules"`
}

// The flattened result of resolving a preset (or ad-hoc module list) into a
// concrete set of build inputs.
type Blueprint struct {
	Name           string
	Modules        []string
	Shell          string
	Packages       []string
	Installers     []string
	Configs        []Config
	ProjectMount   *bool
	Ports          []PortMap
	VirtualVolumes []VirtualVolume
	EnvFiles       []string
}
