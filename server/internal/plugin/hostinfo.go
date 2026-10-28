package plugin

// HostPluginInfo is the safe, plugin-visible view of another installed plugin.
type HostPluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Title   string `json:"title,omitempty"`
	Enabled bool   `json:"enabled"`
}

// HostInfo is the read-only capability snapshot exposed to plugins through
// skin.host.info() and to the SPA through /api/v1/plugins/ui. It never carries
// config values, secrets, or plugin error strings.
type HostInfo struct {
	Version      string           `json:"version"`
	HookAPI      int              `json:"hookApi"`
	Capabilities []string         `json:"capabilities"`
	Plugins      []HostPluginInfo `json:"plugins"`
	Slots        []string         `json:"slots"`
}

// Capabilities returns a copy of the additive host capability list.
func Capabilities() []string {
	return append([]string(nil), hostCapabilities...)
}

// buildHostInfoLocked snapshots the manager. Callers must hold m.mu.
func (m *Manager) buildHostInfoLocked() HostInfo {
	info := HostInfo{
		Version:      m.cfg.HostVersion,
		HookAPI:      HookAPIVersion,
		Capabilities: Capabilities(),
		Plugins:      make([]HostPluginInfo, 0, len(m.order)),
		Slots:        append([]string(nil), fixedSlotOrder...),
	}
	for _, name := range m.order {
		inst := m.instances[name]
		if inst == nil {
			continue
		}
		info.Plugins = append(info.Plugins, HostPluginInfo{
			Name:    name,
			Version: inst.info.Version,
			Title:   inst.info.Title,
			Enabled: inst.enabled,
		})
	}
	return info
}

// refreshHostInfoLocked republishes the snapshot. Callers must hold m.mu.
func (m *Manager) refreshHostInfoLocked() {
	info := m.buildHostInfoLocked()
	m.hostInfo.Store(&info)
}

// HostInfo returns the current snapshot. A manager that never refreshed (for
// example a test that only constructs it) falls back to a live build.
func (m *Manager) HostInfo() HostInfo {
	if p := m.hostInfo.Load(); p != nil {
		return *p
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buildHostInfoLocked()
}
