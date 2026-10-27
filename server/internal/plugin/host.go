package plugin

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// StateStore is the minimal options K/V surface the host needs to persist
// enable/disable state. *store.Mysql satisfies it.
type StateStore interface {
	GetOption(ctx context.Context, k string) (string, bool, error)
	SetOption(ctx context.Context, k, v string) error
}

// RuntimeConfig tunes the host. Zero durations fall back to safe defaults.
type RuntimeConfig struct {
	Dir         string
	HostVersion string
	State       StateStore
	Audit       func(ctx context.Context, action string, detail map[string]any)

	HookTimeout  time.Duration // default 100ms
	AuthTimeout  time.Duration // default 50ms
	RouteTimeout time.Duration // default 1s
}

func (c *RuntimeConfig) withDefaults() {
	if c.HookTimeout <= 0 {
		c.HookTimeout = 100 * time.Millisecond
	}
	if c.AuthTimeout <= 0 {
		c.AuthTimeout = 50 * time.Millisecond
	}
	if c.RouteTimeout <= 0 {
		c.RouteTimeout = time.Second
	}
}

func (c *RuntimeConfig) timeouts() timeouts {
	return timeouts{hook: c.HookTimeout, auth: c.AuthTimeout, route: c.RouteTimeout}
}

// goRoute is a host-level (Go) plugin route registered via Manager.Bus.
type goRoute struct {
	method  string
	path    string
	handler func(ctx context.Context, req map[string]interface{}) (int, interface{})
	timeout time.Duration
}

type instance struct {
	info     Info
	rt       *runtime
	enabled  bool
	state    string // loaded | disabled | error
	lastErr  string
	loadedAt time.Time
}

// Manager scans a plugin directory, owns per-plugin runtimes and dispatches
// hooks. It implements the frozen Host interface.
type Manager struct {
	cfg RuntimeConfig

	mu        sync.RWMutex
	instances map[string]*instance
	order     []string
	extra     hookSet
	extraRoutes []goRoute

	events    chan eventItem
	done      chan struct{}
	closeOnce sync.Once
}

type eventItem struct {
	kind    int // eventUpload | eventLifecycle | eventAuth
	payload any
}

// event kinds routed through the bounded queue.
const (
	eventUpload = iota
	eventLifecycle
	eventAuth
)

// ErrPluginNotFound is returned by Enable/Disable/Reload for unknown names.
var ErrPluginNotFound = errors.New("plugin: not found")

// NewHost builds a Manager and starts its event worker.
func NewHost(cfg RuntimeConfig) *Manager {
	cfg.withDefaults()
	m := &Manager{
		cfg:       cfg,
		instances: map[string]*instance{},
		events:    make(chan eventItem, 256),
		done:      make(chan struct{}),
	}
	go m.eventWorker()
	return m
}

// Bus exposes host-level hook registration (tests / first-party integrations).
func (m *Manager) Bus() HookBus { return goBus{m} }

func (m *Manager) logSink(line string) { log.Print(line) }

func (m *Manager) audit(ctx context.Context, action string, detail map[string]any) {
	if m.cfg.Audit != nil {
		m.cfg.Audit(ctx, action, detail)
	}
}

func optionKey(name string) string { return "plugin_enabled:" + name }

func (m *Manager) enabledOption(ctx context.Context, name string) bool {
	if m.cfg.State == nil {
		return false
	}
	v, ok, err := m.cfg.State.GetOption(ctx, optionKey(name))
	return err == nil && ok && v == "1"
}

func (m *Manager) setEnabledOption(ctx context.Context, name string, on bool) error {
	if m.cfg.State == nil {
		return nil
	}
	v := "0"
	if on {
		v = "1"
	}
	return m.cfg.State.SetOption(ctx, optionKey(name), v)
}

// Load scans the plugin root. Invalid manifests are recorded as errored
// entries; only previously-enabled plugins start. A missing root is not an
// error so fresh deployments boot cleanly.
func (m *Manager) Load(dir string) error {
	m.cfg.Dir = dir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	ctx := context.Background()
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if count >= maxPlugins {
			log.Printf("plugin: ignoring %s (max %d plugins)", e.Name(), maxPlugins)
			continue
		}
		count++
		m.loadLocked(ctx, e.Name())
	}
	sort.Strings(m.order)
	return nil
}

