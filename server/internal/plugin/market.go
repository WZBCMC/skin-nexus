package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skin-nexus/server/internal/outbound"
	"skin-nexus/server/internal/signedpkg"
)

const marketCacheTTL = 10 * time.Minute

type MarketDist struct {
	URL    string `json:"url"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256"`
}

type MarketPackage struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Title        map[string]string `json:"title,omitempty"`
	Description  map[string]string `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	Requires     *RequiresSpec     `json:"requires,omitempty"`
	Conflicts    []string          `json:"conflicts,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	MinServer    string            `json:"minServer,omitempty"`
	Changelog    string            `json:"changelog,omitempty"`
	Readme       string            `json:"readme,omitempty"`
	Dist         MarketDist        `json:"dist"`
}

type MarketRegistry struct {
	Schema      int             `json:"schema"`
	GeneratedAt string          `json:"generatedAt"`
	ExpiresAt   string          `json:"expiresAt"`
	KeyID       string          `json:"keyId"`
	Packages    []MarketPackage `json:"packages"`
	Signature   string          `json:"signature,omitempty"`
}

type MarketEntry struct {
	MarketPackage
	Installed string       `json:"installed,omitempty"`
	CanUpdate bool         `json:"canUpdate,omitempty"`
	Plan      *InstallPlan `json:"plan,omitempty"`
}

func (m *Manager) marketRegistryPayload(r *MarketRegistry) ([]byte, error) {
	// Keep field order identical to the signed payload; signature is omitted.
	payload := struct {
		Schema      int             `json:"schema"`
		GeneratedAt string          `json:"generatedAt"`
		ExpiresAt   string          `json:"expiresAt"`
		KeyID       string          `json:"keyId"`
		Packages    []MarketPackage `json:"packages"`
	}{r.Schema, r.GeneratedAt, r.ExpiresAt, r.KeyID, r.Packages}
	return json.Marshal(payload)
}

func (m *Manager) fetchRegistry(ctx context.Context, force bool) (*MarketRegistry, error) {
	if m.cfg.MarketURL == "" {
		return nil, errors.New("plugin market is not configured")
	}
	if m.cfg.Keyring == nil {
		return nil, errors.New("plugin trust keys are not configured")
	}
	m.marketMu.Lock()
	if !force && m.marketCache != nil && time.Since(m.marketFetched) < marketCacheTTL {
		r := m.marketCache
		m.marketMu.Unlock()
		return r, nil
	}
	m.marketMu.Unlock()

	u, err := url.Parse(m.cfg.MarketURL)
	if err != nil {
		return nil, err
	}
	client := outbound.New([]string{u.Hostname()}, 5*time.Second, 1<<20)
	raw, err := client.Get(ctx, m.cfg.MarketURL)
	if err != nil {
		if cached := m.readMarketCache(); cached != nil {
			m.marketMu.Lock()
			m.marketCache, m.marketFetched = cached, time.Now()
			m.marketMu.Unlock()
			return cached, nil
		}
		return nil, err
	}
	var reg MarketRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("market registry: %w", err)
	}
	if reg.Schema != 1 {
		return nil, fmt.Errorf("unsupported market schema %d", reg.Schema)
	}
	sig, err := signedpkg.DecodeSignature(reg.Signature)
	if err != nil {
		return nil, err
	}
	payload, err := m.marketRegistryPayload(&reg)
	if err != nil {
		return nil, err
	}
	if err := m.cfg.Keyring.Verify(reg.KeyID, payload, sig); err != nil {
		return nil, err
	}
	if reg.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, reg.ExpiresAt)
		if err != nil || time.Now().After(exp) {
			return nil, errors.New("market registry has expired")
		}
	}
	m.marketMu.Lock()
	m.marketCache, m.marketFetched = &reg, time.Now()
	m.marketMu.Unlock()
	m.writeMarketCache(raw)
	return &reg, nil
}

func (m *Manager) marketCachePath() string {
	return filepath.Join(m.stateDir(), "cache", "registry.json")
}

