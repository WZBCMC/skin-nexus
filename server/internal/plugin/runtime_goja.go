package plugin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// errTimeout is pushed into the VM when a hook exceeds its budget.
var errTimeout = errors.New("plugin: hook timeout")

// typed hook function shapes (match the frozen contract callbacks).
type (
	profileHookFn   func(context.Context, *UserProfileFilter) error
	textureHookFn   func(context.Context, *TextureURLFilter) error
	uploadHookFn    func(context.Context, TextureUploadedEvent) error
	lifecycleHookFn func(context.Context, UserLifecycleEvent) error
	authHookFn      func(context.Context, AuthPhaseEvent) error
)

// timed pairs a hook with its per-call budget.
type timed[T any] struct {
	fn      T
	timeout time.Duration
}

// hookSet is the per-target (runtime or host-extra) registration storage.
type hookSet struct {
	profile   []timed[profileHookFn]
	texture   []timed[textureHookFn]
	upload    []timed[uploadHookFn]
	lifecycle []timed[lifecycleHookFn]
	auth      []timed[authHookFn]
}

// routeEntry is one plugin-contributed read-only route.
type routeEntry struct {
	method  string // GET | HEAD
	path    string // cleaned, leading "/"
	fn      goja.Callable
	timeout time.Duration
}

// runtime owns a single plugin's goja VM and its registrations. goja is not
// goroutine-safe, so every VM interaction happens under mu.
type runtime struct {
	info Info
	vm   *goja.Runtime
	mu   sync.Mutex

	hooks  hookSet
	routes []routeEntry

	lastErr string
	logs    []string

	// loadErr captures registration-time failures (bad permission, bad route)
	// so RunString can surface them as a load failure.
	loadErr error

	defaults timeouts
	logSink  func(string)
}

type timeouts struct {
	hook, auth, route time.Duration
}

func newRuntime(info Info, t timeouts, logSink func(string)) (*runtime, error) {
	src, err := os.ReadFile(info.ScriptPath)
	if err != nil {
		return nil, fmt.Errorf("read entrypoint: %w", err)
	}
	rt := &runtime{info: info, vm: goja.New(), defaults: t, logSink: logSink}
	rt.installAPI()
	if _, err := rt.vm.RunString(string(src)); err != nil {
		return nil, fmt.Errorf("run entrypoint: %w", err)
	}
	if rt.loadErr != nil {
		return nil, rt.loadErr
	}
	return rt, nil
}

// installAPI exposes the global `skin` object plugins program against.
func (rt *runtime) installAPI() {
	rt.vm.Set("skin", map[string]interface{}{
		"hooks": map[string]interface{}{
			"onUserProfileFilter": rt.regHook(hookProfile),
			"onTextureURLFilter":  rt.regHook(hookTexture),
			"onTextureUploaded":   rt.regHook(hookUpload),
			"onUserLifecycle":     rt.regHook(hookLifecycle),
			"onAuthPhase":         rt.regHook(hookAuth),
		},
		"route": rt.regRoute,
		"log":   rt.jsLog,
	})
}

// hook kinds
const (
	hookProfile = iota
	hookTexture
	hookUpload
	hookLifecycle
	hookAuth
)

func hookPermission(kind int) string {
	switch kind {
	case hookProfile:
		return PermReadProfiles
	case hookTexture:
		return PermReadTextures
	case hookUpload:
		return PermEditTextures
	case hookLifecycle, hookAuth:
		return PermAuthHooks
	}
	return ""
}

func (rt *runtime) defaultTimeout(kind int) time.Duration {
	if kind == hookAuth {
		return rt.defaults.auth
	}
	return rt.defaults.hook
}

// regHook returns the JS-facing registration function for one hook kind.
func (rt *runtime) regHook(kind int) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		cb, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			rt.loadErr = errors.New("hook callback must be a function")
			return goja.Undefined()
		}
		perm := hookPermission(kind)
		if perm != "" && !rt.info.hasPermission(perm) {
			rt.loadErr = fmt.Errorf("missing permission %q for hook", perm)
			return goja.Undefined()
		}
		d := rt.defaultTimeout(kind)
		if ms := optTimeoutMS(rt.vm, call.Argument(1)); ms > 0 {
			if td := time.Duration(ms) * time.Millisecond; td < d {
				d = td // plugins may tighten, never loosen
			}
		}
		opts := HostOptions{TimeoutMS: int(d.Milliseconds())}
		switch kind {
		case hookProfile:
			_ = rt.OnUserProfileFilter(rt.wrapProfile(cb, d), opts)
		case hookTexture:
			_ = rt.OnTextureURLFilter(rt.wrapTexture(cb, d), opts)
		case hookUpload:
			_ = rt.OnTextureUploaded(rt.wrapUpload(cb, d), opts)
		case hookLifecycle:
			_ = rt.OnUserLifecycle(rt.wrapLifecycle(cb, d), opts)
		case hookAuth:
			_ = rt.OnAuthPhase(rt.wrapAuth(cb, d), opts)
		}
		return goja.Undefined()
	}
}

