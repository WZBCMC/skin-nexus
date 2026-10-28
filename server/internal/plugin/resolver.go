package plugin

import "fmt"

// PlanInput is enough metadata to decide whether a bundle may be installed.
// It deliberately mirrors the manifest fields the market registry exposes.
type PlanInput struct {
	Name        string
	Version     string
	Permissions []string
	Requires    *RequiresSpec
	Conflicts   []string
}

type InstallPlan struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	CanInstall         bool     `json:"canInstall"`
	Missing            []string `json:"missing,omitempty"`
	Conflicts          []string `json:"conflicts,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
	AddedPermissions   []string `json:"addedPermissions,omitempty"`
	RemovedPermissions []string `json:"removedPermissions,omitempty"`
}

// PlanInstall checks forward dependencies/conflicts against scanned plugins
// (including disabled ones), the host version, and the plugin-count cap.
// Reverse dependencies are intentionally out of P2 scope: no external
// ecosystem exists yet, and refusing a reverse break is a resolver upgrade.
func (m *Manager) PlanInstall(in PlanInput) (*InstallPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := &InstallPlan{Name: in.Name, Version: in.Version, CanInstall: true}
	existing := map[string]*instance{}
	for name, inst := range m.instances {
		existing[name] = inst
	}
	var current *instance
	if inst, ok := existing[in.Name]; ok {
		current = inst
	}
	if current == nil && len(m.instances) >= maxPlugins {
		p.CanInstall = false
		p.Warnings = append(p.Warnings, fmt.Sprintf("plugin limit reached (%d)", maxPlugins))
	}
	if in.Requires != nil && in.Requires.Server != "" && m.cfg.HostVersion != "" {
		ok, err := checkMinServer(in.Requires.Server, m.cfg.HostVersion)
		if err != nil || !ok {
			p.CanInstall = false
			p.Missing = append(p.Missing, fmt.Sprintf("host %s does not satisfy %s", m.cfg.HostVersion, in.Requires.Server))
		}
	}
	if in.Requires != nil {
		for name, constraint := range in.Requires.Plugins {
			if name == in.Name {
				p.CanInstall = false
				p.Missing = append(p.Missing, "plugin depends on itself: "+name)
				continue
			}
			dep, ok := existing[name]
			if !ok {
				p.CanInstall = false
				p.Missing = append(p.Missing, fmt.Sprintf("%s %s (not installed)", name, constraint))
				continue
			}
			okVersion, err := satisfiesVersion(dep.info.Version, constraint)
			if err != nil || !okVersion {
				p.CanInstall = false
				p.Missing = append(p.Missing, fmt.Sprintf("%s %s (installed %s)", name, constraint, dep.info.Version))
				continue
			}
			if !dep.enabled {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s is installed but disabled", name))
			}
		}
	}
	for _, name := range in.Conflicts {
		dep, ok := existing[name]
		if !ok || dep.info.Version == "" {
			continue
		}
		// A bare name conflicts with every installed version.
		p.CanInstall = false
		p.Conflicts = append(p.Conflicts, fmt.Sprintf("%s (installed %s)", name, dep.info.Version))
	}
	oldPerms := map[string]bool{}
	if current != nil {
		for _, x := range current.info.Permissions {
			oldPerms[x] = true
		}
	}
	newPerms := map[string]bool{}
	for _, x := range in.Permissions {
		newPerms[x] = true
		if !oldPerms[x] {
			p.AddedPermissions = append(p.AddedPermissions, x)
		}
	}
	for x := range oldPerms {
		if !newPerms[x] {
			p.RemovedPermissions = append(p.RemovedPermissions, x)
		}
	}
	if !p.CanInstall {
		return p, nil
	}
	return p, nil
}
