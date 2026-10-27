package plugin

import (
	"context"
	"fmt"
	"time"
)

// clampTimeout tightens a plugin-requested budget: it may shorten the class
// default but never exceed it (the host owns the ceiling).
func clampTimeout(requestedMS int, def time.Duration) time.Duration {
	if requestedMS > 0 {
		if td := time.Duration(requestedMS) * time.Millisecond; td < def {
			return td
		}
	}
	return def
}

// The runtime satisfies HookBus so JS-registered callbacks land in the same
// typed storage the dispatcher reads. Timeouts are already baked into the
// wrapped closures, so opts only records intent here.
func (rt *runtime) OnUserProfileFilter(fn func(context.Context, *UserProfileFilter) error, _ HostOptions) error {
	rt.hooks.profile = append(rt.hooks.profile, timed[profileHookFn]{fn: fn})
	return nil
}

func (rt *runtime) OnTextureURLFilter(fn func(context.Context, *TextureURLFilter) error, _ HostOptions) error {
	rt.hooks.texture = append(rt.hooks.texture, timed[textureHookFn]{fn: fn})
	return nil
}

func (rt *runtime) OnTextureUploaded(fn func(context.Context, TextureUploadedEvent) error, _ HostOptions) error {
	rt.hooks.upload = append(rt.hooks.upload, timed[uploadHookFn]{fn: fn})
	return nil
}

func (rt *runtime) OnUserLifecycle(fn func(context.Context, UserLifecycleEvent) error, _ HostOptions) error {
	rt.hooks.lifecycle = append(rt.hooks.lifecycle, timed[lifecycleHookFn]{fn: fn})
	return nil
}

func (rt *runtime) OnAuthPhase(fn func(context.Context, AuthPhaseEvent) error, _ HostOptions) error {
	rt.hooks.auth = append(rt.hooks.auth, timed[authHookFn]{fn: fn})
	return nil
}

// RegisterRoute is JS-only in v1: Go callers use Manager.Bus for host-level
// hooks, while plugin routes come from skin.route inside the entrypoint.
func (rt *runtime) RegisterRoute(_ RouteRegistration) error {
	return fmt.Errorf("plugin %s: routes must be registered from JS via skin.route", rt.info.Name)
}

// goBus lets host-level (Go) code register hooks through Manager.Bus, mainly
// for tests and future first-party integrations. Timeouts are enforced by the
// dispatcher because Go callbacks carry no VM interrupt.
type goBus struct{ m *Manager }

func (b goBus) OnUserProfileFilter(fn func(context.Context, *UserProfileFilter) error, opts HostOptions) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.extra.profile = append(b.m.extra.profile, timed[profileHookFn]{fn, clampTimeout(opts.TimeoutMS, b.m.cfg.HookTimeout)})
	return nil
}

func (b goBus) OnTextureURLFilter(fn func(context.Context, *TextureURLFilter) error, opts HostOptions) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.extra.texture = append(b.m.extra.texture, timed[textureHookFn]{fn, clampTimeout(opts.TimeoutMS, b.m.cfg.HookTimeout)})
	return nil
}

func (b goBus) OnTextureUploaded(fn func(context.Context, TextureUploadedEvent) error, opts HostOptions) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.extra.upload = append(b.m.extra.upload, timed[uploadHookFn]{fn, clampTimeout(opts.TimeoutMS, b.m.cfg.HookTimeout)})
	return nil
}

func (b goBus) OnUserLifecycle(fn func(context.Context, UserLifecycleEvent) error, opts HostOptions) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.extra.lifecycle = append(b.m.extra.lifecycle, timed[lifecycleHookFn]{fn, clampTimeout(opts.TimeoutMS, b.m.cfg.HookTimeout)})
	return nil
}

func (b goBus) OnAuthPhase(fn func(context.Context, AuthPhaseEvent) error, opts HostOptions) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.extra.auth = append(b.m.extra.auth, timed[authHookFn]{fn, clampTimeout(opts.TimeoutMS, b.m.cfg.AuthTimeout)})
	return nil
}

func (b goBus) RegisterRoute(r RouteRegistration) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	if len(b.m.extraRoutes) >= maxRoutesTotal {
		return fmt.Errorf("too many host routes")
	}
	b.m.extraRoutes = append(b.m.extraRoutes, goRoute{
		method:  r.Method,
		path:    r.Path,
		handler: r.Handler,
		timeout: clampTimeout(optsTimeoutMS(r), b.m.cfg.RouteTimeout),
	})
	return nil
}

// optsTimeoutMS is unused for Go routes (HostOptions has no route budget);
// kept to document that route timeouts come from config.
func optsTimeoutMS(_ RouteRegistration) int { return 0 }
