package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// ConfigSchema is the supported JSON Schema subset. It deliberately supports
// only what the generated admin form and runtime can validate identically;
// exotic JSON Schema keywords are rejected by parseConfigSchema.
type ConfigSchema struct {
	Raw    json.RawMessage
	Fields []*ConfigField
	ByName map[string]*ConfigField
	Order  []string
}

type ConfigField struct {
	Name        string
	Type        string
	Title       string
	Description string
	UI          string
	Secret      bool
	Required    bool
	Default     any
	Enum        []any
	Minimum     *float64
	Maximum     *float64
	Items       string
	Properties  []*ConfigField
	ByName      map[string]*ConfigField
}

type ConfigError struct {
	Path    string
	Message string
}

func (e *ConfigError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

func parseConfigSchema(raw json.RawMessage) (*ConfigSchema, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("config schema is empty")
	}
	var root struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("config schema: %w", err)
	}
	if root.Type != "object" {
		return nil, fmt.Errorf("config schema type must be object")
	}
	required := map[string]bool{}
	for _, n := range root.Required {
		required[n] = true
	}
	s := &ConfigSchema{Raw: append(json.RawMessage(nil), raw...), ByName: map[string]*ConfigField{}}
	names := make([]string, 0, len(root.Properties))
	for n := range root.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := parseConfigField(name, root.Properties[name], required[name])
		if err != nil {
			return nil, err
		}
		s.Fields = append(s.Fields, f)
		s.ByName[name] = f
		s.Order = append(s.Order, name)
	}
	return s, nil
}

