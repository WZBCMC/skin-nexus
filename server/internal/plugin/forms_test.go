package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const surveyManifest = `{
  "name": "survey",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["ui:forms"],
  "ui": {
    "forms": {
      "auth.register": {
        "schema": {
          "type": "object",
          "properties": {
            "source": {"type": "string", "enum": ["friend", "search"]},
            "note": {"type": "string", "default": "hi"}
          },
          "required": ["source"]
        }
      }
    }
  }
}`

func newFormsHost(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	writePlugin(t, root, "survey", surveyManifest, "0;")
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "survey"); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFormManifestRejections(t *testing.T) {
	cases := []struct{ name, manifest, want string }{
		{
			"missing permission",
			`{"name":"fx","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":[],
			  "ui":{"forms":{"auth.register":{"schema":{"type":"object","properties":{}}}}}}`,
			"ui:forms",
		},
		{
			"unknown form",
			`{"name":"fx","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":["ui:forms"],
			  "ui":{"forms":{"admin.settings":{"schema":{"type":"object","properties":{}}}}}}`,
			"unknown ui form",
		},
		{
			"secret field",
			`{"name":"fx","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":["ui:forms"],
			  "ui":{"forms":{"auth.register":{"schema":{"type":"object","properties":{"token":{"type":"string","format":"secret"}}}}}}}`,
			"secret",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			dir := writePlugin(t, root, "fx", c.manifest, "0;")
			_, err := loadManifest(dir, "fx")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestValidateFormData(t *testing.T) {
	m := newFormsHost(t)
	if ui := m.UI(); len(ui) != 1 || ui[0].Forms["auth.register"].Schema == nil {
		t.Fatalf("UI forms missing: %+v", ui)
	}

	out, err := m.ValidateFormData("auth.register", map[string]any{
		"survey": map[string]any{"source": "friend"},
		"ghost":  map[string]any{"x": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	survey, ok := out["survey"].(map[string]any)
	if !ok || survey["source"] != "friend" || survey["note"] != "hi" {
		t.Fatalf("out = %+v", out)
	}
	if _, hasGhost := out["ghost"]; hasGhost {
		t.Fatal("unknown plugin key must be ignored")
	}

	_, err = m.ValidateFormData("auth.register", map[string]any{
		"survey": map[string]any{"source": "other"},
	})
	var fe *FormFieldError
	if !errors.As(err, &fe) || fe.Plugin != "survey" || fe.Field != "source" {
		t.Fatalf("enum error = %v (%+v)", err, fe)
	}

	if _, err := m.ValidateFormData("auth.register", map[string]any{
		"survey": map[string]any{"source": 42},
	}); err == nil {
		t.Fatal("wrong type must fail")
	}

	if _, err := m.ValidateFormData("auth.register", map[string]any{
		"survey": map[string]any{"source": "friend", "note": strings.Repeat("x", 600)},
	}); err == nil || !strings.Contains(err.Error(), "512") {
		t.Fatalf("oversized string = %v", err)
	}

	if err := m.Disable(context.Background(), "survey"); err != nil {
		t.Fatal(err)
	}
	if out, err := m.ValidateFormData("auth.register", map[string]any{"survey": map[string]any{"source": "friend"}}); err != nil || len(out) != 0 {
		t.Fatalf("disabled form should be inert: %v %+v", err, out)
	}
}

func TestValidateFormDataUnknownForm(t *testing.T) {
	m := newFormsHost(t)
	if _, err := m.ValidateFormData("nope", nil); err == nil {
		t.Fatal("unknown form must fail")
	}
}
