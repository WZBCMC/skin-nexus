// Package poc is the M0 technical spike: a side-by-side harness comparing
// goja (JS) and gopher-lua (Lua) as the SkinNexus in-process plugin runtime.
// Decision gate: docs/tech-route-eval-synthesis.md §8.3 four metrics.
package poc

import "encoding/json"

// FilterContext is the value-transform (Filter) hook payload contract.
// JSON is the host<->script interchange shape; scripts receive/return this
// object. Fields mirror a Yggdrasil profile filter scenario.
type FilterContext struct {
	ProfileName string                 `json:"profileName"`
	UUID        string                 `json:"uuid"`
	Textures    map[string]interface{} `json:"textures"`
}

// Clone round-trips through JSON so benchmarks measure a consistent baseline.
func (c *FilterContext) Clone() FilterContext {
	b, _ := json.Marshal(c)
	var out FilterContext
	_ = json.Unmarshal(b, &out)
	return out
}

// FilterRunner applies one Filter hook implemented in a script runtime.
type FilterRunner interface {
	Apply(c *FilterContext) error
	Close()
}