func parseConfigField(name string, raw json.RawMessage, required bool) (*ConfigField, error) {
	var def struct {
		Type        string                     `json:"type"`
		Title       string                     `json:"title"`
		Description string                     `json:"description"`
		Format      string                     `json:"format"`
		XUI         string                     `json:"x-ui"`
		Default     json.RawMessage            `json:"default"`
		Enum        []any                      `json:"enum"`
		Minimum     *float64                   `json:"minimum"`
		Maximum     *float64                   `json:"maximum"`
		Items       json.RawMessage            `json:"items"`
		Properties  map[string]json.RawMessage `json:"properties"`
		Required    []string                   `json:"required"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&def); err != nil {
		return nil, fmt.Errorf("config field %q: %w", name, err)
	}
	f := &ConfigField{
		Name: name, Type: def.Type, Title: def.Title, Description: def.Description,
		UI: def.XUI, Required: required, Enum: def.Enum, Minimum: def.Minimum, Maximum: def.Maximum,
	}
	if f.Type == "" && len(def.Properties) > 0 {
		f.Type = "object"
	}
	if f.Type == "" {
		f.Type = "string"
	}
	if def.Format == "secret" || f.UI == "password" {
		f.Secret = true
	}
	if f.UI == "" {
		switch f.Type {
		case "string":
			f.UI = "text"
		case "integer", "number":
			f.UI = "number"
		case "boolean":
			f.UI = "checkbox"
		case "array":
			f.UI = "list"
		case "object":
			f.UI = "group"
		}
	}
	if len(f.Enum) > 0 && f.UI != "password" {
		f.UI = "select"
	}
	if len(def.Default) > 0 {
		if f.Secret {
			return nil, fmt.Errorf("config field %q: secret fields must not declare a default", name)
		}
		v, err := decodeJSONValue(def.Default)
		if err != nil {
			return nil, fmt.Errorf("config field %q default: %w", name, err)
		}
		f.Default = v
	}
	switch f.Type {
	case "string", "integer", "number", "boolean", "array", "object":
	case "":
		return nil, fmt.Errorf("config field %q has no type", name)
	default:
		return nil, fmt.Errorf("config field %q type %q unsupported", name, f.Type)
	}
	if f.Type == "array" {
		var items struct {
			Type string `json:"type"`
		}
		if len(def.Items) > 0 {
			if err := json.Unmarshal(def.Items, &items); err != nil {
				return nil, fmt.Errorf("config field %q items: %w", name, err)
			}
		}
		if items.Type == "" {
			items.Type = "string"
		}
		if items.Type != "string" && items.Type != "integer" && items.Type != "number" {
			return nil, fmt.Errorf("config field %q items type %q unsupported", name, items.Type)
		}
		f.Items = items.Type
	}
	if f.Type == "object" {
		req := map[string]bool{}
		for _, r := range def.Required {
			req[r] = true
		}
		names := make([]string, 0, len(def.Properties))
		for n := range def.Properties {
			names = append(names, n)
		}
		sort.Strings(names)
		f.ByName = map[string]*ConfigField{}
		for _, n := range names {
			nf, err := parseConfigField(n, def.Properties[n], req[n])
			if err != nil {
				return nil, fmt.Errorf("config field %q.%s: %w", name, n, err)
			}
			f.Properties = append(f.Properties, nf)
			f.ByName[n] = nf
		}
	}
	return f, nil
}

func decodeJSONValue(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return normalizeJSONNumber(v), nil
}

func normalizeJSONNumber(v any) any {
	switch n := v.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
		f, _ := n.Float64()
		return f
	case []any:
		for i := range n {
			n[i] = normalizeJSONNumber(n[i])
		}
	case map[string]any:
		for k := range n {
			n[k] = normalizeJSONNumber(n[k])
		}
	}
	return v
}

// Normalize merges defaults and validates values. Unknown keys are rejected so
// a typo cannot silently persist.
func (s *ConfigSchema) Normalize(values map[string]any) (map[string]any, error) {
	out := map[string]any{}
	if values != nil {
		for k := range values {
			if _, ok := s.ByName[k]; !ok {
				return nil, &ConfigError{Path: k, Message: "unknown config key"}
			}
		}
	}
	for _, f := range s.Fields {
		v, present := values[f.Name]
		if !present || v == nil {
			if f.Type == "object" {
				nv, err := validateField(f, map[string]any{}, f.Name)
				if err != nil {
					return nil, err
				}
				out[f.Name] = nv
				continue
			}
			if f.Required && f.Default == nil {
				return nil, &ConfigError{Path: f.Name, Message: "required"}
			}
			if f.Default != nil {
				out[f.Name] = cloneJSON(f.Default)
			}
			continue
		}
		nv, err := validateField(f, v, f.Name)
		if err != nil {
			return nil, err
		}
		out[f.Name] = nv
	}
	return out, nil
}

func validateField(f *ConfigField, v any, path string) (any, error) {
	if v == nil {
		return nil, &ConfigError{Path: path, Message: "must not be null"}
	}
	switch f.Type {
	case "string":
		s, ok := v.(string)
		if !ok {
			return nil, &ConfigError{Path: path, Message: "must be a string"}
		}
		if len(s) > 64<<10 {
			return nil, &ConfigError{Path: path, Message: "string too long"}
		}
		if err := checkEnum(f, s, path); err != nil {
			return nil, err
		}
		return s, nil
	case "integer":
		n, ok := asFloat(v)
		if !ok || math.Trunc(n) != n {
			return nil, &ConfigError{Path: path, Message: "must be an integer"}
		}
		if err := checkRange(f, n, path); err != nil {
			return nil, err
		}
		if err := checkEnum(f, int64(n), path); err != nil {
			return nil, err
		}
		return int64(n), nil
	case "number":
		n, ok := asFloat(v)
		if !ok {
			return nil, &ConfigError{Path: path, Message: "must be a number"}
		}
		if err := checkRange(f, n, path); err != nil {
			return nil, err
		}
		return n, nil
	case "boolean":
		b, ok := v.(bool)
		if !ok {
			return nil, &ConfigError{Path: path, Message: "must be a boolean"}
		}
		return b, nil
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return nil, &ConfigError{Path: path, Message: "must be an array"}
		}
		out := make([]any, 0, len(arr))
		for i, item := range arr {
			iv, err := validateScalar(f.Items, item)
			if err != nil {
				return nil, &ConfigError{Path: fmt.Sprintf("%s[%d]", path, i), Message: err.Error()}
			}
			out = append(out, iv)
		}
		return out, nil
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, &ConfigError{Path: path, Message: "must be an object"}
		}
		out := map[string]any{}
		for k := range obj {
			if _, ok := f.ByName[k]; !ok {
				return nil, &ConfigError{Path: path + "." + k, Message: "unknown config key"}
			}
		}
		for _, child := range f.Properties {
			cv, present := obj[child.Name]
			if !present || cv == nil {
				if child.Required && child.Default == nil {
					return nil, &ConfigError{Path: path + "." + child.Name, Message: "required"}
				}
				if child.Default != nil {
					out[child.Name] = cloneJSON(child.Default)
				}
				continue
			}
			nv, err := validateField(child, cv, path+"."+child.Name)
			if err != nil {
				return nil, err
			}
			out[child.Name] = nv
		}
		return out, nil
	default:
		return nil, &ConfigError{Path: path, Message: "unsupported type " + f.Type}
	}
}

func validateScalar(typ string, v any) (any, error) {
	switch typ {
	case "string":
		if s, ok := v.(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("must be a string")
	case "integer":
		n, ok := asFloat(v)
		if !ok || math.Trunc(n) != n {
			return nil, fmt.Errorf("must be an integer")
		}
		return int64(n), nil
	case "number":
		n, ok := asFloat(v)
		if !ok {
			return nil, fmt.Errorf("must be a number")
		}
		return n, nil
	}
	return nil, fmt.Errorf("unsupported item type %s", typ)
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func checkRange(f *ConfigField, n float64, path string) error {
	if f.Minimum != nil && n < *f.Minimum {
		return &ConfigError{Path: path, Message: fmt.Sprintf("must be >= %v", *f.Minimum)}
	}
	if f.Maximum != nil && n > *f.Maximum {
		return &ConfigError{Path: path, Message: fmt.Sprintf("must be <= %v", *f.Maximum)}
	}
	return nil
}

func checkEnum(f *ConfigField, v any, path string) error {
	if len(f.Enum) == 0 {
		return nil
	}
	for _, e := range f.Enum {
		if valuesEqual(e, v) {
			return nil
		}
	}
	return &ConfigError{Path: path, Message: "value not in enum"}
}

func valuesEqual(a, b any) bool {
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			return af == bf
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func cloneJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, vv := range x {
			out[k] = cloneJSON(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = cloneJSON(x[i])
		}
		return out
	default:
		return x
	}
}

// Defaults materializes the schema defaults without enforcing required fields;
// the admin form uses it to render a plugin whose required secret is not set
// yet. Runtime loads still go through Normalize and fail closed.
func (s *ConfigSchema) Defaults() map[string]any {
	out := map[string]any{}
	for _, f := range s.Fields {
		if f.Default != nil {
			out[f.Name] = cloneJSON(f.Default)
			continue
		}
		if f.Type == "object" {
			out[f.Name] = defaultsForFields(f.Properties)
		}
	}
	return out
}

func defaultsForFields(fields []*ConfigField) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if f.Default != nil {
			out[f.Name] = cloneJSON(f.Default)
			continue
		}
		if f.Type == "object" {
			out[f.Name] = defaultsForFields(f.Properties)
		}
	}
	return out
}

// SecretFields returns the top-level keys that must be encrypted at rest.
func (s *ConfigSchema) SecretFields() map[string]bool {
	out := map[string]bool{}
	for _, f := range s.Fields {
		if f.Secret {
			out[f.Name] = true
		}
	}
	return out
}

// MaskSecrets returns a copy safe for admin display.
func (s *ConfigSchema) MaskSecrets(values map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range values {
		if f := s.ByName[k]; f != nil && f.Secret {
			if str, ok := v.(string); ok && str != "" {
				out[k] = "********"
			} else {
				out[k] = ""
			}
			continue
		}
		out[k] = cloneJSON(v)
	}
	return out
}

// configState is the live, in-memory config snapshot shared by the host and a
// plugin VM. Admin updates replace the map atomically; plugins never see the
// persisted ciphertext.
type configState struct {
	mu     sync.RWMutex
	values map[string]any
}

func newConfigState(values map[string]any) *configState {
	if values == nil {
		values = map[string]any{}
	}
	return &configState{values: values}
}

func (c *configState) get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return cloneJSON(v), ok
}

func (c *configState) all() map[string]any {
	if c == nil {
		return map[string]any{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.values == nil {
		return map[string]any{}
	}
	return cloneJSON(c.values).(map[string]any)
}

func (c *configState) replace(values map[string]any) {
	if c == nil {
		return
	}
	if values == nil {
		values = map[string]any{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = cloneJSON(values).(map[string]any)
}
