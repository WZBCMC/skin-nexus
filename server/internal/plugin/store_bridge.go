package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

func (rt *runtime) throw(err error) {
	panic(rt.vm.NewGoError(err))
}

func (rt *runtime) storeAPI() map[string]interface{} {
	return map[string]interface{}{
		"kv": map[string]interface{}{
			"get":    rt.storeKVGet,
			"set":    rt.storeKVSet,
			"delete": rt.storeKVDelete,
			"list":   rt.storeKVList,
		},
		"collection": rt.storeCollection,
		"onInstall":  rt.storeOnInstall,
		"onUpgrade":  rt.storeOnUpgrade,
	}
}

func (rt *runtime) requireStore(kv bool) *storeService {
	if rt.store == nil {
		rt.throw(errors.New("plugin store is not configured"))
	}
	if kv && !rt.store.canKV() {
		rt.throw(errors.New("missing permission store:kv"))
	}
	return rt.store
}

func (rt *runtime) storeKVGet(call goja.FunctionCall) goja.Value {
	s := rt.requireStore(true)
	v, ok, err := s.kvGet(call.Argument(0).String())
	if err != nil {
		rt.throw(err)
	}
	if !ok {
		if d := call.Argument(1); !goja.IsUndefined(d) {
			return d
		}
		return goja.Undefined()
	}
	return rt.vm.ToValue(v)
}

func (rt *runtime) storeKVSet(call goja.FunctionCall) goja.Value {
	s := rt.requireStore(true)
	var ttl *time.Time
	if opts := call.Argument(2); !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		obj := opts.ToObject(rt.vm)
		if ms := obj.Get("ttlMS"); ms != nil && !goja.IsUndefined(ms) {
			d := time.Duration(ms.ToInteger()) * time.Millisecond
			if d > 0 {
				t := time.Now().Add(d)
				ttl = &t
			}
		}
	}
	if err := s.kvSet(call.Argument(0).String(), call.Argument(1).String(), ttl); err != nil {
		rt.throw(err)
	}
	return rt.vm.ToValue(true)
}

func (rt *runtime) storeKVDelete(call goja.FunctionCall) goja.Value {
	s := rt.requireStore(true)
	if err := s.kvDelete(call.Argument(0).String()); err != nil {
		rt.throw(err)
	}
	return rt.vm.ToValue(true)
}

func (rt *runtime) storeKVList(call goja.FunctionCall) goja.Value {
	s := rt.requireStore(true)
	prefix, limit := storeListArgs(rt.vm, call.Argument(0))
	rows, err := s.kvList(prefix, limit)
	if err != nil {
		rt.throw(err)
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"key": r.Key, "value": r.Value})
	}
	return rt.vm.ToValue(out)
}

func storeListArgs(vm *goja.Runtime, v goja.Value) (string, int) {
	prefix, limit := "", 100
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return prefix, limit
	}
	obj := v.ToObject(vm)
	if p := obj.Get("prefix"); p != nil && !goja.IsUndefined(p) {
		prefix = p.String()
	}
	if l := obj.Get("limit"); l != nil && !goja.IsUndefined(l) {
		limit = int(l.ToInteger())
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return prefix, limit
}

func (rt *runtime) storeCollection(call goja.FunctionCall) goja.Value {
	name := call.Argument(0).String()
	s := rt.requireStore(false)
	if _, ok := s.collection(name); !ok {
		rt.throw(fmt.Errorf("collection %q is not declared", name))
	}
	return rt.vm.ToValue(map[string]interface{}{
		"get": func(c goja.FunctionCall) goja.Value {
			v, ok, err := s.docGet(name, c.Argument(0).String())
			if err != nil {
				rt.throw(err)
			}
			if !ok {
				return goja.Undefined()
			}
			return rt.vm.ToValue(v)
		},
		"put": func(c goja.FunctionCall) goja.Value {
			if err := s.docPut(name, c.Argument(0).String(), c.Argument(1).Export()); err != nil {
				rt.throw(err)
			}
			return rt.vm.ToValue(true)
		},
		"delete": func(c goja.FunctionCall) goja.Value {
			if err := s.docDelete(name, c.Argument(0).String()); err != nil {
				rt.throw(err)
			}
			return rt.vm.ToValue(true)
		},
		"list": func(c goja.FunctionCall) goja.Value {
			prefix, limit := storeListArgs(rt.vm, c.Argument(0))
			rows, err := s.docList(name, prefix, limit)
			if err != nil {
				rt.throw(err)
			}
			return rt.vm.ToValue(rows)
		},
	})
}

func (rt *runtime) storeOnInstall(call goja.FunctionCall) goja.Value {
	if !rt.info.hasPermission(PermStoreDocs) {
		rt.loadErr = errors.New("missing permission store:docs for onInstall")
		return goja.Undefined()
	}
	fn, ok := goja.AssertFunction(call.Argument(0))
	if !ok {
		rt.loadErr = errors.New("onInstall requires a function")
		return goja.Undefined()
	}
	rt.installFn = fn
	return goja.Undefined()
}

func (rt *runtime) storeOnUpgrade(call goja.FunctionCall) goja.Value {
	if !rt.info.hasPermission(PermStoreDocs) {
		rt.loadErr = errors.New("missing permission store:docs for onUpgrade")
		return goja.Undefined()
	}
	fn, ok := goja.AssertFunction(call.Argument(0))
	if !ok {
		rt.loadErr = errors.New("onUpgrade requires a function")
		return goja.Undefined()
	}
	rt.upgradeFn = fn
	return goja.Undefined()
}

func (rt *runtime) runMigrations(ctx context.Context, from, to int) error {
	_ = ctx
	if from > 0 && rt.upgradeFn == nil {
		return nil
	}
	if from == 0 && rt.installFn == nil {
		return nil
	}
	fn := rt.installFn
	if from > 0 {
		fn = rt.upgradeFn
	}
	payload := map[string]any{"from": from, "to": to}
	_, err := rt.withTimeout(5*time.Second, func() (goja.Value, error) {
		return fn(goja.Undefined(), rt.vm.ToValue(payload))
	})
	return err
}
