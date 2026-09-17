package podman

// Labels used to mark and identify morgul-managed containers.
const (
	LabelRoot        = "morgul"
	LabelProjectPath = "morgul.project-path"
	LabelPreset      = "morgul.preset"
	LabelModules     = "morgul.modules"
)

// The resolved view of a morgul-managed container, derived from a
// single `podman ps -a` + labels query.
type Container struct {
	Name        string
	ImageTag    string
	ProjectPath string
	Preset      string
	Modules     []string
	Running     bool
	Mounts      []string
	Ports       []Port
}

// A single published port mapping reported by podman.
type Port struct {
	Host      int
	Container int
	Protocol  string
}

// Describes a `podman run` invocation for a new container.
type RunOpts struct {
	Name     string
	Hostname string
	Labels   map[string]string
	Userns   string   // e.g. "--userns=keep-id"; empty on darwin
	Workdir  string   // optional working directory inside the container
	Mounts   []string // each entry is already formatted ("src:dst[:options]")
	Ports    []string // each entry is already formatted ("host:container")
	EnvFiles []string
	Image    string
}
