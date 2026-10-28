package plugin

import "context"

type actorKey struct{}

// WithActor carries the real admin uid into host lifecycle/config audits.
func WithActor(ctx context.Context, uid int64) context.Context {
	return context.WithValue(ctx, actorKey{}, uid)
}

// ActorFromContext returns the acting uid (0 = system/background/token path).
func ActorFromContext(ctx context.Context) int64 {
	uid, _ := ctx.Value(actorKey{}).(int64)
	return uid
}
