package union

import (
	"encoding/json"
	"testing"
)

// TestVersionFieldToleratesNumberEncoding locks the 2026-10-24 live-hub
// finding: /serverlist `version` and /privatekey `privateKeyVersion` arrive
// as bare JSON numbers (3 not "3"). Wire field names frozen; decode must
// accept BOTH encodings onto the plain-string exported fields.
func TestVersionFieldToleratesNumberEncoding(t *testing.T) {
	// number-encoded (live hub shape)
	var sl ServerList
	if err := json.Unmarshal([]byte(`{"servers":[],"version":3}`), &sl); err != nil {
		t.Fatalf("number version: %v", err)
	}
	if sl.Version != "3" {
		t.Fatalf("number version decoded to %q, want \"3\"", sl.Version)
	}
	// string-encoded (spec sample shape)
	var sl2 ServerList
	if err := json.Unmarshal([]byte(`{"servers":[],"version":"3"}`), &sl2); err != nil {
		t.Fatalf("string version: %v", err)
	}
	if sl2.Version != "3" {
		t.Fatalf("string version decoded to %q, want \"3\"", sl2.Version)
	}
	// number-encoded privateKeyVersion
	var pk PrivateKeyDoc
	if err := json.Unmarshal([]byte(`{"privateKey":"PEM","privateKeyVersion":7}`), &pk); err != nil {
		t.Fatalf("number pkv: %v", err)
	}
	if pk.PrivateKeyVersion != "7" || pk.PrivateKey != "PEM" {
		t.Fatalf("pk decoded version=%q key_len=%d", pk.PrivateKeyVersion, len(pk.PrivateKey))
	}
	// null → empty
	var pk2 PrivateKeyDoc
	if err := json.Unmarshal([]byte(`{"privateKey":"PEM","privateKeyVersion":null}`), &pk2); err != nil {
		t.Fatalf("null pkv: %v", err)
	}
	if pk2.PrivateKeyVersion != "" {
		t.Fatalf("null pkv decoded to %q, want \"\"", pk2.PrivateKeyVersion)
	}
}
