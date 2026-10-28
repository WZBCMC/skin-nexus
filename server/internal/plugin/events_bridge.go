package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

var knownEvents = map[string]bool{
	"user.registered":  true,
	"user.logged_in":   true,
	"user.deleted":     true,
	"player.added":     true,
	"player.renamed":   true,
	"player.deleted":   true,
	"profile.updated":  true,
	"texture.uploaded": true,
	"texture.deleted":  true,
}

var knownGuards = map[string]bool{
	"user.registration.check": true,
	"auth.login.check":        true,
	"auth.join.check":         true,
	"profile.update.check":    true,
	"player.rename.check":     true,
	"texture.upload.check":    true,
	"texture.delete.check":    true,
}

func payloadMap(p EventPayload) map[string]any {
	m := map[string]any{"kind": p.Kind}
	if p.UserID != 0 {
		m["userId"] = p.UserID
	}
	if p.PlayerID != 0 {
		m["playerId"] = p.PlayerID
	}
	if p.PlayerName != "" {
		m["playerName"] = p.PlayerName
	}
	if p.UUID != "" {
		m["uuid"] = p.UUID
	}
	if p.Hash != "" {
		m["hash"] = p.Hash
	}
	if p.Type != "" {
		m["type"] = p.Type
	}
	if p.IP != "" {
		m["ip"] = p.IP
	}
	if p.Data != nil {
		m["data"] = p.Data
	}
	return m
}

func (rt *runtime) onEvent(call goja.FunctionCall) goja.Value {
	if !rt.info.hasPermission(PermEventsSubscribe) {
		rt.loadErr = errors.New("missing permission events:subscribe")
		return goja.Undefined()
	}
	kind := call.Argument(0).String()
	if !knownEvents[kind] {
		rt.loadErr = fmt.Errorf("unknown event %q", kind)
		return goja.Undefined()
	}
	fn, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		rt.loadErr = errors.New("event handler must be a function")
		return goja.Undefined()
	}
	d := rt.defaults.hook
	if opts := call.Argument(2); !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		if t := opts.ToObject(rt.vm).Get("timeoutMS"); t != nil && !goja.IsUndefined(t) {
			d = clampTimeout(int(t.ToInteger()), rt.defaults.hook)
		}
	}
	rt.hooks.events[kind] = append(rt.hooks.events[kind], timed[eventFn]{fn: rt.wrapEvent(fn, d)})
	return goja.Undefined()
}

func (rt *runtime) onGuard(call goja.FunctionCall) goja.Value {
	if !rt.info.hasPermission(PermAuthGuards) {
		rt.loadErr = errors.New("missing permission auth:guards")
		return goja.Undefined()
	}
	kind := call.Argument(0).String()
	if !knownGuards[kind] {
		rt.loadErr = fmt.Errorf("unknown guard %q", kind)
		return goja.Undefined()
	}
	fn, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		rt.loadErr = errors.New("guard handler must be a function")
		return goja.Undefined()
	}
	d := rt.defaults.auth
	failClosed := true
	if opts := call.Argument(2); !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		obj := opts.ToObject(rt.vm)
		if t := obj.Get("timeoutMS"); t != nil && !goja.IsUndefined(t) {
			d = clampTimeout(int(t.ToInteger()), rt.defaults.auth)
		}
		if e := obj.Get("onError"); e != nil && !goja.IsUndefined(e) {
			switch e.String() {
			case "deny":
				failClosed = true
			case "allow":
				failClosed = false
			default:
				rt.loadErr = fmt.Errorf("guard onError must be deny or allow")
				return goja.Undefined()
			}
		}
	}
	rt.hooks.guards[kind] = append(rt.hooks.guards[kind], guardEntry{
		fn: rt.wrapGuard(fn, d), timeout: d, failClosed: failClosed,
	})
	return goja.Undefined()
}

func (rt *runtime) wrapEvent(fn goja.Callable, d time.Duration) eventFn {
	return func(ctx context.Context, p EventPayload) error {
		_, err := rt.withTimeout(d, func() (goja.Value, error) {
			return fn(goja.Undefined(), rt.vm.ToValue(payloadMap(p)))
		})
		if err != nil {
			rt.record(err)
		}
		return nil
	}
}

func (rt *runtime) wrapGuard(fn goja.Callable, d time.Duration) guardFn {
	return func(ctx context.Context, p EventPayload) (GuardDecision, error) {
		val, err := rt.withTimeout(d, func() (goja.Value, error) {
			return fn(goja.Undefined(), rt.vm.ToValue(payloadMap(p)))
		})
		if err != nil {
			rt.record(err)
			return GuardDecision{Allow: false}, err
		}
		if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
			return GuardDecision{Allow: true}, nil
		}
		if b, ok := val.Export().(bool); ok {
			return GuardDecision{Allow: b}, nil
		}
		obj, ok := val.Export().(map[string]any)
		if !ok {
			return GuardDecision{Allow: false, Message: "guard returned an invalid response"}, nil
		}
		allow, _ := obj["allow"].(bool)
		msg, _ := obj["message"].(string)
		code, _ := obj["code"].(string)
		return GuardDecision{Allow: allow, Message: msg, Code: code}, nil
	}
}
