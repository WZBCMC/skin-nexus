package plugin

import (
	"encoding/json"
	"strings"
	"testing"
)

const testConfigSchema = `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "default": "nexus"},
    "count": {"type": "integer", "minimum": 1, "maximum": 10, "default": 3},
    "enabled": {"type": "boolean", "default": true},
    "mode": {"type": "string", "enum": ["a", "b"], "default": "a"},
    "token": {"type": "string", "format": "secret"},
    "tags": {"type": "array", "items": {"type": "string"}},
    "nested": {"type": "object", "properties": {"x": {"type": "number", "default": 1.5}}}
  }
}`

func mustConfigSchema(t *testing.T, raw string) *ConfigSchema {
	t.Helper()
	s, err := parseConfigSchema(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestConfigSchemaDefaultsAndMask(t *testing.T) {
	s := mustConfigSchema(t, testConfigSchema)
	got, err := s.Normalize(map[string]any{"name": "site", "token": "secret-value"})
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "site" || got["count"] != int64(3) || got["enabled"] != true || got["mode"] != "a" {
		t.Fatalf("normalized = %#v", got)
	}
	if got["token"] != "secret-value" {
		t.Fatalf("secret not kept in normalized: %#v", got)
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok || nested["x"] != 1.5 {
		t.Fatalf("nested default = %#v", got["nested"])
	}
	masked := s.MaskSecrets(got)
	if masked["token"] != "********" {
		t.Fatalf("mask = %#v", masked["token"])
	}
	if masked["name"] != "site" {
		t.Fatalf("mask changed non-secret: %#v", masked)
	}
	if !s.SecretFields()["token"] {
		t.Fatal("token not marked secret")
	}
}

func TestConfigSchemaRejects(t *testing.T) {
	s := mustConfigSchema(t, testConfigSchema)
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"unknown", map[string]any{"nope": 1}, "unknown config key"},
		{"bad type", map[string]any{"count": "3"}, "must be an integer"},
		{"range low", map[string]any{"count": 0}, "must be >="},
		{"range high", map[string]any{"count": 11}, "must be <="},
		{"enum", map[string]any{"mode": "z"}, "enum"},
		{"array item", map[string]any{"tags": []any{1}}, "must be a string"},
		{"nested unknown", map[string]any{"nested": map[string]any{"y": 1}}, "unknown config key"},
		{"boolean", map[string]any{"enabled": "yes"}, "must be a boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Normalize(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestConfigSchemaRequiredAndSecretDefault(t *testing.T) {
	s, err := parseConfigSchema(json.RawMessage(`{"type":"object","required":["x"],"properties":{"x":{"type":"string"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Normalize(nil); err == nil {
		t.Fatal("required field accepted")
	}
	if _, err := parseConfigSchema(json.RawMessage(`{"type":"object","properties":{"x":{"type":"string","format":"secret","default":"oops"}}}`)); err == nil {
		t.Fatal("secret default accepted")
	}
}
