package ygg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"skin-nexus/server/internal/plugin"
)

// TestBuildProfileAppliesPluginFiltersBeforeSign pins the ordering contract:
// plugin URL/profile filters must run BEFORE AssembleTextures, otherwise the
// signature would cover stale URLs and clients would reject the profile.
func TestBuildProfileAppliesPluginFiltersBeforeSign(t *testing.T) {
	signer, err := NewLocalSigner(filepath.Join(t.TempDir(), "sign.pem"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	svc := NewService(newMemStore(), signer)
	svc.now = func() time.Time { return now }
	svc.TextureURL = func(_ context.Context, hash, _, typ string) string {
		return "https://cdn.example.com/" + typ + "/" + hash
	}
	svc.ProfileFilter = func(_ context.Context, p *plugin.UserProfileFilter) {
		delete(p.TexturesURL, "CAPE") // plugin drops the cape
	}

	p := Player{
		ID: 1, Name: "steve", UUID: "0123456789abcdef0123456789abcdef",
		SkinURL: "http://x/textures/h1", SkinHash: "h1",
		CapeURL: "http://x/textures/h2", CapeHash: "h2",
	}
	prof, err := svc.buildProfile(context.Background(), p, true)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(prof.Properties[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Textures map[string]struct {
			URL string `json:"url"`
		} `json:"textures"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Textures["SKIN"].URL; got != "https://cdn.example.com/skin/h1" {
		t.Fatalf("SKIN url = %q", got)
	}
	if _, hasCape := decoded.Textures["CAPE"]; hasCape {
		t.Fatal("cape should have been removed by the profile filter")
	}

	// The signature must cover the FILTERED payload: re-assembling with the
	// same filtered set and same clock must reproduce value+signature.
	expected, err := AssembleTextures(p.UUID, p.Name, TextureSet{
		SkinURL: "https://cdn.example.com/skin/h1",
	}, now, true, signer)
	if err != nil {
		t.Fatal(err)
	}
	if expected[0].Value != prof.Properties[0].Value || expected[0].Signature != prof.Properties[0].Signature {
		t.Fatal("signature does not cover the filtered textures payload")
	}
}

// TestBuildProfileNoPluginsUnchanged guards the zero-plugin fast path.
func TestBuildProfileNoPluginsUnchanged(t *testing.T) {
	signer, err := NewLocalSigner(filepath.Join(t.TempDir(), "sign.pem"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	svc := NewService(newMemStore(), signer)
	svc.now = func() time.Time { return now }

	p := Player{ID: 1, Name: "steve", UUID: "u", SkinURL: "http://x/textures/h1", SkinHash: "h1"}
	prof, err := svc.buildProfile(context.Background(), p, false)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := AssembleTextures(p.UUID, p.Name, TextureSet{SkinURL: p.SkinURL}, now, false, signer)
	if err != nil {
		t.Fatal(err)
	}
	if expected[0].Value != prof.Properties[0].Value {
		t.Fatal("no-plugin profile changed")
	}
}
