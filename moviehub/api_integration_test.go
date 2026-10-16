package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"moviehub/auth"
	"moviehub/handler"
	"moviehub/mw"
	"moviehub/store"
)

// ใช้ schema ชั่วคราวในฐานข้อมูลทดสอบ ไม่แตะตารางของแอปที่ใช้งานอยู่
func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ตั้ง TEST_DATABASE_URL เพื่อทดสอบกับ PostgreSQL จริง")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("moviehub_test_%d", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("ลบ schema ทดสอบไม่ได้: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	sql, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO movies (id, title, title_th, original_title, original_language, release_date, vote_average, popularity)
		VALUES (1423191, 'Resident Evil', 'เรสซิเดนท์ อีวิล', 'Resident Evil', 'en', '2026-01-01', 7.3, 100),
		       (460458, 'Resident Evil: Welcome to Raccoon City', NULL, 'Resident Evil: Welcome to Raccoon City', 'en', '2022-01-01', 5.9, 50);
		INSERT INTO movie_genres (movie_id, genre_id, position) VALUES (1423191, 27, 0), (460458, 28, 0)`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func readResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatalf("อ่านคำตอบไม่ได้: %v: %s", err, w.Body.String())
	}
	return value
}

func TestAPIIntegration(t *testing.T) {
	pool := integrationPool(t)
	issuer := &auth.Issuer{Secret: []byte("test-only-secret-for-integration-123456"), TTL: time.Hour}
	h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: issuer}
	router := mw.CORS([]string{"http://localhost:3000"}, newMux(h))
	request := func(t *testing.T, method, path, token, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: ต้องการ %d ได้ %d: %s", method, path, want, w.Code, w.Body.String())
		}
		if want == http.StatusNoContent {
			if w.Body.Len() != 0 {
				t.Fatalf("204 ต้องไม่มีเนื้อหา: %s", w.Body.String())
			}
		} else {
			if w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) {
				t.Fatalf("คำตอบต้องเป็น JSON: %s", w.Body.String())
			}
			if want >= 400 {
				failure := readResponse[struct {
					Error struct{ Code, Message string }
				}](t, w)
				if failure.Error.Code == "" || failure.Error.Message == "" {
					t.Fatalf("ต้องมี error.code และ error.message: %s", w.Body.String())
				}
			}
		}
		return w
	}
	login := func(t *testing.T, email, password string) string {
		t.Helper()
		body, err := json.Marshal(map[string]string{"email": email, "password": password})
		if err != nil {
			t.Fatal(err)
		}
		w := request(t, "POST", "/api/auth/login", "", string(body), 200)
		token := readResponse[struct{ Token string }](t, w).Token
		if token == "" {
			t.Fatal("ต้องได้รับ JWT หลังเข้าสู่ระบบ")
		}
		return token
	}

	var nida, ton, staff store.Member
	var nidaToken, tonToken, oldStaffToken, staffToken string
	t.Run("registration and login", func(t *testing.T) {
		w := request(t, "POST", "/api/auth/register", "", `{"email":" NIDA@example.com ","password":"popcorn2569","displayName":" นิดา ","role":"admin"}`, 201)
		nida = readResponse[store.Member](t, w)
		if nida.Email != "nida@example.com" || nida.DisplayName != "นิดา" || nida.Role != "member" || strings.Contains(w.Body.String(), "password") {
			t.Fatalf("ข้อมูลสมาชิกไม่ถูกต้องหรือมีรหัสผ่านรั่ว: %s", w.Body.String())
		}
		request(t, "POST", "/api/auth/register", "", `{"email":"NIDA@example.com","password":"another123","displayName":"นิดาปลอม"}`, 409)
		request(t, "POST", "/api/auth/register", "", `{"email":"ton@example.com","password":"1234","displayName":"ต้น"}`, 400)
		request(t, "POST", "/api/auth/register", "", fmt.Sprintf(`{"email":"long@example.com","password":%q,"displayName":"ยาว"}`, strings.Repeat("ป๊อปคอร์น", 3)), 400)
		request(t, "POST", "/api/auth/register", "", `{"email":"invalid","password":"popcorn2569","displayName":"นิดา"}`, 400)
		request(t, "POST", "/api/auth/register", "", `{"email":"blank@example.com","password":"popcorn2569","displayName":" "}`, 400)
		request(t, "POST", "/api/auth/register", "", `{`, 400)
		var hash string
		if err := pool.QueryRow(t.Context(), "SELECT password_hash FROM members WHERE id = $1", nida.ID).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(hash, "$2a$10$") || !auth.CheckPassword(hash, "popcorn2569") {
			t.Fatal("ต้องเก็บรหัสผ่านเป็น bcrypt cost 10")
		}
		wrong := request(t, "POST", "/api/auth/login", "", `{"email":"nida@example.com","password":"wrongpass"}`, 401)
		missing := request(t, "POST", "/api/auth/login", "", `{"email":"missing@example.com","password":"wrongpass"}`, 401)
		if wrong.Body.String() != missing.Body.String() {
			t.Fatal("อีเมลที่ไม่มีอยู่และรหัสผิดต้องได้ข้อความเดียวกัน")
		}
		nidaToken = login(t, " NIDA@example.com ", "popcorn2569")
		ton = readResponse[store.Member](t, request(t, "POST", "/api/auth/register", "", `{"email":"ton@example.com","password":"sunflower99","displayName":"ต้น"}`, 201))
		staff = readResponse[store.Member](t, request(t, "POST", "/api/auth/register", "", `{"email":"staff@example.com","password":"staffpass2569","displayName":"พี่หน้าโรง"}`, 201))
		tonToken = login(t, "ton@example.com", "sunflower99")
		oldStaffToken = login(t, "staff@example.com", "staffpass2569")
	})
	if nidaToken == "" || tonToken == "" || oldStaffToken == "" {
		t.Fatal("สร้างบัญชีทดสอบไม่สำเร็จ")
	}

	t.Run("movies and identity", func(t *testing.T) {
		movies := readResponse[struct{ Items []store.Movie }](t, request(t, "GET", "/api/movies?q=evil", "", "", 200))
		if len(movies.Items) != 2 || movies.Items[0].Genre == nil || *movies.Items[0].Genre != "สยองขวัญ" || movies.Items[0].Rating != 7.3 {
			t.Fatalf("ข้อมูลหนังไม่ถูกต้อง: %+v", movies)
		}
		request(t, "GET", "/api/movies/1423191", "", "", 200)
		request(t, "GET", "/api/movies/999", "", "", 404)
		request(t, "GET", "/api/movies/abc/reviews", "", "", 400)
		if w := request(t, "GET", "/api/movies/1423191/reviews", "", "", 200); w.Body.String() != "{\"items\":[]}\n" {
			t.Fatalf("รายการว่างต้องเป็น []: %s", w.Body.String())
		}
		me := readResponse[store.Member](t, request(t, "GET", "/api/me", nidaToken, "", 200))
		if me.ID != nida.ID || me.Email != nida.Email {
			t.Fatalf("ตัวตนไม่ตรงกับ JWT: %+v", me)
		}
		request(t, "GET", "/api/me", nidaToken+"x", "", 401)
	})

	t.Run("reviews and admin permissions", func(t *testing.T) {
		request(t, "POST", "/api/movies/1423191/reviews", nidaToken, `{"text":"สนุกมาก"}`, 400)
		request(t, "POST", "/api/movies/1423191/reviews", nidaToken, `{`, 400)
		request(t, "POST", "/api/movies/999/reviews", nidaToken, `{"text":"หนังเรื่องนี้ไม่มีอยู่จริงแน่ๆ"}`, 404)
		w := request(t, "POST", "/api/movies/1423191/reviews", nidaToken, fmt.Sprintf(`{"text":"ตื่นเต้นตั้งแต่ฉากแรกจนจบ ซอมบี้ทำถึงมาก","memberId":%d}`, ton.ID), 201)
		review := readResponse[store.Review](t, w)
		request(t, "POST", "/api/movies/1423191/reviews", nidaToken, `{"text":"ขอรีวิวอีกรอบนะ ดูซ้ำแล้วชอบกว่าเดิม"}`, 409)
		items := readResponse[struct{ Items []store.Review }](t, request(t, "GET", "/api/movies/1423191/reviews", "", "", 200)).Items
		if len(items) != 1 || items[0].MemberID != nida.ID || items[0].Member.DisplayName != "นิดา" {
			t.Fatalf("ต้องใช้เจ้าของตั๋วเป็นผู้เขียน: %+v", items)
		}
		path := "/api/reviews/" + strconv.FormatInt(review.ID, 10)
		request(t, "DELETE", path, tonToken, "", 404)
		request(t, "GET", "/api/admin/members", tonToken, "", 403)
		if _, err := pool.Exec(t.Context(), "UPDATE members SET role = 'admin' WHERE id = $1", staff.ID); err != nil {
			t.Fatal(err)
		}
		request(t, "GET", "/api/admin/members", oldStaffToken, "", 403)
		staffToken = login(t, "staff@example.com", "staffpass2569")
		members := readResponse[struct{ Items []store.Member }](t, request(t, "GET", "/api/admin/members", staffToken, "", 200)).Items
		if len(members) != 3 || members[2].Role != "admin" {
			t.Fatalf("รายชื่อสมาชิกไม่ถูกต้อง: %+v", members)
		}
		tonReview := readResponse[store.Review](t, request(t, "POST", "/api/movies/1423191/reviews", tonToken, `{"text":"ภาพสวยแต่เรื่องเดาง่ายไปหน่อย"}`, 201))
		request(t, "DELETE", "/api/reviews/"+strconv.FormatInt(tonReview.ID, 10), staffToken, "", 204)
		request(t, "DELETE", path, nidaToken, "", 204)
		request(t, "DELETE", path, nidaToken, "", 404)
	})

	t.Run("wishlist isolation and upsert", func(t *testing.T) {
		first := request(t, "PUT", "/api/me/wishlist/1423191", nidaToken, `{"note":"ดูกับเพื่อน","memberId":999}`, 200)
		repeated := request(t, "PUT", "/api/me/wishlist/1423191", nidaToken, `{"note":"ดูกับเพื่อน"}`, 200)
		if first.Body.String() != repeated.Body.String() {
			t.Fatal("PUT ซ้ำต้องได้ข้อมูลเดิมและ addedAt เดิม")
		}
		request(t, "PUT", "/api/me/wishlist/460458", nidaToken, "", 200)
		items := readResponse[struct{ Items []store.WishlistItem }](t, request(t, "GET", "/api/me/wishlist", nidaToken, "", 200)).Items
		if len(items) != 2 || items[0].Movie.ID != items[0].MovieID {
			t.Fatalf("รายการอยากดูไม่ถูกต้อง: %+v", items)
		}
		request(t, "DELETE", "/api/me/wishlist/1423191", tonToken, "", 204)
		other := request(t, "GET", "/api/me/wishlist", tonToken, "", 200)
		if other.Body.String() != "{\"items\":[]}\n" {
			t.Fatalf("ต้องไม่เห็น wishlist ของคนอื่น: %s", other.Body.String())
		}
		items = readResponse[struct{ Items []store.WishlistItem }](t, request(t, "GET", "/api/me/wishlist", nidaToken, "", 200)).Items
		if len(items) != 2 {
			t.Fatal("บัญชีอื่นต้องลบ wishlist ของนิดาไม่ได้")
		}
		updated := readResponse[store.WishlistEntry](t, request(t, "PUT", "/api/me/wishlist/1423191", nidaToken, `{"note":"เปลี่ยนโน้ต"}`, 200))
		if updated.Note == nil || *updated.Note != "เปลี่ยนโน้ต" {
			t.Fatalf("ต้องแก้โน้ตได้: %+v", updated)
		}
		request(t, "PUT", "/api/me/wishlist/999", nidaToken, "", 404)
		request(t, "PUT", "/api/me/wishlist/1423191", nidaToken, `{`, 400)
		request(t, "PUT", "/api/me/wishlist/1423191", nidaToken, fmt.Sprintf(`{"note":%q}`, strings.Repeat("ก", 201)), 400)
		request(t, "DELETE", "/api/me/wishlist/1423191", nidaToken, "", 204)
		request(t, "DELETE", "/api/me/wishlist/1423191", nidaToken, "", 204)
	})

	t.Run("votes and member isolation", func(t *testing.T) {
		request(t, "PUT", "/api/movies/1423191/vote", nidaToken, `{"score":0}`, 400)
		request(t, "PUT", "/api/movies/1423191/vote", nidaToken, `{"score":11}`, 400)
		request(t, "PUT", "/api/movies/1423191/vote", nidaToken, `{"score":8.5}`, 400)
		request(t, "PUT", "/api/movies/999/vote", nidaToken, `{"score":8}`, 404)
		vote := readResponse[store.Vote](t, request(t, "PUT", "/api/movies/1423191/vote", nidaToken, `{"score":8,"memberId":999}`, 200))
		if vote.MovieID != 1423191 || vote.Score != 8 {
			t.Fatalf("คะแนนไม่ถูกต้อง: %+v", vote)
		}
		request(t, "PUT", "/api/movies/1423191/vote", nidaToken, `{"score":10}`, 200)
		request(t, "PUT", "/api/movies/1423191/vote", tonToken, `{"score":1}`, 200)
		request(t, "POST", "/api/movies/1423191/reviews", nidaToken, `{"text":"คะแนนเต็มเพราะชอบมากจริงๆ"}`, 201)
		items := readResponse[struct{ Items []store.Review }](t, request(t, "GET", "/api/movies/1423191/reviews", "", "", 200)).Items
		if len(items) != 1 || items[0].Score == nil || *items[0].Score != 10 {
			t.Fatalf("รีวิวต้องแนบคะแนนของผู้เขียน: %+v", items)
		}
		request(t, "DELETE", "/api/movies/1423191/vote", tonToken, "", 204)
		var score, count int
		if err := pool.QueryRow(t.Context(), "SELECT score FROM votes WHERE member_id = $1 AND movie_id = 1423191", nida.ID).Scan(&score); err != nil || score != 10 {
			t.Fatalf("บัญชีอื่นต้องลบคะแนนของนิดาไม่ได้: score=%d err=%v", score, err)
		}
		request(t, "DELETE", "/api/movies/1423191/vote", nidaToken, "", 204)
		request(t, "DELETE", "/api/movies/1423191/vote", nidaToken, "", 204)
		if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM votes").Scan(&count); err != nil || count != 0 {
			t.Fatalf("คะแนนต้องถูกลบ: count=%d err=%v", count, err)
		}
	})
}
