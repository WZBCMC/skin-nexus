package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
)

const maxFormStringLen = 512

// FormFieldError lets the HTTP layer point at the offending plugin/field
// without leaking schema internals.
type FormFieldError struct {
	Plugin  string
	Field   string
	Message string
}

func (e *FormFieldError) Error() string {
	if e.Field == "" {
		return e.Plugin + ": " + e.Message
	}
	return e.Plugin + "." + e.Field + ": " + e.Message
}

type formTarget struct {
	plugin string
	schema *ConfigSchema
}

func (m *Manager) formTargets(formID string) []formTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []formTarget
	for _, name := range m.order {
		inst := m.instances[name]
		if !inst.enabled || inst.info.UI == nil {
			continue
		}
		spec, ok := inst.info.UI.Forms[formID]
		if !ok {
			continue
		}
		schema, err := parseConfigSchema(spec.Schema)
		if err != nil {
			continue // validated at load; never fail registration on a bad schema
		}
		out = append(out, formTarget{plugin: name, schema: schema})
	}
	return out
}

// ValidateFormData normalizes the per-plugin form payload for a host form.
// Unknown plugin keys are ignored; enabled plugins that declare the form but
// received no values still contribute their schema defaults.
func (m *Manager) ValidateFormData(formID string, data map[string]any) (map[string]any, error) {
	if !allowedFormIDs[formID] {
		return nil, fmt.Errorf("unknown form %q", formID)
	}
	out := map[string]any{}
	for _, t := range m.formTargets(formID) {
		raw := map[string]any{}
		if v, ok := data[t.plugin]; ok && v != nil {
			obj, isMap := v.(map[string]any)
			if !isMap {
				return nil, &FormFieldError{Plugin: t.plugin, Message: "must be an object"}
			}
			raw = obj
		}
		norm, err := t.schema.Normalize(raw)
		if err != nil {
			fe := &FormFieldError{Plugin: t.plugin, Message: err.Error()}
			var ce *ConfigError
			if errors.As(err, &ce) {
				fe.Field, fe.Message = ce.Path, ce.Message
			}
			return nil, fe
		}
		if err := checkFormValueLimits(norm); err != nil {
			return nil, &FormFieldError{Plugin: t.plugin, Message: err.Error()}
		}
		if len(norm) > 0 {
			out[t.plugin] = norm
		}
	}
	if len(out) > 0 {
		raw, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		if len(raw) > MaxPluginFormBytes {
			return nil, fmt.Errorf("plugin form data exceeds %d bytes", MaxPluginFormBytes)
		}
	}
	return out, nil
}

func checkFormValueLimits(v any) error {
	switch x := v.(type) {
	case string:
		if len(x) > maxFormStringLen {
			return fmt.Errorf("value exceeds %d bytes", maxFormStringLen)
		}
	case []any:
		for _, item := range x {
			if err := checkFormValueLimits(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range x {
			if err := checkFormValueLimits(item); err != nil {
				return err
			}
		}
	}
	return nil
}
