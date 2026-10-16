package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrDuplicate = errors.New("duplicate")
	ErrNoMovie   = errors.New("movie not found")
)

type ReviewAuthor struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
}

type Review struct {
	ID        int64        `json:"id"`
	MemberID  int64        `json:"memberId"`
	MovieID   int64        `json:"movieId"`
	Text      string       `json:"text"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Member    ReviewAuthor `json:"member"`
	Score     *int16       `json:"score"`
}

func (s *Store) ListReviews(ctx context.Context, movieID int64) ([]Review, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.id, r.member_id, r.movie_id, r.text, r.created_at, r.updated_at,
                m.display_name, v.score
         FROM reviews r
         JOIN members m ON m.id = r.member_id
         LEFT JOIN votes v ON v.member_id = r.member_id AND v.movie_id = r.movie_id
         WHERE r.movie_id = $1
         ORDER BY r.created_at DESC`, movieID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.MemberID, &r.MovieID, &r.Text,
			&r.CreatedAt, &r.UpdatedAt, &r.Member.DisplayName, &r.Score); err != nil {
			return nil, err
		}
		r.Member.ID = r.MemberID
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddReview(ctx context.Context, memberID, movieID int64, text string) (Review, error) {
	r := Review{MemberID: memberID, MovieID: movieID, Text: text}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO reviews (member_id, movie_id, text)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`, memberID, movieID, text).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return Review{}, ErrDuplicate
		case "23503":
			if pgErr.ConstraintName == "reviews_movie_id_fkey" {
				return Review{}, ErrNoMovie
			}
			return Review{}, ErrNotFound
		}
	}
	return r, err
}

// ตรวจความเป็นเจ้าของในคำสั่ง DELETE เดียวกัน เพื่อไม่ให้ลบรีวิวของคนอื่น
func (s *Store) DeleteReview(ctx context.Context, reviewID, memberID int64, isAdmin bool) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM reviews WHERE id = $1 AND (member_id = $2 OR $3)`,
		reviewID, memberID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
