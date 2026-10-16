package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type Vote struct {
	MovieID int64 `json:"movieId"`
	Score   int16 `json:"score"`
}

func (s *Store) PutVote(ctx context.Context, memberID, movieID int64, score int16) (Vote, error) {
	var vote Vote
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO votes (member_id, movie_id, score)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (member_id, movie_id)
		 DO UPDATE SET score = EXCLUDED.score, updated_at = now()
		 RETURNING movie_id, score`, memberID, movieID, score).
		Scan(&vote.MovieID, &vote.Score)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if pgErr.ConstraintName == "votes_movie_id_fkey" {
			return Vote{}, ErrNoMovie
		}
		return Vote{}, ErrNotFound
	}
	return vote, err
}

func (s *Store) DeleteVote(ctx context.Context, memberID, movieID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM votes WHERE member_id = $1 AND movie_id = $2`, memberID, movieID)
	return err
}
