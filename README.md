# morgul

Declarative (development) pod management tool for rootless Podman.

## Quickstart

Run the included `scripts/init.sh` shell script to copy the example modules, presets, and the base template into `~/.config/morgul`. Then run `scripts/build.sh` which produces the plug-and-play binary into `bin`.

## Modules & presets

Configuration is split into two layers:

- **Modules** (`~/.config/morgul/modules/*.json`): composable units of tooling. Each module declares `packages` (APT), `installers` (shell commands run at build time), `configs` (host files/dirs copied into the image), plus runtime `ports`, `virtual_volumes`, and `env_files`.
- **Presets** (`~/.config/morgul/presets/*.json`): named, ordered combinations of modules plus project-level defaults (`project_mount`).
- **Colors** (`~/.config/morgul/colors.json`): optional TUI colorscheme (omitted keys fall back to built-in defaults).

When creating a pod you can pick a preset, or choose "custom" and select modules ad-hoc. The resolved module list is persisted to the pod's labels and `pod_spec.json`, so preset/modules setup of each pod is tracker through its lifetime.

The directives shared by modules and presets are:

- `packages`: List of APT packages to install during image build
- `installers`: List of shell commands (or chained commands) executed during image build by the pod user (use `sudo` for root)
- `configs`: Host files/dirs to copy into the image at build time
    - Format: `{ "src": <path>, "dst": <path>, "exclude": <pattern> }`. Symlinks are followed. Symlinks are followed, so configs can be managed with tools like `stow`.
- `ports`: Host-to-pod port forwarding
    - Format: `{ "host": <port>, "pod": <port> }`
- `virtual_volumes`: Named Podman volumes (owned by the pod user) for persistent pod-local storage (e.g. caches)
    - Format: `{ "name": <name>, "path": <path_on_pod> }`
- `env_files`: Host `.env` files whose key-value pairs are injected as environment variables into the pod at runtime

Notably ports, volume mounts, project mount, and env files are also configurable via the pod edit view (mapped to `e`) so even presets don't need to be adjusted for every project-specific tweak.

## Pod creation in practice

Morgul takes the user-supplied configs, and substitutes the values into the base template (`Dockerfile.base`, based on `buildpack-deps:trixie`) by replacing the placeholders (`{{PROFILE_PKGS}}`, `{{PROFILE_DIRS}}`, `{{PROFILE_INSTALLERS}}`, and `{{PROFILE_CONFIGS}}`). To persist configuration across reboots and recreations, a `.morgul` directory is created in the project root. Inside it are per-pod scoped directories containing:

- `Dockerfile.sd`: The substituted Dockerfile combining the base template with the resolved modules
- `pod_spec.json`: The pod config (name, project path, image tag, preset, modules, mounts, etc.)
- `build.log`/`recreate.log`: Log files from initial creation and recreation (for module/preset debugging purposes)

A temporary `configs` directory stages the copied dotfiles into the build context and is cleaned up automatically after pod creation finishes.

## Keyboard mappings

| Key | Action |
| - | - |
| Type (insert mode) | Fuzzy filter by pod name |
| `jk` / `Esc` | Leave insert mode (default mode) |
| `i` | Enter insert mode |
| `j`/`k` | Move selection |
| `Enter` | Attach to selected pod |
| `n` | New pod (preset or custom modules) |
| `e` | Edit pod config (ports, volumes, env files, project mount) |
| `s` | Stop selected pod |
| `x` | Kill (force stop) selected pod |
| `d` | Eemove selected pod |
| `p` | Purge all pods, images, volumes, and `.morgul` dirs |
| `q` | Quit |

