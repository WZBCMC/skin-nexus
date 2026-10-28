package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skin-nexus/server/internal/signedpkg"
)

var (
	ErrInstallBlocked  = errors.New("plugin install blocked by plan")
	ErrConsentRequired = errors.New("plugin install requires capability consent")
	ErrNoRollback      = errors.New("no rollback version available")
)

type DataPurger interface {
	DeletePluginData(ctx context.Context, plugin string) error
}

type InstallResult struct {
	Plan   *InstallPlan `json:"plan"`
	State  string       `json:"state"`
	Source string       `json:"source,omitempty"`
}

func (m *Manager) stateDir() string {
	if m.cfg.StateDir != "" {
		return m.cfg.StateDir
	}
	return filepath.Join(m.cfg.Dir, ".state")
}

func (m *Manager) ensureInstallDirs() error {
	for _, d := range []string{"staging", "rollback", "quarantine", "cache"} {
		if err := os.MkdirAll(filepath.Join(m.stateDir(), d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func txID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// InstallBytes writes bytes to a temporary file and installs it. Market
// downloads already verified the transport SHA-256; the bundle signature is
// verified again here.
func (m *Manager) InstallBytes(ctx context.Context, data []byte, consent bool, source string) (*InstallResult, error) {
	if err := m.ensureInstallDirs(); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Join(m.stateDir(), "staging"), "bundle-*.zip")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return m.InstallBundle(ctx, path, consent, source)
}

// InstallBundle verifies a signed bundle, stages it, and atomically swaps it
// into the live plugin directory. New installs stay disabled; upgrades keep
// their previous enabled state.
func (m *Manager) InstallBundle(ctx context.Context, zipPath string, consent bool, source string) (result *InstallResult, retErr error) {
	if m.cfg.Keyring == nil {
		return nil, errors.New("plugin trust keys are not configured")
	}
	if err := m.ensureInstallDirs(); err != nil {
		return nil, err
	}
	meta, err := signedpkg.ReadBundleMeta(zipPath)
	if err != nil {
		return nil, err
	}
	txid := txID()
	if txStore := m.installTxStore(); txStore != nil {
		kind, from := "install", ""
		if cur := m.statusOf(meta.Name); cur != nil {
			kind, from = "upgrade", cur.Version
		}
		_ = txStore.CreateInstallTx(ctx, InstallTxRecord{
			ID: txid, Plugin: meta.Name, Kind: kind, FromVersion: from, ToVersion: meta.Version,
			Source: source, State: "verifying", ActorUID: ActorFromContext(ctx),
		})
		defer func() {
			state := "failed"
			if result != nil && result.State != "" {
				state = result.State
			}
			msg := ""
			if retErr != nil {
				msg = retErr.Error()
			}
			_ = txStore.UpdateInstallTx(context.WithoutCancel(ctx), txid, state, msg)
		}()
	}
	if _, err := signedpkg.VerifyBundle(zipPath, m.cfg.Keyring); err != nil {
		return nil, err
	}
	tx := txID()
	stage := filepath.Join(m.stateDir(), "staging", tx)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := extractZip(zipPath, stage); err != nil {
		return nil, err
	}
	root, err := findPluginRoot(stage)
	if err != nil {
		return nil, err
	}
	info, err := loadManifest(root, meta.Name)
	if err != nil {
		return nil, err
	}
	if info.Version != meta.Version {
		return nil, fmt.Errorf("bundle version %s != manifest version %s", meta.Version, info.Version)
	}
	if m.cfg.HostVersion != "" {
		if err := info.CheckMinServer(m.cfg.HostVersion); err != nil {
			return &InstallResult{Plan: &InstallPlan{Name: meta.Name, Version: meta.Version, CanInstall: false, Missing: []string{err.Error()}}, State: "blocked", Source: source}, ErrInstallBlocked
		}
	}
	plan, err := m.PlanInstall(PlanInput{
		Name: info.Name, Version: info.Version, Permissions: info.Permissions,
		Requires: info.Requires, Conflicts: info.Conflicts,
	})
	if err != nil {
		return nil, err
	}
	if !plan.CanInstall {
		return &InstallResult{Plan: plan, State: "blocked", Source: source}, ErrInstallBlocked
	}
	if len(plan.AddedPermissions) > 0 && !consent {
		return &InstallResult{Plan: plan, State: "consent", Source: source}, ErrConsentRequired
	}
	wasEnabled := m.enabledOption(ctx, info.Name)
	m.stopRuntime(info.Name)
	target := filepath.Join(m.cfg.Dir, info.Name)
	backup := filepath.Join(m.stateDir(), "rollback", info.Name)
	if _, err := os.Stat(target); err == nil {
		_ = os.RemoveAll(backup)
		if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(target, backup); err != nil {
			return nil, fmt.Errorf("move old plugin aside: %w", err)
		}
	}
	if err := os.Rename(root, target); err != nil {
		// Best-effort restore of the previous version.
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, target)
		}
		return nil, fmt.Errorf("activate plugin: %w", err)
	}
	if err := m.Load(m.cfg.Dir); err != nil {
		_ = m.restoreRollback(info.Name, target, backup)
		return nil, err
	}
	st := m.statusOf(info.Name)
	if st == nil || st.State == "error" || (wasEnabled && st.State != "loaded") {
		_ = m.restoreRollback(info.Name, target, backup)
		if st != nil {
			return nil, fmt.Errorf("plugin activation failed: %s", st.LastError)
		}
		return nil, errors.New("plugin activation failed")
	}
	action := "plugin_install"
	if wasEnabled {
		action = "plugin_upgrade"
	}
	m.audit(ctx, action, map[string]any{"plugin": info.Name, "version": info.Version, "source": source})
	return &InstallResult{Plan: plan, State: st.State, Source: source}, nil
}

func (m *Manager) statusOf(name string) *Status {
	for _, st := range m.List() {
		if st.Name == name {
			return &st
		}
	}
	return nil
}

func (m *Manager) restoreRollback(name, target, backup string) error {
	_ = os.RemoveAll(target)
	if _, err := os.Stat(backup); err == nil {
		_ = os.Rename(backup, target)
	}
	return m.Load(m.cfg.Dir)
}

// Rollback restores the previous version saved by the last upgrade.
func (m *Manager) Rollback(ctx context.Context, name string) error {
	backup := filepath.Join(m.stateDir(), "rollback", name)
	if _, err := os.Stat(backup); err != nil {
		return ErrNoRollback
	}
	target := filepath.Join(m.cfg.Dir, name)
	m.stopRuntime(name)
	trash := filepath.Join(m.stateDir(), "quarantine", "trash", txID())
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, trash); err != nil {
			return err
		}
	}
	if err := os.Rename(backup, target); err != nil {
		_ = os.Rename(trash, target)
		return err
	}
	if err := m.Load(m.cfg.Dir); err != nil {
		return err
	}
	m.audit(ctx, "plugin_rollback", map[string]any{"plugin": name})
	return nil
}

