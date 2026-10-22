// userapi.go is the session-authenticated "authorized apps" surface: list the
// caller's live OIDC grants and revoke them. Mounted under the existing
// /api/v1/oidc/ DeviceAPIHandler mux (single ServeMux, exact-path patterns).
package oidc

import (
	"encoding/json"
	"net/http"
	"time"
)

// GET /api/v1/oidc/tokens — the caller's live (non-revoked, non-expired)
// access-token ledger entries, newest first.
func (p *Provider) listTokens(w http.ResponseWriter, r *http.Request) {
	uid, err := p.sessionUser(r)
	if err != nil {
		unauthorized(w)
		return
	}
	recs, err := p.backend.ListUserAccessTokens(r.Context(), uid)
	if err != nil {
		writeJSON(w, map[string]any{"error": "查询失败"})
		return
	}
	type item struct {
		JTI         string    `json:"jti"`
		ClientID    string    `json:"client_id"`
		Scopes      []string  `json:"scopes"`
		SelectedPID int64     `json:"selected_pid"`
		CreatedAt   time.Time `json:"created_at"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	out := make([]item, 0, len(recs))
	for _, rec := range recs {
		out = append(out, item{
			JTI:         rec.JTI,
			ClientID:    rec.ClientID,
			Scopes:      rec.Scopes,
			SelectedPID: rec.SelectedPID,
			CreatedAt:   rec.CreatedAt,
			ExpiresAt:   rec.ExpiresAt,
		})
	}
	writeJSON(w, map[string]any{"tokens": out})
}

// POST /api/v1/oidc/tokens/revoke — body {"jti": "..."} (or
// {"all": true} to sign out everywhere). Single revocation kills the jti plus
// the caller's live refresh tokens for the same client (the refresh table has
// no jti column — uid+client is the narrowest granularity; the response
// reports that scope). {"all": true} revokes every access + refresh token.
func (p *Provider) revokeToken(w http.ResponseWriter, r *http.Request) {
	uid, err := p.sessionUser(r)
	if err != nil {
		unauthorized(w)
		return
	}
	var body struct {
		JTI string `json:"jti"`
		All bool   `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"error": "bad body"})
		return
	}
	if body.All {
		if err := p.backend.RevokeUserAccessTokens(r.Context(), uid); err != nil {
			writeJSON(w, map[string]any{"error": "吊销失败"})
			return
		}
		if err := p.backend.RevokeUserRefreshTokens(r.Context(), uid); err != nil {
			writeJSON(w, map[string]any{"error": "吊销失败"})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "scope": "all"})
		return
	}
	if body.JTI == "" {
		writeJSON(w, map[string]any{"error": "missing jti"})
		return
	}
	// Ownership check: never let one user revoke another user's jti.
	rec, err := p.backend.GetAccessToken(r.Context(), body.JTI)
	if err != nil || rec.UserID != uid {
		writeJSON(w, map[string]any{"error": "未找到该授权"})
		return
	}
	if err := p.backend.RevokeAccessToken(r.Context(), body.JTI); err != nil {
		writeJSON(w, map[string]any{"error": "吊销失败"})
		return
	}
	// Refresh table carries no jti: revoke the uid+client refresh scope.
	_ = p.backend.RevokeClientRefreshTokens(r.Context(), uid, rec.ClientID)
	writeJSON(w, map[string]any{"ok": true, "scope": "jti+client_refresh"})
}