func (m *Manager) loadLocked(ctx context.Context, name string) {
	dir := filepath.Join(m.cfg.Dir, name)
	inst := &instance{state: "disabled"}
	info, err := loadManifest(dir, name)
	if err != nil {
		inst.info = Info{Manifest: Manifest{Name: name}}
		inst.state = "error"
		inst.lastErr = err.Error()
		m.instances[name] = inst
		m.order = append(m.order, name)
		m.audit(ctx, "plugin_load_failed", map[string]any{"plugin": name, "error": err.Error()})
		return
	}
	inst.info = info
	if m.enabledOption(ctx, name) {
		if rt, rerr := newRuntime(info, m.cfg.timeouts(), m.logSink); rerr != nil {
			inst.state = "error"
			inst.lastErr = rerr.Error()
			m.audit(ctx, "plugin_load_failed", map[string]any{"plugin": name, "version": info.Version, "error": rerr.Error()})
		} else {
			inst.rt = rt
			inst.enabled = true
			inst.state = "loaded"
			inst.loadedAt = time.Now()
			m.audit(ctx, "plugin_loaded", map[string]any{"plugin": name, "version": info.Version, "permissions": info.Permissions})
		}
	}
	m.instances[name] = inst
	m.order = append(m.order, name)
}

// Enable validates, persists and starts a plugin.
func (m *Manager) Enable(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[name]
	if !ok {
		return ErrPluginNotFound
	}
	if inst.enabled {
		return nil
	}
	info, err := loadManifest(filepath.Join(m.cfg.Dir, name), name)
	if err != nil {
		inst.state = "error"
		inst.lastErr = err.Error()
		return err
	}
	inst.info = info
	if err := m.setEnabledOption(ctx, name, true); err != nil {
		return err
	}
	rt, rerr := newRuntime(info, m.cfg.timeouts(), m.logSink)
	if rerr != nil {
		_ = m.setEnabledOption(ctx, name, false)
		inst.state = "error"
		inst.lastErr = rerr.Error()
		m.audit(ctx, "plugin_load_failed", map[string]any{"plugin": name, "version": info.Version, "error": rerr.Error()})
		return rerr
	}
	inst.rt = rt
	inst.enabled = true
	inst.state = "loaded"
	inst.lastErr = ""
	inst.loadedAt = time.Now()
	m.audit(ctx, "plugin_enable", map[string]any{"plugin": name, "version": info.Version, "permissions": info.Permissions})
	return nil
}

// Disable removes a plugin from dispatch and closes its VM.
func (m *Manager) Disable(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[name]
	if !ok {
		return ErrPluginNotFound
	}
	if !inst.enabled {
		return nil
	}
	if err := m.setEnabledOption(ctx, name, false); err != nil {
		return err
	}
	if inst.rt != nil {
		inst.rt.close()
	}
	inst.rt = nil
	inst.enabled = false
	inst.state = "disabled"
	m.audit(ctx, "plugin_disable", map[string]any{"plugin": name, "version": inst.info.Version})
	return nil
}

// Reload re-reads a plugin's manifest and restarts its VM in place.
func (m *Manager) Reload(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[name]
	if !ok {
		return ErrPluginNotFound
	}
	wasEnabled := inst.enabled
	if inst.rt != nil {
		inst.rt.close()
		inst.rt = nil
		inst.enabled = false
	}
	info, err := loadManifest(filepath.Join(m.cfg.Dir, name), name)
	if err != nil {
		inst.state = "error"
		inst.lastErr = err.Error()
		m.audit(ctx, "plugin_load_failed", map[string]any{"plugin": name, "error": err.Error()})
		return err
	}
	inst.info = info
	if wasEnabled {
		rt, rerr := newRuntime(info, m.cfg.timeouts(), m.logSink)
		if rerr != nil {
			inst.state = "error"
			inst.lastErr = rerr.Error()
			m.audit(ctx, "plugin_load_failed", map[string]any{"plugin": name, "version": info.Version, "error": rerr.Error()})
			return rerr
		}
		inst.rt = rt
		inst.enabled = true
		inst.state = "loaded"
		inst.loadedAt = time.Now()
	} else {
		inst.state = "disabled"
	}
	inst.lastErr = ""
	m.audit(ctx, "plugin_reload", map[string]any{"plugin": name, "version": info.Version})
	return nil
}

// Close stops the event worker and releases every VM.
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		close(m.done)
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, inst := range m.instances {
			if inst.rt != nil {
				inst.rt.close()
				inst.rt = nil
			}
		}
	})
	return nil
}

// eventWorker drains the bounded event queue off the request path.
func (m *Manager) eventWorker() {
	for {
		select {
		case <-m.done:
			return
		case item := <-m.events:
			m.dispatchEvent(item)
		}
	}
}

