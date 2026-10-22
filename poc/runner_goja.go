package poc

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// GojaRunner runs a JS Filter hook on pooled goja runtimes.
// Bridging: Go struct -> goja value via reflection mapper (zero JSON copy in),
// script result -> FilterContext via JSON round-trip (deterministic, simple).
type GojaRunner struct {
	pool sync.Pool
	prog *goja.Program
	name string
}

func NewGojaRunner(name, src string) (*GojaRunner, error) {
	prog, err := goja.Compile(name, src, false)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", name, err)
	}
	r := &GojaRunner{prog: prog, name: name}
	r.pool.New = func() interface{} {
		vm := goja.New()
		// Expose struct fields by their json tag (profileName, textures, ...)
		// so script-side property writes go through to the Go struct.
		vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", false))
		if _, err := vm.RunProgram(prog); err != nil {
			panic(fmt.Sprintf("init %s: %v", name, err))
		}
		return vm
	}
	return r, nil
}

func (r *GojaRunner) Apply(c *FilterContext) error {
	vm := r.pool.Get().(*goja.Runtime)
	defer r.pool.Put(vm)

	fn, ok := goja.AssertFunction(vm.Get("applyFilter"))
	if !ok {
		return fmt.Errorf("%s: applyFilter is not a function", r.name)
	}
	out, err := fn(goja.Undefined(), vm.ToValue(c))
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}
	if out != nil && !goja.IsUndefined(out) && !goja.IsNull(out) {
		b, err := json.Marshal(out.Export())
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, c); err != nil {
			return err
		}
	}
	return nil
}

func (r *GojaRunner) Close() {}
