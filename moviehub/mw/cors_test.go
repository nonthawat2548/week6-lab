package mw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "http://localhost:3001", ""} {
		t.Run(origin, func(t *testing.T) {
			called := false
			h := CORS([]string{" http://localhost:3000 ", ""}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusUnauthorized)
			}))
			for _, method := range []string{http.MethodOptions, http.MethodDelete} {
				called = false
				r := httptest.NewRequest(method, "/api/reviews/1", nil)
				r.Header.Set("Origin", origin)
				r.Header.Set("Access-Control-Request-Method", "DELETE")
				r.Header.Set("Access-Control-Request-Headers", "authorization")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if method == http.MethodOptions && (w.Code != http.StatusNoContent || called) {
					t.Fatalf("preflight ต้องผ่านก่อน login: status=%d called=%v", w.Code, called)
				}
				if method == http.MethodDelete && (w.Code != http.StatusUnauthorized || !called) {
					t.Fatalf("คำขอจริงต้องถึง handler: status=%d called=%v", w.Code, called)
				}
				if w.Header().Get("Vary") != "Origin" {
					t.Fatal("ต้องแยก cache ตาม Origin")
				}
				if origin == "http://localhost:3000" {
					if w.Header().Get("Access-Control-Allow-Origin") != origin ||
						!strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") ||
						!strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "DELETE") ||
						w.Header().Get("Access-Control-Max-Age") != "600" {
						t.Fatalf("หัวจดหมาย CORS ไม่ครบ: %v", w.Header())
					}
				} else {
					for name := range w.Header() {
						if strings.HasPrefix(name, "Access-Control-") {
							t.Fatalf("origin ที่ไม่ได้รับอนุญาตต้องไม่มีหัวจดหมาย %s", name)
						}
					}
				}
			}
		})
	}
}
