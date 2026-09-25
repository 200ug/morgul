package podman

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const DetachKeys = "ctrl-x,ctrl-q"

// Implements Client by shelling out to the podman binary.
type ExecClient struct{}

// Verifies podman is available and (on darwin) ensures the podman machine is
// running before returning a client.
func NewClient() (*ExecClient, error) {
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, fmt.Errorf("podman not in PATH")
	}
	if IsDarwin() {
		if err := ensureMachineRunning(); err != nil {
			return nil, err
		}
	}
	return &ExecClient{}, nil
}

type psContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
	Mounts []string          `json:"Mounts"`
	Ports  []psPort          `json:"Ports"`
}

type psPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (c *ExecClient) ListContainers() ([]Container, error) {
	raw, err := c.output("ps", "-a", "--format", "json", "--filter", "label="+LabelRoot)
	if err != nil {
		return nil, err
	}
	var parsed []psContainer
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	containers := make([]Container, 0, len(parsed))
	for _, ps := range parsed {
		containers = append(containers, newContainer(ps))
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return containers, nil
}

func newContainer(ps psContainer) Container {
	var modules []string
	if raw, ok := ps.Labels[LabelModules]; ok && raw != "" {
		for _, m := range strings.Split(raw, ",") {
			if m = strings.TrimSpace(m); m != "" {
				modules = append(modules, m)
			}
		}
	}
	var ports []Port
	for _, p := range ps.Ports {
		ports = append(ports, Port{Host: p.HostPort, Container: p.ContainerPort, Protocol: p.Protocol})
	}
	name := ps.ID
	if len(ps.Names) > 0 && ps.Names[0] != "" {
		name = ps.Names[0]
	}
	return Container{
		Name:        name,
		ImageTag:    ps.Image,
		ProjectPath: ps.Labels[LabelProjectPath],
		Preset:      ps.Labels[LabelPreset],
		Modules:     modules,
		Running:     ps.State == "running",
		Mounts:      ps.Mounts,
		Ports:       ports,
	}
}

func (c *ExecClient) BuildImage(tag, dockerfile, context string, buildArgs map[string]string, out io.Writer) error {
	args := []string{"build", "-t", tag, "-f", dockerfile}
	keys := make([]string, 0, len(buildArgs))
	for k := range buildArgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, buildArgs[k]))
	}
	args = append(args, context)
	return c.runWithOutput(out, args...)
}

func (c *ExecClient) RunContainer(opts RunOpts, out io.Writer) error {
	args := []string{"run", "-d", "--name", opts.Name, "--hostname", opts.Hostname}
	for k, v := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	if opts.Userns != "" {
		args = append(args, opts.Userns)
	}
	if opts.Workdir != "" {
		args = append(args, "-w", opts.Workdir)
	}
	for _, m := range opts.Mounts {
		args = append(args, "-v", m)
	}
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	for _, ef := range opts.EnvFiles {
		args = append(args, "--env-file", ef)
	}
	args = append(args, opts.Image)
	return c.runWithOutput(out, args...)
}

func (c *ExecClient) StartContainer(name string) error {
	return c.run("start", name)
}

func (c *ExecClient) StopContainer(name string, force bool) error {
	action := "stop"
	if force {
		action = "kill"
	}
	return c.run(action, name)
}

func (c *ExecClient) RemoveContainer(name string) error {
	return c.run("rm", "-f", name)
}

func (c *ExecClient) RemoveImage(tag string) error {
	return c.run("rmi", tag)
}

func (c *ExecClient) RemoveVolume(name string) error {
	return c.run("volume", "rm", name)
}

func (c *ExecClient) AttachCmd(name, shell, detachKeys string) *exec.Cmd {
	return exec.Command("podman", "exec", "-it", "--detach-keys", detachKeys, name, shell)
}

func (c *ExecClient) ImageExists(tag string) bool {
	err := c.run("image", "inspect", tag, "--format", "{{.Id}}")
	return err == nil
}

func (c *ExecClient) ContainerExists(name string) (bool, bool) {
	out, err := c.output("inspect", name, "--format", "{{.State.Running}}")
	if err != nil {
		return false, false
	}
	running, err := strconv.ParseBool(strings.TrimSpace(string(out)))
	if err != nil {
		return true, false
	}
	return true, running
}

func (c *ExecClient) run(args ...string) error {
	cmd := exec.Command("podman", args...)
	return cmd.Run()
}

func (c *ExecClient) runWithOutput(out io.Writer, args ...string) error {
	cmd := exec.Command("podman", args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func (c *ExecClient) output(args ...string) ([]byte, error) {
	return exec.Command("podman", args...).Output()
}
