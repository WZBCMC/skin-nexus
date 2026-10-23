package adminapi

import (
	"net/http"

	"skin-nexus/server/internal/store"
)

// ---- invitation codes (1:1 live blessing plugin admin surface) ----
// Legacy routes were GET list / POST generate / PUT edit under
// admin/invitation-codes; the SPA backend surface keeps the same verbs.

func (b *Backend) handleInvitationList(w http.ResponseWriter, r *http.Request) {
	free, used, err := b.Store.ListInvitationCodes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	if free == nil {
		free = []store.InvitationCode{}
	}
	if used == nil {
		used = []store.InvitationCode{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"free": free, "used": used})
}

func (b *Backend) handleInvitationGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount      int    `json:"amount"`
		Description string `json:"description"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid JSON"})
		return
	}
	if req.Amount == 0 {
		req.Amount = 1
	}
	codes, err := b.Store.CreateInvitationCodes(r.Context(), req.Amount, req.Description)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"codes": codes})
}

func (b *Backend) handleInvitationEdit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          int64  `json:"id"`
		Description string `json:"description"`
	}
	if err := jsonDecode(r, &req); err != nil || req.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "id and description required"})
		return
	}
	if err := b.Store.SetInvitationCodeDescription(r.Context(), req.ID, req.Description); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "invitation code not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
