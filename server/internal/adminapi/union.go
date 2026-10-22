// union.go: admin surface of the MUA Union member module — signer status,
// rotation control. Dependencies are injected from main (the signer/API live
// in the union package; adminapi stays decoupled and 404s when unset).
package adminapi

import (
	"net/http"
)

// UnionDeps are wired from main when SKIN_UNION_ENABLED; nil = routes absent.
type UnionDeps struct {
	// Status returns the current signer/rotation view for the dashboard.
	Status func() map[string]any
	// ResumeRotation lifts the daily-switch suspension (operator action).
	ResumeRotation func() error
}

// MountUnion registers the union admin routes when deps are present.
func (b *Backend) MountUnion(deps UnionDeps) {
	b.union = deps
}

func (b *Backend) handleUnionStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, b.union.Status())
}

func (b *Backend) handleUnionRotationResume(w http.ResponseWriter, r *http.Request) {
	if err := b.union.ResumeRotation(); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
