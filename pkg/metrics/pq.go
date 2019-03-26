package metrics

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// NewStore returns a store that provides an access to our database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type Store struct {
	db *sql.DB
}

// EnsureValidator ensures that a validator with a given public key is present
// in the database and has the memo set to given value.
// This operation may cause a validator inser or update.
func (s *Store) EnsureValidator(ctx context.Context, publicKey []byte, memo string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO validators (public_key, memo)
		VALUES ($1, $2)
			ON CONFLICT (public_key) DO UPDATE SET memo = $2
		RETURNING id
	`, publicKey, memo).Scan(&id)
	return id, err
}

func (s *Store) InsertBlock(ctx context.Context, height int64, hash []byte, created time.Time, proposerID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO blocks (block_height, block_hash, block_time, proposer_id)
		VALUES ($1, $2, $3, $4)
	`, height, hash, created, proposerID)
	return err
}

func (s *Store) MarkBlock(ctx context.Context, blockID, validatorID int64, validated bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO block_participations (block_id, validator_id, validated)
		VALUES ($1, $2, $3)
			ON CONFLICT (block_id, validator_id) DO UPDATE SET validated = $3
	`, blockID, validatorID, validated)
	return err
}