// Uninstall removes the live plugin directory. Data is kept unless purge.
func (m *Manager) Uninstall(ctx context.Context, name string, purge bool) error {
	m.mu.RLock()
	_, ok := m.instances[name]
	m.mu.RUnlock()
	if !ok {
		return ErrPluginNotFound
	}
	_ = m.Disable(ctx, name)
	target := filepath.Join(m.cfg.Dir, name)
	trash := filepath.Join(m.stateDir(), "quarantine", "trash", txID())
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, trash); err != nil {
			return err
		}
	}
	m.removeInstance(name)
	if purge {
		if p, ok := m.platformStore().(DataPurger); ok {
			if err := p.DeletePluginData(ctx, name); err != nil {
				return err
			}
		}
	}
	m.audit(ctx, "plugin_uninstall", map[string]any{"plugin": name, "purge": purge})
	return nil
}

func (m *Manager) stopRuntime(name string) {
	m.mu.Lock()
	inst, ok := m.instances[name]
	if !ok {
		m.mu.Unlock()
		return
	}
	rt := inst.rt
	inst.rt = nil
	inst.enabled = false
	inst.state = "disabled"
	inst.health = "disabled"
	m.mu.Unlock()
	if rt != nil {
		rt.close()
	}
}

func (m *Manager) removeInstance(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst, ok := m.instances[name]; ok {
		if inst.rt != nil {
			inst.rt.close()
		}
		delete(m.instances, name)
	}
	out := m.order[:0]
	for _, n := range m.order {
		if n != name {
			out = append(out, n)
		}
	}
	m.order = out
}

// CleanupOldState prunes rollback copies older than the retention window.
func (m *Manager) CleanupOldState(maxAge time.Duration) {
	root := filepath.Join(m.stateDir(), "rollback")
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cut := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.ModTime().Before(cut) {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}
