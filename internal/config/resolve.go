package config

import "fmt"

// Resolver flattens presets or ad-hoc module lists into a blueprint.
type Resolver struct {
	Store *Store
}

func (r *Resolver) ResolvePreset(id string) (*Blueprint, error) {
	preset, err := r.Store.LoadPreset(id)
	if err != nil {
		return nil, err
	}
	bp, err := r.resolveModules(preset.Modules)
	if err != nil {
		return nil, err
	}
	bp.Name = preset.ID
	bp.Shell = preset.Shell
	if bp.Shell == "" {
		bp.Shell = DefaultShell
	}
	bp.ProjectMount = preset.ProjectMount
	return bp, nil
}

func (r *Resolver) ResolveCustom(moduleIDs []string, shell string, projectMount *bool) (*Blueprint, error) {
	bp, err := r.resolveModules(moduleIDs)
	if err != nil {
		return nil, err
	}
	bp.Name = "custom"
	bp.Shell = shell
	if bp.Shell == "" {
		bp.Shell = DefaultShell
	}
	bp.ProjectMount = projectMount
	return bp, nil
}

func (r *Resolver) resolveModules(ids []string) (*Blueprint, error) {
	bp := &Blueprint{}
	for _, id := range ids {
		m, err := r.Store.LoadModule(id)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", id, err)
		}
		bp.Modules = append(bp.Modules, m.ID)
		bp.Packages = append(bp.Packages, m.Packages...)
		bp.Installers = append(bp.Installers, m.Installers...)
		bp.Configs = append(bp.Configs, m.Configs...)
		bp.Ports = append(bp.Ports, m.Ports...)
		bp.VirtualVolumes = append(bp.VirtualVolumes, m.VirtualVolumes...)
		bp.EnvFiles = append(bp.EnvFiles, m.EnvFiles...)
	}
	return bp, nil
}
