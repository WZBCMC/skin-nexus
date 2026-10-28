package plugin

import "context"

// EmitEvent queues a generic event for plugin subscribers. It never blocks the
// request path or returns an error: the bounded queue drops when saturated.
func (m *Manager) EmitEvent(ctx context.Context, kind string, p EventPayload) {
	if !knownEvents[kind] {
		return
	}
	p.Kind = kind
	m.emit(eventGeneric, p)
}

// CheckGuard runs synchronous guards in plugin-name order. A deny returns
// GuardError. A failing fail-closed guard also denies; a fail-open guard is
// skipped. The super-admin break-glass option disables all guards.
func (m *Manager) CheckGuard(ctx context.Context, kind string, p EventPayload) error {
	if !knownGuards[kind] {
		return nil
	}
	if m.guardsBypassed(ctx) {
		return nil
	}
	p.Kind = kind
	type target struct {
		plugin     string
		fn         guardFn
		failClosed bool
	}
	var targets []target
	m.mu.RLock()
	for _, name := range m.order {
		inst := m.instances[name]
		if !inst.enabled || inst.rt == nil {
			continue
		}
		for _, g := range inst.rt.hooks.guards[kind] {
			targets = append(targets, target{plugin: name, fn: g.fn, failClosed: g.failClosed})
		}
	}
	m.mu.RUnlock()
	for _, t := range targets {
		d, err := t.fn(ctx, p)
		if err != nil {
			if t.failClosed {
				return &GuardError{Plugin: t.plugin, Message: "guard unavailable", Code: "plugin.guard_unavailable"}
			}
			continue
		}
		if !d.Allow {
			msg := d.Message
			if msg == "" {
				msg = "blocked by plugin guard"
			}
			code := d.Code
			if code == "" {
				code = "plugin.guard_denied"
			}
			return &GuardError{Plugin: t.plugin, Message: msg, Code: code}
		}
	}
	return nil
}

// GuardsBypassed reports the super-admin break-glass switch. The option is
// written only through the audited admin endpoint.
func (m *Manager) GuardsBypassed(ctx context.Context) bool { return m.guardsBypassed(ctx) }

func (m *Manager) guardsBypassed(ctx context.Context) bool {
	st := m.platformStore()
	if st == nil {
		return false
	}
	v, ok, err := st.GetOption(ctx, "plugin_guards_bypass")
	return err == nil && ok && v == "1"
}

// SetGuardsBypassed flips the break-glass switch and audits the change.
func (m *Manager) SetGuardsBypassed(ctx context.Context, on bool) error {
	st := m.platformStore()
	if st == nil {
		return ErrPluginNoStore
	}
	v := "0"
	if on {
		v = "1"
	}
	if err := st.SetOption(ctx, "plugin_guards_bypass", v); err != nil {
		return err
	}
	m.audit(ctx, "plugin_guards_bypass", map[string]any{"enabled": on})
	return nil
}
