package mw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"moviehub/auth"
)

func TestRequireLogin(t *testing.T) {
	issuer := &auth.Issuer{Secret: []byte("test-only-secret-for-middleware-123456"), TTL: time.Hour}
	token, err := issuer.Issue(42, "member")
	if err != nil {
		t.Fatal(err)
	}
	expiredIssuer := &auth.Issuer{Secret: issuer.Secret, TTL: -time.Minute}
	expired, err := expiredIssuer.Issue(42, "member")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, header, code string
		status             int
	}{
		{"ไม่มีตั๋ว", "", "LOGIN_REQUIRED", http.StatusUnauthorized},
		{"ไม่มี Bearer", token, "LOGIN_REQUIRED", http.StatusUnauthorized},
		{"ตั๋วว่าง", "Bearer ", "LOGIN_REQUIRED", http.StatusUnauthorized},
		{"ตั๋วเสีย", "Bearer " + token + "x", "TOKEN_INVALID", http.StatusUnauthorized},
		{"ตั๋วหมดอายุ", "Bearer " + expired, "TOKEN_INVALID", http.StatusUnauthorized},
		{"ตั๋วจริง", "Bearer " + token, "", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := func(w http.ResponseWriter, r *http.Request) {
				called = true
				if who := WhoFrom(r.Context()); who != (Who{ID: 42, Role: "member"}) {
					t.Fatalf("ตัวตนใน context ไม่ถูกต้อง: %+v", who)
				}
				w.WriteHeader(http.StatusNoContent)
			}
			r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			RequireLogin(issuer, next)(w, r)
			if w.Code != tc.status || called != (tc.status == http.StatusNoContent) {
				t.Fatalf("status=%d called=%v body=%s", w.Code, called, w.Body.String())
			}
			if tc.code != "" {
				var body struct {
					Error struct{ Code, Message string }
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.Code != tc.code || body.Error.Message == "" {
					t.Fatalf("error ไม่ถูกต้อง: %s", w.Body.String())
				}
			}
		})
	}
}