func optTimeoutMS(vm *goja.Runtime, v goja.Value) int {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return 0
	}
	obj := v.ToObject(vm)
	if obj == nil {
		return 0
	}
	tv := obj.Get("timeoutMS")
	if tv == nil || goja.IsUndefined(tv) {
		return 0
	}
	return int(tv.ToInteger())
}

// withTimeout runs a VM closure under an interrupt deadline.
func (rt *runtime) withTimeout(d time.Duration, run func() (goja.Value, error)) (goja.Value, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.vm == nil {
		return nil, errors.New("plugin: runtime closed")
	}
	timer := time.AfterFunc(d, func() { rt.vm.Interrupt(errTimeout) })
	defer func() { timer.Stop(); rt.vm.ClearInterrupt() }()
	return run()
}

func (rt *runtime) record(err error) {
	msg := err.Error()
	if len(msg) > 512 {
		msg = msg[:512]
	}
	rt.lastErr = msg
	rt.logs = appendBounded(rt.logs, "[error] "+msg)
	if rt.logSink != nil {
		rt.logSink(fmt.Sprintf("plugin %s: %s", rt.info.Name, msg))
	}
}

func appendBounded(s []string, v string) []string {
	s = append(s, v)
	if len(s) > 64 {
		s = s[len(s)-64:]
	}
	return s
}

// wrapProfile converts a JS filter into the contract's profile-filter shape.
func (rt *runtime) wrapProfile(cb goja.Callable, d time.Duration) profileHookFn {
	return func(ctx context.Context, p *UserProfileFilter) error {
		val, err := rt.withTimeout(d, func() (goja.Value, error) {
			return cb(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
				"playerId":   p.PlayerID,
				"playerName": p.PlayerName,
				"uuid":       p.UUID,
				"textures": map[string]interface{}{
					"SKIN": p.TexturesURL["SKIN"],
					"CAPE": p.TexturesURL["CAPE"],
				},
			}))
		})
		_ = ctx
		if err != nil {
			rt.record(err)
			return nil // fail-open
		}
		if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
			return nil
		}
		obj, ok := val.Export().(map[string]interface{})
		if !ok {
			return nil
		}
		if p.TexturesURL == nil {
			p.TexturesURL = map[string]string{}
		}
		for _, k := range []string{"SKIN", "CAPE"} {
			rv, present := obj[k]
			if !present {
				continue
			}
			s, isStr := rv.(string)
			if !isStr {
				continue
			}
			if s == "" {
				delete(p.TexturesURL, k)
			} else {
				p.TexturesURL[k] = s
			}
		}
		return nil
	}
}

// wrapTexture converts a JS URL filter into the contract's texture-filter shape.
func (rt *runtime) wrapTexture(cb goja.Callable, d time.Duration) textureHookFn {
	return func(ctx context.Context, t *TextureURLFilter) error {
		val, err := rt.withTimeout(d, func() (goja.Value, error) {
			return cb(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
				"hash": t.Hash, "url": t.URL, "type": t.Type,
			}))
		})
		_ = ctx
		if err != nil {
			rt.record(err)
			return nil
		}
		if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
			return nil
		}
		if s, isStr := val.Export().(string); isStr && s != "" {
			t.URL = s
		}
		return nil
	}
}

func (rt *runtime) wrapUpload(cb goja.Callable, d time.Duration) uploadHookFn {
	return func(ctx context.Context, e TextureUploadedEvent) error {
		_, err := rt.withTimeout(d, func() (goja.Value, error) {
			return cb(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
				"userId": e.UserID, "playerId": e.PlayerID, "hash": e.Hash, "type": e.Type,
			}))
		})
		_ = ctx
		if err != nil {
			rt.record(err)
		}
		return nil
	}
}

