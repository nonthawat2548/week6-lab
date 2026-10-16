package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"moviehub/auth"
	"moviehub/handler"
)

func TestProtectedRoutes(t *testing.T) {
	h := &handler.Handler{Tickets: &auth.Issuer{
		Secret: []byte("test-only-secret-for-routes-123456789"), TTL: time.Hour,
	}}
	mux := newMux(h)
	for _, route := range []string{
		"GET /api/me",
		"POST /api/movies/1/reviews",
		"DELETE /api/reviews/1",
		"GET /api/admin/members",
		"GET /api/me/wishlist",
		"PUT /api/me/wishlist/1",
		"DELETE /api/me/wishlist/1",
		"PUT /api/movies/1/vote",
		"DELETE /api/movies/1/vote",
	} {
		t.Run(route, func(t *testing.T) {
			method, path, _ := strings.Cut(route, " ")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("เส้นทางที่ต้องเข้าสู่ระบบตอบ %d: %s", w.Code, w.Body.String())
			}
		})
	}
	token, err := h.Tickets.Issue(1, "member")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/members", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("สมาชิกทั่วไปต้องได้ 403: %d", w.Code)
	}
}
