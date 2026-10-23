package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"skin-nexus/server/internal/store"
)

// GET /api/v1/me/closet — the user's closet entries with texture metadata.
func (b *Backend) handleClosetList(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	rows, err := b.Store.ListCloset(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		item := map[string]any{
			"id": c.ID, "textureId": c.TextureID,
			"itemName": c.ItemName, "type": c.Type,
			"model": c.Model,
		}
		if c.Hash != "" {
			item["url"] = b.Store.TextureURL(c.Hash)
		} else {
			item["url"] = "" // orphan legacy row — visible, removable
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"closet": out})
}

// POST /api/v1/me/closet {textureId, itemName?} — idempotent add.
func (b *Backend) handleClosetAdd(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var req struct {
		TextureID int64  `json:"textureId"`
		ItemName  string `json:"itemName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TextureID == 0 {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := b.Store.AddClosetEntry(r.Context(), uid, req.TextureID, strings.TrimSpace(req.ItemName))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "纹理不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "insert failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "created": created})
}

// PUT /api/v1/me/closet/{id} {itemName} — rename the alias (legacy
// rename_closet_item_name).
func (b *Backend) handleClosetRename(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "bad id")
		return
	}
	var req struct {
		ItemName string `json:"itemName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := b.Store.SetClosetItemName(r.Context(), uid, id, strings.TrimSpace(req.ItemName)); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "条目不存在")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /api/v1/me/closet/{id} — remove one entry (legacy remove_closet_item).
func (b *Backend) handleClosetRemove(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "bad id")
		return
	}
	if err := b.Store.RemoveClosetEntry(r.Context(), uid, id); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "条目不存在")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
