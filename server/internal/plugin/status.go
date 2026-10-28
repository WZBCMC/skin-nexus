package plugin

import "time"

// Status is the admin-facing snapshot of one scanned plugin directory.
type Status struct {
	Name                string         `json:"name"`
	Version             string         `json:"version"`
	Title               string         `json:"title,omitempty"`
	Author              string         `json:"author,omitempty"`
	Homepage            string         `json:"homepage,omitempty"`
	Description         string         `json:"description,omitempty"`
	HookAPI             int            `json:"hookApi"`
	Permissions         []string       `json:"permissions"`
	Enabled             bool           `json:"enabled"`
	State               string         `json:"state"` // loaded | disabled | error
	Health              string         `json:"health,omitempty"`
	Runtime             string         `json:"runtime,omitempty"`
	LastError           string         `json:"lastError,omitempty"`
	Hooks               map[string]int `json:"hooks"`
	Routes              int            `json:"routes"`
	UIPages             int            `json:"uiPages,omitempty"`
	ConfigSchemaVersion int            `json:"configSchemaVersion,omitempty"`
	Source              string         `json:"source,omitempty"`
	Trust               string         `json:"trust,omitempty"`
	Signer              string         `json:"signer,omitempty"`
	Digest              string         `json:"digest,omitempty"`
	LoadedAt            time.Time      `json:"loadedAt,omitempty"`
	InstalledAt         time.Time      `json:"installedAt,omitempty"`
	UpdatedAt           time.Time      `json:"updatedAt,omitempty"`
	Calls               int64          `json:"calls,omitempty"`
	Errors              int64          `json:"errors,omitempty"`
	Timeouts            int64          `json:"timeouts,omitempty"`
	RouteCalls          int64          `json:"routeCalls,omitempty"`
	Logs                []string       `json:"logs,omitempty"`
}

func uiPageCount(m Manifest) int {
	if m.UI == nil {
		return 0
	}
	return len(m.UI.Pages)
}

func healthForState(state string) string {
	switch state {
	case "loaded":
		return "ok"
	case "error":
		return "error"
	default:
		return "disabled"
	}
}

// List returns one Status per scanned plugin directory, in name order.
func (m *Manager) List() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Status, 0, len(m.order))
	for _, name := range m.order {
		inst := m.instances[name]
		st := Status{
			Name:                name,
			Version:             inst.info.Version,
			Title:               inst.info.Title,
			Author:              inst.info.Author,
			Homepage:            inst.info.Homepage,
			Description:         inst.info.Description,
			HookAPI:             inst.info.HookAPI,
			Permissions:         inst.info.Permissions,
			Enabled:             inst.enabled,
			State:               inst.state,
			Health:              inst.health,
			Runtime:             inst.info.Runtime,
			LastError:           inst.lastErr,
			Hooks:               map[string]int{},
			UIPages:             uiPageCount(inst.info.Manifest),
			ConfigSchemaVersion: inst.info.configSchemaVersion(),
			Source:              inst.source,
			Trust:               inst.trust,
			Signer:              inst.signer,
			Digest:              inst.checksum,
			LoadedAt:            inst.loadedAt,
			InstalledAt:         inst.installedAt,
			UpdatedAt:           inst.updatedAt,
		}
		if st.Runtime == "" {
			st.Runtime = "goja"
		}
		if st.Health == "" {
			st.Health = healthForState(inst.state)
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
			st.Calls = inst.rt.calls
			st.Errors = inst.rt.errors
			st.Timeouts = inst.rt.timeouts
			st.RouteCalls = inst.rt.routeCalls
			st.Logs = append([]string(nil), inst.rt.logs...)
			if inst.rt.lastErr != "" {
				st.LastError = inst.rt.lastErr
			}
			inst.rt.mu.Unlock()
		}
		out = append(out, st)
	}
	return out
}
