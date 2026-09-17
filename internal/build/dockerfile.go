package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/2ug/morgul/internal/config"
)

// Substitutes the blueprint's values into the base dockerfile and writes the
// result to the container's .morgul/ directory as Dockerfile.sd.
func SubstituteDockerfile(bp *config.Blueprint, baseDockerfilePath, projectPath, containerName string) error {
	template, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		return err
	}
	projectName := filepath.Base(projectPath)
	subst := strings.Replace(string(template), "# {{PROFILE_PKGS}}\n", packagesBlock(bp), 1)
	subst = strings.Replace(subst, "# {{PROFILE_INSTALLERS}}\n", installersBlock(bp), 1)
	subst = strings.Replace(subst, "# {{PROFILE_CONFIGS}}\n", configsBlock(bp), 1)
	subst = strings.Replace(subst, "# {{PROFILE_DIRS}}\n", dirsBlock(bp, projectName), 1)

	sdDir := filepath.Join(projectPath, ".morgul", containerName)
	if err := os.MkdirAll(sdDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sdDir, "Dockerfile.sd"), []byte(subst), 0644)
}

func packagesBlock(bp *config.Blueprint) string {
	if len(bp.Packages) == 0 {
		return ""
	}
	b := strings.Builder{}
	b.WriteString("RUN sudo apt update && sudo apt install -y --no-install-recommends ")
	for _, pkg := range bp.Packages {
		fmt.Fprintf(&b, "%s ", pkg)
	}
	b.WriteString("&& sudo rm -rf /var/lib/apt/lists/*\n")
	return b.String()
}

func installersBlock(bp *config.Blueprint) string {
	b := strings.Builder{}
	for _, inst := range bp.Installers {
		fmt.Fprintf(&b, "RUN %s\n", inst)
	}
	return b.String()
}

func dirsBlock(bp *config.Blueprint, projectName string) string {
	var dirs []string
	if bp.ProjectMount == nil || *bp.ProjectMount {
		dirs = append(dirs, fmt.Sprintf("/home/dev/%s", projectName))
	}
	for _, v := range bp.VirtualVolumes {
		dirs = append(dirs, v.Path)
	}
	if len(dirs) == 0 {
		return ""
	}
	b := strings.Builder{}
	b.WriteString("RUN mkdir -p")
	for _, d := range dirs {
		fmt.Fprintf(&b, " %s", d)
	}
	b.WriteString(" && chown -R $UID:$GID /home/$USERNAME")
	for _, d := range dirs {
		if !strings.HasPrefix(d, "/home/") {
			fmt.Fprintf(&b, " && chown $UID:$GID %s", d)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func configsBlock(bp *config.Blueprint) string {
	b := strings.Builder{}
	for _, cf := range bp.Configs {
		// NOTE: these paths are staged into .morgul/<container>/configs/
		//       before the build, so COPY resolves them relative to the context.
		src := strings.Replace(cf.SourcePattern, "~", "configs", 1)
		dst := strings.Replace(cf.DestinationPath, "~", "/home/dev", 1)
		fmt.Fprintf(&b, "COPY --chown=$UID:$GID %s %s\n", src, dst)
	}
	return b.String()
}
