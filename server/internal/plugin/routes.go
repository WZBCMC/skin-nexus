package plugin

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"
)

// maxRouteResponse caps a plugin route payload so a runaway plugin cannot
// stream unbounded bodies through the host.
const maxRouteResponse = 1 << 20

// Handler serves /api/plugins/<name>/<path> for enabled plugins. Routes are
// looked up per request from the live registry, so enable/disable takes
// effect immediately without touching the host ServeMux.
func (m *Manager) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/plugins/")
		name, sub, _ := strings.Cut(rest, "/")
		want := path.Clean("/" + sub)

		m.mu.RLock()
		inst, ok := m.instances[name]
		var (
			rt    *runtime
			entry routeEntry
			got   goRoute
			found bool
		)
		if ok && inst.enabled {
			if inst.rt != nil {
				inst.rt.mu.Lock()
				for _, e := range inst.rt.routes {
					if e.method == r.Method && e.path == want {
						entry, rt, found = e, inst.rt, true
						break
					}
				}
				inst.rt.mu.Unlock()
			}
			if !found {
				for _, g := range m.extraRoutes {
					if g.method == r.Method && g.path == want {
						got, found = g, true
						break
					}
				}
			}
		}
		m.mu.RUnlock()

		if !found {
			http.NotFound(w, r)
			return
		}

		ctx := r.Context()
		var (
			code int
			body interface{}
		)
		if rt != nil {
			code, body = rt.callRoute(ctx, entry, r.URL.Query())
		} else {
			query := map[string]interface{}{}
			for k, vs := range r.URL.Query() {
				if len(vs) > 0 {
					query[k] = vs[0]
				}
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				code, body = got.handler(ctx, map[string]interface{}{
					"method": got.method, "path": got.path, "query": query,
				})
			}()
			select {
			case <-done:
			case <-ctx.Done():
				http.Error(w, "plugin route unavailable", http.StatusServiceUnavailable)
				return
			}
		}

		raw, err := json.Marshal(body)
		if err != nil || len(raw) > maxRouteResponse {
			http.Error(w, "plugin response too large", http.StatusInsufficientStorage)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if r.Method == http.MethodGet {
			_, _ = w.Write(raw)
		}
	})
}
