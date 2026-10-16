package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("กุญแจลับสำหรับทดสอบเท่านั้น-ยาวพอแน่นอน")

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestForgedTickets(t *testing.T) {
	issuer := &Issuer{Secret: testSecret, TTL: time.Hour}
	genuine, err := issuer.Issue(1, "member")
	if err != nil {
		t.Fatal(err)
	}
	if id, role, err := issuer.Parse(genuine); err != nil || id != 1 || role != "member" {
		t.Fatalf("ตั๋วจริงต้องผ่าน: id=%d role=%q err=%v", id, role, err)
	}
	now := time.Now()
	good := jwt.MapClaims{"sub": "1", "role": "admin", "exp": now.Add(time.Hour).Unix()}
	parts := strings.Split(genuine, ".")
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), "member", "admin", 1))
	edited := parts[0] + "." + base64.RawURLEncoding.EncodeToString(body) + "." + parts[2]
	cases := []struct {
		name  string
		token string
	}{
		{"แก้เนื้อความ", edited},
		{"เซ็นด้วยกุญแจอื่น", sign(t, jwt.SigningMethodHS256, []byte("กุญแจของคนอื่น-ยาวพอเหมือนกันนะ"), good)},
		{"อ้างว่าไม่ต้องเซ็น", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good)},
		{"กุญแจถูกแต่วิธีเซ็นอื่น", sign(t, jwt.SigningMethodHS512, testSecret, good)},
		{"ไม่มีวันหมดอายุ", sign(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{"sub": "1", "role": "admin"})},
		{"หมดอายุแล้ว", sign(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{"sub": "1", "role": "member", "exp": now.Add(-time.Minute).Unix()})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if id, role, err := issuer.Parse(tc.token); err == nil || id != 0 || role != "" {
				t.Fatalf("ตั๋วปลอมต้องถูกปฏิเสธ: id=%d role=%q err=%v", id, role, err)
			}
		})
	}
}

func TestInvalidSubject(t *testing.T) {
	issuer := &Issuer{Secret: testSecret, TTL: time.Hour}
	for _, subject := range []string{"", "abc", "0", "-1", "9223372036854775808"} {
		t.Run(subject, func(t *testing.T) {
			token := sign(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{
				"sub": subject, "role": "member", "exp": time.Now().Add(time.Hour).Unix(),
			})
			if _, _, err := issuer.Parse(token); err == nil {
				t.Fatal("เลขสมาชิกที่ไม่ถูกต้องต้องถูกปฏิเสธ")
			}
		})
	}
}
