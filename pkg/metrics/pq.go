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

// InsertValidator adds a validator information into the database. It returns
// the newly created validator ID on success.
// This method returns ErrConflict if the validator cannot be inserted due to
// conflicting data.
func (s *Store) InsertValidator(ctx context.Context, publicKey, address []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO validators (public_key, address)
		VALUES ($1, $2)
		RETURNING id
	`, publicKey, address).Scan(&id)
	return id, castPgErr(err)
}

// ValidatorAddressID returns an ID of a validator with given address. It
// returns ErrNotFound if no such address is present in the database.
func (s *Store) ValidatorAddressID(ctx context.Context, address []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM validators WHERE address = $1
		LIMIT 1
	`, address).Scan(&id)
	return id, castPgErr(err)
}

func (s *Store) InsertBlock(ctx context.Context, b Block) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO blocks (block_height, block_hash, block_time, proposer_id, participant_ids)
		VALUES ($1, $2, $3, $4, $5)
	`, b.Height, b.Hash, b.Time.UTC(), b.ProposerID, pq.Array(b.ParticipantIDs))
	return castPgErr(err)
}

// LatestBlock returns the block with the greatest high value. This method
// returns ErrNotFound if no block exist.
func (s *Store) LatestBlock(ctx context.Context) (*Block, error) {
	var b Block

	err := s.db.QueryRowContext(ctx, `
		SELECT block_height, block_hash, block_time, proposer_id, participant_ids
		FROM blocks
		ORDER BY block_height DESC
		LIMIT 1
	`).Scan(&b.Height, &b.Hash, &b.Time, &b.ProposerID, pq.Array(&b.ParticipantIDs))
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
	Height         int64
	Hash           []byte
	Time           time.Time
	ProposerID     int64
	ParticipantIDs []int64
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
