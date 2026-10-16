package handler

import "net/http"

func (h *Handler) AdminMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.Store.ListMembers(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": members})
}
