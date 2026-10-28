package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"

	"skin-nexus/server/internal/plugin"
)

type marketMeta struct {
	Schema       int                  `json:"schema"`
	Title        map[string]string    `json:"title,omitempty"`
	Description  map[string]string    `json:"description,omitempty"`
	Author       string               `json:"author,omitempty"`
	Changelog    string               `json:"changelog,omitempty"`
	Readme       string               `json:"readme,omitempty"`
	MinServer    string               `json:"minServer,omitempty"`
	Requires     *plugin.RequiresSpec `json:"requires,omitempty"`
	Conflicts    []string             `json:"conflicts,omitempty"`
	Capabilities []string             `json:"capabilities,omitempty"`
}

func readMarketMeta(path string) (*marketMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m marketMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("market.json: %w", err)
	}
	if m.Schema != 0 && m.Schema != 1 {
		return nil, fmt.Errorf("market.json schema %d unsupported", m.Schema)
	}
	return &m, nil
}

func validateMarketMeta(info plugin.Info, m *marketMeta) error {
	if m == nil {
		return nil
	}
	if !sameSet(m.Capabilities, info.Permissions) {
		return fmt.Errorf("market.json capabilities %v != plugin.json permissions %v", m.Capabilities, info.Permissions)
	}
	if m.MinServer != "" && info.MinServer != "" && m.MinServer != info.MinServer {
		return fmt.Errorf("market.json minServer %q != plugin.json minServer %q", m.MinServer, info.MinServer)
	}
	if m.MinServer != "" && info.MinServer == "" {
		return fmt.Errorf("market.json declares minServer but plugin.json does not")
	}
	if m.Requires != nil && info.Requires != nil && !reflect.DeepEqual(*m.Requires, *info.Requires) {
		return fmt.Errorf("market.json requires differs from plugin.json")
	}
	if m.Requires != nil && info.Requires == nil {
		return fmt.Errorf("market.json declares requires but plugin.json does not")
	}
	if len(m.Conflicts) > 0 && !sameSet(m.Conflicts, info.Conflicts) {
		return fmt.Errorf("market.json conflicts differ from plugin.json")
	}
	return nil
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}
