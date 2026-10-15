package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"moviehub/auth"
	"moviehub/handler"
	"moviehub/mw"
	"moviehub/store"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatal("อ่านไฟล์ .env ไม่ได้: ", err)
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		data, err := os.ReadFile(".jwt_secret")
		if err != nil {
			log.Fatal("โหลด JWT secret ไม่ได้: ตั้งค่า JWT_SECRET หรือสร้างไฟล์ .jwt_secret: ", err)
		}
		secret = strings.TrimSpace(string(data))
	}
	if len(secret) <= 32 {
		log.Fatal("JWT_SECRET ต้องยาวกว่า 32 ตัวอักษร")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด ", err)
	}

	tickets := &auth.Issuer{Secret: []byte(secret), TTL: time.Hour}
	h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/movies", h.SearchMovies)
	mux.HandleFunc("GET /api/movies/{id}", h.GetMovie)
	mux.HandleFunc("GET /api/movies/{id}/reviews", h.ListReviews)

	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port
	log.Println("MovieHub API ฟังอยู่ที่", addr, "ต้อนรับ", origins)
	log.Fatal(http.ListenAndServe(addr, mw.Logging(mw.CORS(origins, mux))))
}
