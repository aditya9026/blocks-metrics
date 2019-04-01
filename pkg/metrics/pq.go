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
	if len(b.ParticipantIDs) == 0 {
		return errors.Wrap(ErrConflict, "no participants on block")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return castPgErr(err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO blocks (block_height, block_hash, block_time, proposer_id)
		VALUES ($1, $2, $3, $4)
	`, b.Height, b.Hash, b.Time.UTC(), b.ProposerID)
	if err != nil {
		return castPgErr(err)
	}

	for _, part := range b.ParticipantIDs {
		_, err = tx.ExecContext(ctx, `
		INSERT INTO block_participations (validated, block_id, validator_id)
		VALUES (true, $1, $2)
		`, b.Height, part)
		if err != nil {
			return castPgErr(err)
		}
	}

	for _, missed := range b.MissingIDs {
		_, err = tx.ExecContext(ctx, `
		INSERT INTO block_participations (validated, block_id, validator_id)
		VALUES (false, $1, $2)
		`, b.Height, missed)
		if err != nil {
			return castPgErr(err)
		}
	}

	err = tx.Commit()
	return castPgErr(err)
}

// LatestBlock returns the block with the greatest high value. This method
// returns ErrNotFound if no block exist.
// Note that it doesn't load the validators by default
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
		// normalize it here, as not always stored like this in the db
		b.Time = b.Time.UTC()
		return &b, nil
	case ErrNotFound:
		return nil, errors.Wrap(err, "no blocks")
	default:
		return nil, errors.Wrap(castPgErr(err), "cannot select block")
	}
}

// LoadBlock returns the block with the given block height from the database.
// This method returns ErrNotFound if no block exist.
// Note that it doesn't load the validators by default
//
// TODO: de-duplicate LatestBlock() code
func (s *Store) LoadBlock(ctx context.Context, blockHeight int64) (*Block, error) {
	var b Block

	err := s.db.QueryRowContext(ctx, `
		SELECT block_height, block_hash, block_time, proposer_id
		FROM blocks
		WHERE block_height = $1
	`, blockHeight).Scan(&b.Height, &b.Hash, &b.Time, &b.ProposerID)
	switch err := castPgErr(err); err {
	case nil:
		// normalize it here, as not always stored like this in the db
		b.Time = b.Time.UTC()
		return &b, nil
	case ErrNotFound:
		return nil, errors.Wrap(err, "no blocks")
	default:
		return nil, errors.Wrap(castPgErr(err), "cannot select block")
	}
}

// LoadParticipants will load the participants for the given block and update the structure.
// Together with LatestBlock, you get the full info
func (s *Store) LoadParticipants(ctx context.Context, b *Block) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT validator_id, validated
		FROM block_participations
		WHERE block_id = $1
	`, b.Height)
	if err != nil {
		return castPgErr(err)
	}

	var participants []int64
	var missing []int64
	for rows.Next() {
		var pid int64
		var validated bool
		if err := rows.Scan(&pid, &validated); err != nil {
			return castPgErr(err)
		}
		if validated {
			participants = append(participants, pid)
		} else {
			missing = append(missing, pid)
		}
	}

	b.ParticipantIDs = participants
	b.MissingIDs = missing
	return nil
}

type Block struct {
	Height         int64
	Hash           []byte
	Time           time.Time
	ProposerID     int64
	ParticipantIDs []int64
	MissingIDs     []int64
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
