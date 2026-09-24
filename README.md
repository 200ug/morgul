# morgul

Opinionated take on a declarative dev container management tool. Built directly on top of rootless Podman command line interface.

## Basics

### Quickstart

Run the included `scripts/init.sh` shell script to copy the example modules, presets, and the base template into `~/.config/morgul`. Then run `scripts/build.sh` which produces the plug-and-play binary into `bin`.

### Modules & presets

Configuration is split into two layers:

- **Modules** (`~/.config/morgul/modules/*.json`): composable units of tooling. Each module declares `packages` (APT), `installers` (shell commands run at build time), `configs` (host files/dirs copied into the image), plus runtime `ports`, `virtual_volumes`, and `env_files`.
- **Presets** (`~/.config/morgul/presets/*.json`): named, ordered combinations of modules plus project-level defaults (`shell`, `project_mount`).
- **Colors** (`~/.config/morgul/colors.json`): optional TUI palette (`accent`, `dim`, `success`, `error`, `warning`, `text`, `on_accent`). Any omitted key falls back to the built-in default, so the file can hold a partial palette.

When creating a container you can pick a preset, or choose "custom" and select modules ad-hoc. The resolved module list is persisted to the container's labels and `pod_spec.json`, so the details pane always shows which preset/modules a container was built from.

The directives shared by modules and presets are:

- `packages`: List of APT packages to install during image build
- `installers`: List of shell commands (or chained commands) executed during image build by the container user (use `sudo` for root)
- `configs`: Host files/dirs to copy into the image at build time
    - Format: `{ "src": <path>, "dst": <path>, "exclude": <pattern> }`. Symlinks are followed, so configs can be managed with tools like `stow`.
- `ports`: Host-to-container port forwarding
    - Format: `{ "host": <port>, "container": <port> }`
- `virtual_volumes`: Named podman volumes (owned by the container user) for persistent container-local storage (e.g. caches)
    - Format: `{ "name": <name>, "path": <path_on_container> }`
- `env_files`: Host `.env` files whose key-value pairs are injected as environment variables into the container at runtime

Notably ports, volume mounts, project mount, and env files are also configurable via the container edit view (mapped to `e`) so presets don't need to be adjusted for every per-project tweak.

### Container creation

During creation, the resolved modules are injected into the base template (`Dockerfile.base`, based on `buildpack-deps:trixie`) by replacing the placeholders (`{{PROFILE_PKGS}}`, `{{PROFILE_DIRS}}`, `{{PROFILE_INSTALLERS}}`, and `{{PROFILE_CONFIGS}}`).

To persist configuration across reboots and recreations, a `.morgul` directory is created in the project root. Inside it are per-container scoped directories containing:

- `Dockerfile.sd`: The substituted Dockerfile combining the base template with the resolved modules
- `pod_spec.json`: The container config (name, project path, image tag, preset, modules, mounts, etc.)
- `build.log`/`recreate.log`: Log files from initial creation and recreation

A temporary `configs` directory stages the copied dotfiles into the build context and is cleaned up automatically after creation.

### Keyboard mappings

The interface is modal, vim-style:

| Key | Action |
| - | - |
| type (insert mode) | fuzzy filter by container name; top match auto-selected |
| `jk` / `ESC` | leave insert mode (default mode) |
| `i` | enter insert mode |
| `j`/`k` or `↑`/`↓` | move selection |
| `ENTER` | attach to selected container |
| `n` | new container (preset or custom modules) |
| `e` | edit container config (ports, volumes, env files, project mount) |
| `s` | stop selected container |
| `x` | kill (force stop) selected container |
| `d` | remove selected container |
| `p` | purge all containers, images, volumes, and `.morgul` dirs |
| `q` | quit |

The screen is split vertically into two panes: the left shows the search box, the container list (stopped containers dimmed), and the total count; the right shows details of the selected container (preset, modules, ports, mounts, etc.).

### Deleting without the project root

Containers whose project directory (and thus `pod_spec.json`) has been removed can still be stopped, killed, and deleted — the required metadata is inferred from the container listing and its labels.