func (m *Manager) emit(kind int, payload any) {
	select {
	case m.events <- eventItem{kind, payload}:
	default:
		// queue saturated: drop rather than block the request path
	}
}

func (m *Manager) dispatchEvent(item eventItem) {
	ctx := context.Background()
	m.mu.RLock()
	switch item.kind {
	case eventUpload:
		e := item.payload.(TextureUploadedEvent)
		for _, h := range m.extra.upload {
			h := h
			runGuarded(ctx, h.timeout, func(context.Context) error { return h.fn(ctx, e) })
		}
	case eventLifecycle:
		e := item.payload.(UserLifecycleEvent)
		for _, h := range m.extra.lifecycle {
			h := h
			runGuarded(ctx, h.timeout, func(context.Context) error { return h.fn(ctx, e) })
		}
	case eventAuth:
		e := item.payload.(AuthPhaseEvent)
		for _, h := range m.extra.auth {
			h := h
			runGuarded(ctx, h.timeout, func(context.Context) error { return h.fn(ctx, e) })
		}
	}

	type target struct {
		fn func(context.Context) error
	}
	var targets []target
	for _, name := range m.order {
		inst := m.instances[name]
		if !inst.enabled || inst.rt == nil {
			continue
		}
		rt := inst.rt
		switch item.kind {
		case eventUpload:
			e := item.payload.(TextureUploadedEvent)
			for _, h := range rt.hooks.upload {
				h := h
				targets = append(targets, target{func(context.Context) error { return h.fn(ctx, e) }})
			}
		case eventLifecycle:
			e := item.payload.(UserLifecycleEvent)
			for _, h := range rt.hooks.lifecycle {
				h := h
				targets = append(targets, target{func(context.Context) error { return h.fn(ctx, e) }})
			}
		case eventAuth:
			e := item.payload.(AuthPhaseEvent)
			for _, h := range rt.hooks.auth {
				h := h
				targets = append(targets, target{func(context.Context) error { return h.fn(ctx, e) }})
			}
		}
	}
	m.mu.RUnlock()

	for _, t := range targets {
		_ = t.fn(ctx) // JS hooks self-timeout inside the VM
	}
}

// runGuarded bounds a host-level (Go) hook so a misbehaving callback cannot
// stall the dispatcher.
func runGuarded(ctx context.Context, d time.Duration, fn func(context.Context) error) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = fn(ctx)
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// ProfileFilter chains host-level then plugin profile filters (name order).
// Failures are fail-open: the payload simply keeps its prior value.
func (m *Manager) ProfileFilter(ctx context.Context, p *UserProfileFilter) {
	m.mu.RLock()
	extra := append([]timed[profileHookFn](nil), m.extra.profile...)
	var pluginFns []profileHookFn
	for _, name := range m.order {
		inst := m.instances[name]
		if inst.enabled && inst.rt != nil {
			for _, h := range inst.rt.hooks.profile {
				pluginFns = append(pluginFns, h.fn)
			}
		}
	}
	m.mu.RUnlock()

	for _, h := range extra {
		runGuarded(ctx, h.timeout, func(context.Context) error { return h.fn(ctx, p) })
	}
	for _, fn := range pluginFns {
		_ = fn(ctx, p)
	}
}

// TextureURL applies URL filters and returns the (possibly rewritten) URL.
func (m *Manager) TextureURL(ctx context.Context, hash, base, typ string) string {
	f := TextureURLFilter{Hash: hash, URL: base, Type: typ}
	m.mu.RLock()
	extra := append([]timed[textureHookFn](nil), m.extra.texture...)
	var pluginFns []textureHookFn
	for _, name := range m.order {
		inst := m.instances[name]
		if inst.enabled && inst.rt != nil {
			for _, h := range inst.rt.hooks.texture {
				pluginFns = append(pluginFns, h.fn)
			}
		}
	}
	m.mu.RUnlock()

	for _, h := range extra {
		runGuarded(ctx, h.timeout, func(context.Context) error { return h.fn(ctx, &f) })
	}
	for _, fn := range pluginFns {
		_ = fn(ctx, &f)
	}
	return f.URL
}

// Emit* enqueue fire-and-forget events; they never block the request path.
func (m *Manager) EmitTextureUploaded(_ context.Context, e TextureUploadedEvent) {
	m.emit(eventUpload, e)
}

func (m *Manager) EmitUserLifecycle(_ context.Context, e UserLifecycleEvent) {
	m.emit(eventLifecycle, e)
}

func (m *Manager) EmitAuthPhase(_ context.Context, e AuthPhaseEvent) {
	m.emit(eventAuth, e)
}