func (m *Manager) writeMarketCache(raw []byte) {
	path := m.marketCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func (m *Manager) readMarketCache() *MarketRegistry {
	raw, err := os.ReadFile(m.marketCachePath())
	if err != nil {
		return nil
	}
	var reg MarketRegistry
	if json.Unmarshal(raw, &reg) != nil || reg.Schema != 1 {
		return nil
	}
	if reg.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, reg.ExpiresAt)
		if err != nil || time.Now().After(exp) {
			return nil
		}
	}
	return &reg
}

func (m *Manager) MarketCatalog(ctx context.Context, force bool) ([]MarketEntry, error) {
	reg, err := m.fetchRegistry(ctx, force)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	versions := map[string]string{}
	for name, inst := range m.instances {
		versions[name] = inst.info.Version
	}
	m.mu.RUnlock()
	out := make([]MarketEntry, 0, len(reg.Packages))
	for _, p := range reg.Packages {
		e := MarketEntry{MarketPackage: p, Installed: versions[p.Name]}
		e.CanUpdate = e.Installed != "" && e.Installed != p.Version
		e.Plan, _ = m.PlanInstall(PlanInput{
			Name: p.Name, Version: p.Version, Permissions: p.Capabilities,
			Requires: p.Requires, Conflicts: p.Conflicts,
		})
		out = append(out, e)
	}
	return out, nil
}

func (m *Manager) MarketPlan(ctx context.Context, name string) (*InstallPlan, error) {
	reg, err := m.fetchRegistry(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, p := range reg.Packages {
		if p.Name == name {
			return m.PlanInstall(PlanInput{
				Name: p.Name, Version: p.Version, Permissions: p.Capabilities,
				Requires: p.Requires, Conflicts: p.Conflicts,
			})
		}
	}
	return nil, ErrPluginNotFound
}

// MarketInstall downloads a signed artifact from the trusted registry, checks
// transport SHA-256, then hands it to the bundle installer for signature,
// staging and atomic swap.
func (m *Manager) MarketInstall(ctx context.Context, name string, consent bool) (*InstallResult, error) {
	reg, err := m.fetchRegistry(ctx, false)
	if err != nil {
		return nil, err
	}
	var pkg *MarketPackage
	for i := range reg.Packages {
		if reg.Packages[i].Name == name {
			pkg = &reg.Packages[i]
			break
		}
	}
	if pkg == nil {
		return nil, ErrPluginNotFound
	}
	if pkg.MinServer != "" && m.cfg.HostVersion != "" {
		ok, err := checkMinServer(pkg.MinServer, m.cfg.HostVersion)
		if err != nil || !ok {
			return &InstallResult{Plan: &InstallPlan{Name: pkg.Name, Version: pkg.Version, CanInstall: false, Missing: []string{"host " + m.cfg.HostVersion + " does not satisfy " + pkg.MinServer}}, State: "blocked", Source: "market"}, ErrInstallBlocked
		}
	}
	plan, err := m.PlanInstall(PlanInput{
		Name: pkg.Name, Version: pkg.Version, Permissions: pkg.Capabilities,
		Requires: pkg.Requires, Conflicts: pkg.Conflicts,
	})
	if err != nil {
		return nil, err
	}
	if !plan.CanInstall {
		return &InstallResult{Plan: plan, State: "blocked", Source: "market"}, ErrInstallBlocked
	}
	if len(plan.AddedPermissions) > 0 && !consent {
		return &InstallResult{Plan: plan, State: "consent", Source: "market"}, ErrConsentRequired
	}
	u, err := url.Parse(pkg.Dist.URL)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("market package has invalid dist URL")
	}
	client := outbound.New([]string{u.Hostname()}, 15*time.Second, 32<<20)
	raw, err := client.Get(ctx, pkg.Dist.URL)
	if err != nil {
		return nil, err
	}
	if pkg.Dist.Size > 0 && int64(len(raw)) != pkg.Dist.Size {
		return nil, errors.New("market artifact size mismatch")
	}
	sum := sha256.Sum256(raw)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), pkg.Dist.SHA256) {
		return nil, errors.New("market artifact sha256 mismatch")
	}
	return m.InstallBytes(ctx, raw, consent, "market")
}
