package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/signedpkg"
)

type manifestView struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Permissions []string             `json:"permissions"`
	MinServer   string               `json:"minServer"`
	Requires    *plugin.RequiresSpec `json:"requires"`
	Conflicts   []string             `json:"conflicts"`
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bundle := fs.String("bundle", "", "bundle zip")
	keys := fs.String("keys", "", "trust keys (id:base64pub,...)")
	hostVersion := fs.String("host-version", "", "check minServer against this host version")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *bundle == "" {
		fmt.Fprintln(stderr, "verify: --bundle is required")
		return 2
	}
	if *keys == "" {
		*keys = os.Getenv("SKIN_PLUGIN_TRUST_KEYS")
	}
	ring, err := keyringFor(*keys, "", nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	meta, err := signedpkg.VerifyBundle(*bundle, ring)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	raw, err := signedpkg.ReadEntry(*bundle, "plugin.json", 1<<20)
	if err != nil {
		fmt.Fprintln(stderr, "plugin.json:", err)
		return 1
	}
	var m manifestView
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintln(stderr, "plugin.json:", err)
		return 1
	}
	if m.Name != meta.Name || m.Version != meta.Version {
		fmt.Fprintf(stderr, "manifest name/version %s@%s != bundle %s@%s\n", m.Name, m.Version, meta.Name, meta.Version)
		return 1
	}
	if *hostVersion != "" && m.MinServer != "" {
		ok, err := plugin.CheckMinServer(m.MinServer, *hostVersion)
		if err != nil || !ok {
			fmt.Fprintf(stderr, "minServer %s not satisfied by host %s\n", m.MinServer, *hostVersion)
			return 1
		}
	}
	out := map[string]any{"ok": true, "name": meta.Name, "version": meta.Version, "keyId": meta.KeyID}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(out)
	} else {
		fmt.Fprintf(stdout, "OK %s %s (%s)\n", meta.Name, meta.Version, meta.KeyID)
	}
	return 0
}