func (rt *runtime) wrapLifecycle(cb goja.Callable, d time.Duration) lifecycleHookFn {
	return func(ctx context.Context, e UserLifecycleEvent) error {
		_, err := rt.withTimeout(d, func() (goja.Value, error) {
			return cb(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
				"kind": e.Kind, "userId": e.UserID,
			}))
		})
		_ = ctx
		if err != nil {
			rt.record(err)
		}
		return nil
	}
}

func (rt *runtime) wrapAuth(cb goja.Callable, d time.Duration) authHookFn {
	return func(ctx context.Context, e AuthPhaseEvent) error {
		_, err := rt.withTimeout(d, func() (goja.Value, error) {
			return cb(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
				"phase": e.Phase, "uuid": e.UUID, "player": e.Player,
			}))
		})
		_ = ctx
		if err != nil {
			rt.record(err)
		}
		return nil
	}
}

// regRoute validates and stores a plugin-contributed route.
func (rt *runtime) regRoute(call goja.FunctionCall) goja.Value {
	method := strings.ToUpper(strings.TrimSpace(call.Argument(0).String()))
	rawPath := call.Argument(1).String()
	cb, ok := goja.AssertFunction(call.Argument(2))
	if method != "GET" && method != "HEAD" {
		rt.loadErr = fmt.Errorf("route method %q not allowed (GET/HEAD only)", method)
		return goja.Undefined()
	}
	clean := path.Clean("/" + strings.TrimPrefix(rawPath, "/"))
	if clean == "/" || strings.Contains(clean, "..") {
		rt.loadErr = fmt.Errorf("invalid route path %q", rawPath)
		return goja.Undefined()
	}
	if !ok {
		rt.loadErr = errors.New("route handler must be a function")
		return goja.Undefined()
	}
	if !rt.info.hasPermission(PermRegisterRoute) {
		rt.loadErr = fmt.Errorf("missing permission %q for route", PermRegisterRoute)
		return goja.Undefined()
	}
	if len(rt.routes) >= maxRoutesPerPlugin {
		rt.loadErr = errors.New("too many routes")
		return goja.Undefined()
	}
	for _, r := range rt.routes {
		if r.method == method && r.path == clean {
			rt.loadErr = fmt.Errorf("duplicate route %s %s", method, clean)
			return goja.Undefined()
		}
	}
	rt.routes = append(rt.routes, routeEntry{method: method, path: clean, fn: cb, timeout: rt.defaults.route})
	return goja.Undefined()
}

// jsLog appends plugin log lines (bounded) and mirrors to the host logger.
func (rt *runtime) jsLog(call goja.FunctionCall) goja.Value {
	parts := make([]string, 0, len(call.Arguments))
	for _, a := range call.Arguments {
		parts = append(parts, a.String())
	}
	line := strings.Join(parts, " ")
	rt.logs = appendBounded(rt.logs, line)
	if rt.logSink != nil {
		rt.logSink(fmt.Sprintf("plugin %s: %s", rt.info.Name, line))
	}
	log.Printf("plugin %s: %s", rt.info.Name, line)
	return goja.Undefined()
}

// callRoute executes one stored route and converts the JS return value.
func (rt *runtime) callRoute(ctx context.Context, e routeEntry, q url.Values) (int, interface{}) {
	query := map[string]interface{}{}
	for k, vs := range q {
		if len(vs) > 0 {
			query[k] = vs[0]
		}
	}
	val, err := rt.withTimeout(e.timeout, func() (goja.Value, error) {
		return e.fn(goja.Undefined(), rt.vm.ToValue(map[string]interface{}{
			"method": e.method, "path": e.path, "query": query,
		}))
	})
	_ = ctx
	if err != nil {
		rt.record(err)
		return 503, map[string]interface{}{"error": "plugin route unavailable"}
	}
	if val == nil || goja.IsUndefined(val) {
		return 200, map[string]interface{}{}
	}
	exported := val.Export()
	if obj, ok := exported.(map[string]interface{}); ok {
		if st, hasStatus := obj["status"]; hasStatus {
			code := 200
			switch n := st.(type) {
			case int64:
				code = int(n)
			case float64:
				code = int(n)
			}
			return code, obj["body"]
		}
	}
	return 200, exported
}

// close releases the VM. Callers must ensure no in-flight calls remain.
func (rt *runtime) close() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.vm = nil
}
