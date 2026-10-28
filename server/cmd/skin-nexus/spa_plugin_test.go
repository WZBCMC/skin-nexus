package main

import "testing"

func TestIsKnownSPAPathPluginUI(t *testing.T) {
	ui := func(p string) bool { return p == "ext/demo/page" }
	if !isKnownSPAPath("ext/demo/page", ui) {
		t.Fatal("declared plugin page should be SPA-known")
	}
	if isKnownSPAPath("ext/demo/missing", ui) {
		t.Fatal("undeclared plugin page must 404")
	}
	if !isKnownSPAPath("skinlib", nil) || isKnownSPAPath("nope", nil) {
		t.Fatal("built-in whitelist changed")
	}
}
