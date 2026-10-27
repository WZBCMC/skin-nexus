package plugin

import "time"

// Status is the admin-facing snapshot of one scanned plugin directory.
type Status struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Title       string   `json:"title,omitempty"`
	Author      string   `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Description string   `json:"description,omitempty"`
	HookAPI     int      `json:"hookApi"`
	Permissions []string `json:"permissions"`
	Enabled     bool     `json:"enabled"`
	State       string   `json:"state"` // loaded | disabled | error
	LastError   string   `json:"lastError,omitempty"`
	Hooks       map[string]int `json:"hooks"`
	Routes      int      `json:"routes"`
	LoadedAt    time.Time `json:"loadedAt,omitempty"`
}

// List returns one Status per scanned plugin directory, in name order.
func (m *Manager) List() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Status, 0, len(m.order))
	for _, name := range m.order {
		inst := m.instances[name]
		st := Status{
			Name:        name,
			Version:     inst.info.Version,
			Title:       inst.info.Title,
			Author:      inst.info.Author,
			Homepage:    inst.info.Homepage,
			Description: inst.info.Description,
			HookAPI:     inst.info.HookAPI,
			Permissions: inst.info.Permissions,
			Enabled:     inst.enabled,
			State:       inst.state,
			LastError:   inst.lastErr,
			Hooks:       map[string]int{},
			LoadedAt:    inst.loadedAt,
		}
		if inst.rt != nil {
			inst.rt.mu.Lock()
			st.Hooks = map[string]int{
				"profile":   len(inst.rt.hooks.profile),
				"texture":   len(inst.rt.hooks.texture),
				"upload":    len(inst.rt.hooks.upload),
				"lifecycle": len(inst.rt.hooks.lifecycle),
				"auth":      len(inst.rt.hooks.auth),
			}
			st.Routes = len(inst.rt.routes)
			if inst.rt.lastErr != "" {
				st.LastError = inst.rt.lastErr
			}
			inst.rt.mu.Unlock()
		}
		out = append(out, st)
	}
	return out
}
