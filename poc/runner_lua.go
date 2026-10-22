package poc

import (
	"encoding/json"
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// LuaRunner runs a Lua 5.1 Filter hook on pooled LStates.
// Bridging: FilterContext -> JSON -> map -> LTable (deep copy in),
// LTable -> JSON -> FilterContext (deep copy out). This is the heavier
// path AI2/AI1 predicted; the benchmark quantifies it.
type LuaRunner struct {
	pool sync.Pool
	name string
}

func NewLuaRunner(name, src string) (*LuaRunner, error) {
	r := &LuaRunner{name: name}
	r.pool.New = func() interface{} {
		L := lua.NewState()
		if err := L.DoString(src); err != nil {
			panic(fmt.Sprintf("init %s: %v", name, err))
		}
		return L
	}
	// Fail fast on syntax errors.
	L := r.pool.Get().(*lua.LState)
	defer r.pool.Put(L)
	return r, nil
}

func toLValue(L *lua.LState, v interface{}) lua.LValue {
	switch t := v.(type) {
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, val := range t {
			tbl.RawSetString(k, toLValue(L, val))
		}
		return tbl
	case []interface{}:
		tbl := L.NewTable()
		for i, val := range t {
			tbl.RawSetInt(i+1, toLValue(L, val))
		}
		return tbl
	case string:
		return lua.LString(t)
	case float64:
		return lua.LNumber(t)
	case bool:
		return lua.LBool(t)
	default:
		return lua.LNil
	}
}

func (r *LuaRunner) Apply(c *FilterContext) error {
	L := r.pool.Get().(*lua.LState)
	defer r.pool.Put(L)

	// Deep copy in via JSON.
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	fn := L.GetGlobal("apply_filter")
	if fn.Type() != lua.LTFunction {
		return fmt.Errorf("%s: apply_filter is not a function", r.name)
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, toLValue(L, m)); err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}
	ret := L.Get(-1)
	L.Pop(1)

	if tbl, ok := ret.(*lua.LTable); ok {
		out := make(map[string]interface{})
		tbl.ForEach(func(k, v lua.LValue) {
			out[k.String()] = luaToGo(v)
		})
		ob, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return json.Unmarshal(ob, c)
	}
	return nil
}

func luaToGo(v lua.LValue) interface{} {
	switch t := v.(type) {
	case *lua.LTable:
		m := make(map[string]interface{})
		t.ForEach(func(k, val lua.LValue) {
			m[k.String()] = luaToGo(val)
		})
		return m
	case lua.LString:
		return string(t)
	case lua.LNumber:
		return float64(t)
	case lua.LBool:
		return bool(t)
	default:
		return nil
	}
}

func (r *LuaRunner) Close() {}
