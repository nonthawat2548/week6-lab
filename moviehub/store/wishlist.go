package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type WishlistEntry struct {
	MovieID int64     `json:"movieId"`
	Note    *string   `json:"note"`
	AddedAt time.Time `json:"addedAt"`
}

type WishlistItem struct {
	WishlistEntry
	Movie Movie `json:"movie"`
}

func (s *Store) ListWishlist(ctx context.Context, memberID int64) ([]WishlistItem, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT w.movie_id, w.note, w.added_at,
		        m.id, m.title, m.title_th,
		        (SELECT coalesce(g.name_th, g.name_en) FROM movie_genres mg
		           JOIN genres g ON g.id = mg.genre_id
		          WHERE mg.movie_id = m.id AND mg.position = 0),
		        extract(year FROM m.release_date)::int,
		        round(m.vote_average, 1)::float8,
		        'https://image.tmdb.org/t/p/w500' || m.poster_path
		 FROM wishlist_items w
		 JOIN movies m ON m.id = w.movie_id
		 WHERE w.member_id = $1
		 ORDER BY w.added_at DESC, w.movie_id`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []WishlistItem{}
	for rows.Next() {
		var item WishlistItem
		if err := rows.Scan(&item.MovieID, &item.Note, &item.AddedAt,
			&item.Movie.ID, &item.Movie.Title, &item.Movie.TitleTh,
			&item.Movie.Genre, &item.Movie.Year, &item.Movie.Rating, &item.Movie.Poster); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// PutWishlist เพิ่มหรือแก้โน้ตของหนังเดิม โดยคง added_at เดิมเมื่อกดซ้ำ
func (s *Store) PutWishlist(ctx context.Context, memberID, movieID int64, note *string) (WishlistEntry, error) {
	var item WishlistEntry
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO wishlist_items (member_id, movie_id, note)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (member_id, movie_id) DO UPDATE SET note = EXCLUDED.note
		 RETURNING movie_id, note, added_at`, memberID, movieID, note).
		Scan(&item.MovieID, &item.Note, &item.AddedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if pgErr.ConstraintName == "wishlist_items_movie_id_fkey" {
			return WishlistEntry{}, ErrNoMovie
		}
		return WishlistEntry{}, ErrNotFound
	}
	return item, err
}

func (s *Store) DeleteWishlist(ctx context.Context, memberID, movieID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM wishlist_items WHERE member_id = $1 AND movie_id = $2`, memberID, movieID)
	return err
}
