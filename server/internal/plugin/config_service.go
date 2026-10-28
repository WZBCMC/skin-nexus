package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrPluginNoConfig  = errors.New("plugin has no config schema")
	ErrPluginNoStore   = errors.New("plugin store is not configured")
	ErrPluginNoSecrets = errors.New("plugin secret box is not configured")
)

// ConfigView is the admin-safe shape: secret values are always masked.
type ConfigView struct {
	Schema  json.RawMessage `json:"schema"`
	Values  map[string]any  `json:"values"`
	Secrets map[string]bool `json:"secrets,omitempty"`
}

func (m *Manager) loadConfig(ctx context.Context, name string, info Info) (*ConfigSchema, *configState, error) {
	if info.Config == nil {
		return nil, newConfigState(nil), nil
	}
	schema, err := parseConfigSchema(info.Config.Schema)
	if err != nil {
		return nil, nil, err
	}
	raw := map[string]any{}
	if st := m.platformStore(); st != nil {
		if data, ok, err := st.GetPluginConfig(ctx, name); err != nil {
			return schema, newConfigState(nil), err
		} else if ok && len(data) > 0 {
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			if err := dec.Decode(&raw); err != nil {
				return schema, newConfigState(nil), fmt.Errorf("config: %w", err)
			}
		}
		for key := range schema.SecretFields() {
			ct, nonce, _, ok, err := st.GetPluginSecret(ctx, name, key)
			if err != nil {
				return schema, newConfigState(nil), err
			}
			if !ok {
				continue
			}
			if m.cfg.SecretBox == nil {
				return schema, newConfigState(nil), ErrPluginNoSecrets
			}
			plain, err := m.cfg.SecretBox.OpenString(ct, nonce)
			if err != nil {
				return schema, newConfigState(nil), fmt.Errorf("secret %s: %w", key, err)
			}
			raw[key] = plain
		}
	}
	values, err := schema.Normalize(raw)
	if err != nil {
		return schema, newConfigState(nil), err
	}
	return schema, newConfigState(values), nil
}

// PluginConfig returns the schema and masked values for the admin form.
func (m *Manager) PluginConfig(name string) (*ConfigView, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[name]
	if !ok {
		return nil, ErrPluginNotFound
	}
	if inst.configSchema == nil {
		return nil, ErrPluginNoConfig
	}
	values := map[string]any{}
	if inst.config != nil {
		values = inst.config.all()
	}
	if len(values) == 0 {
		values = inst.configSchema.Defaults()
	}
	return &ConfigView{
		Schema:  inst.configSchema.Raw,
		Values:  inst.configSchema.MaskSecrets(values),
		Secrets: inst.configSchema.SecretFields(),
	}, nil
}

// SavePluginConfig validates, persists and applies a config update. A masked
// secret ("********") preserves the stored value, which lets the admin form
// round-trip without ever sending the plaintext back to the browser.
func (m *Manager) SavePluginConfig(ctx context.Context, name string, values map[string]any) (*ConfigView, error) {
	m.mu.RLock()
	inst, ok := m.instances[name]
	if !ok {
		m.mu.RUnlock()
		return nil, ErrPluginNotFound
	}
	schema := inst.configSchema
	old := inst.config.all()
	m.mu.RUnlock()
	if schema == nil {
		return nil, ErrPluginNoConfig
	}
	st := m.platformStore()
	if st == nil {
		return nil, ErrPluginNoStore
	}
	normalized, err := schema.Normalize(values)
	if err != nil {
		return nil, err
	}
	secretFields := schema.SecretFields()
	public := map[string]any{}
	for k, v := range normalized {
		if secretFields[k] {
			continue
		}
		public[k] = v
	}
	raw, err := json.Marshal(public)
	if err != nil {
		return nil, err
	}
	if err := st.SetPluginConfig(ctx, name, raw); err != nil {
		return nil, err
	}
	for key := range secretFields {
		v, present := values[key]
		if !present {
			if oldV, ok := old[key]; ok {
				normalized[key] = oldV
			}
			continue
		}
		if s, ok := v.(string); ok && s == "********" {
			if oldV, ok := old[key]; ok {
				normalized[key] = oldV
			} else {
				delete(normalized, key)
			}
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			if err := st.DeletePluginSecret(ctx, name, key); err != nil {
				return nil, err
			}
			delete(normalized, key)
			continue
		}
		s := fmt.Sprint(v)
		if m.cfg.SecretBox == nil {
			return nil, ErrPluginNoSecrets
		}
		ct, nonce, err := m.cfg.SecretBox.SealString(s)
		if err != nil {
			return nil, err
		}
		if err := st.SetPluginSecret(ctx, name, key, ct, nonce, m.cfg.SecretBox.KeyVersion()); err != nil {
			return nil, err
		}
		normalized[key] = s
	}
	m.mu.Lock()
	if cur, ok := m.instances[name]; ok {
		if cur.config == nil {
			cur.config = newConfigState(nil)
		}
		cur.config.replace(normalized)
	}
	m.mu.Unlock()
	m.audit(ctx, "plugin_config_update", map[string]any{"plugin": name, "keys": sortedKeys(values)})
	return m.PluginConfig(name)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// stable audit output; sort.Strings is not worth another import here.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
