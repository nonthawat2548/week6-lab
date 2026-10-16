package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"moviehub/mw"
	"moviehub/store"
)

func (h *Handler) ListWishlist(w http.ResponseWriter, r *http.Request) {
	who := mw.WhoFrom(r.Context())
	items, err := h.Store.ListWishlist(r.Context(), who.ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type wishlistReq struct {
	Note *string `json:"note"`
}

func (h *Handler) PutWishlist(w http.ResponseWriter, r *http.Request) {
	movieID, ok := pathID(r, "movieId")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
		return
	}
	var req wishlistReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
		return
	}
	if req.Note != nil && utf8.RuneCountInString(*req.Note) > 200 {
		writeError(w, http.StatusBadRequest, "BAD_NOTE", "โน้ตต้องยาวไม่เกิน 200 ตัวอักษร")
		return
	}
	who := mw.WhoFrom(r.Context())
	item, err := h.Store.PutWishlist(r.Context(), who.ID, movieID, req.Note)
	switch {
	case errors.Is(err, store.ErrNoMovie):
		writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
		return
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
		return
	case err != nil:
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteWishlist(w http.ResponseWriter, r *http.Request) {
	movieID, ok := pathID(r, "movieId")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
		return
	}
	who := mw.WhoFrom(r.Context())
	if err := h.Store.DeleteWishlist(r.Context(), who.ID, movieID); err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
