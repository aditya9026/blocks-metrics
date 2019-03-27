package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/iov-one/block-metrics/pkg/errors"
	"github.com/lib/pq"
)

// NewStore returns a store that provides an access to our database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type Store struct {
	db *sql.DB
}

// EnsureValidator ensures that a validator with a given public key is present
// in the database.
func (s *Store) EnsureValidator(ctx context.Context, publicKey []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO validators (public_key)
		VALUES ($1)
			ON CONFLICT (public_key) DO UPDATE
			SET public_key = $1 -- do nothing but force return
		RETURNING id
	`, publicKey).Scan(&id)
	return id, castPgErr(err)
}

func (s *Store) InsertBlock(ctx context.Context, height int64, hash []byte, created time.Time, proposerID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO blocks (block_height, block_hash, block_time, proposer_id)
		VALUES ($1, $2, $3, $4)
	`, height, hash, created, proposerID)
	return castPgErr(err)
}

// MarkBlock marks a block validated/missed by given validator. If block was
// marked for that validator already an updated of the old value is made.
// This method returns ErrConflict if either block or validator with given ID
// does not exist.
func (s *Store) MarkBlock(ctx context.Context, blockID, validatorID int64, validated bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO block_participations (block_id, validator_id, validated)
		VALUES ($1, $2, $3)
			ON CONFLICT (block_id, validator_id) DO UPDATE SET validated = $3
	`, blockID, validatorID, validated)
	return castPgErr(err)
}

// LatestBlock returns the block with the greatest high value. This method
// returns ErrNotFound if no block exist.
func (s *Store) LatestBlock(ctx context.Context) (*Block, error) {
	var b Block
	err := s.db.QueryRowContext(ctx, `
		SELECT block_height, block_hash, block_time, proposer_id
		FROM blocks
		ORDER BY block_height DESC
		LIMIT 1
	`).Scan(&b.Height, &b.Hash, &b.Time, &b.ProposerID)
	switch err := castPgErr(err); err {
	case nil:
		return &b, nil
	case ErrNotFound:
		return nil, errors.Wrap(err, "no blocks")
	default:
		return nil, errors.Wrap(castPgErr(err), "cannot select block")
	}
}

type Block struct {
	Height     int64
	Hash       []byte
	Time       time.Time
	ProposerID int64
}

var (
	// ErrNotFound is returned when an operation cannot be completed
	// because entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict is returned when an operation cannot be completed
	// because of database constraints.
	ErrConflict = errors.New("conflict")
)

func castPgErr(err error) error {
	if err == nil {
		return nil
	}

	if err == sql.ErrNoRows {
		return ErrNotFound
	}

	if e, ok := err.(*pq.Error); ok {
		switch prefix := e.Code[:2]; prefix {
		case "20":
			return errors.Wrap(ErrNotFound, e.Message)
		case "23":
			return errors.Wrap(ErrConflict, e.Message)
		}
		err = errors.Wrap(err, string(e.Code))
	}

	return err
}
