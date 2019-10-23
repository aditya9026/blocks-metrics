package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/iov-one/blocks-metrics/pkg/errors"
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot create transaction")
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO blocks (block_height, block_hash, block_time, proposer_id, messages, fee_frac)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, b.Height, b.Hash, b.Time.UTC(), b.ProposerID, pq.Array(b.Messages), b.FeeFrac)
	if err != nil {
		return wrapPgErr(err, "insert block")
	}

	for _, part := range b.ParticipantIDs {
		_, err = tx.ExecContext(ctx, `
		INSERT INTO block_participations (validated, block_id, validator_id)
		VALUES (true, $1, $2)
		`, b.Height, part)
		if err != nil {
			return wrapPgErr(err, "insert block participant")
		}
	}

	for _, missed := range b.MissingIDs {
		_, err = tx.ExecContext(ctx, `
		INSERT INTO block_participations (validated, block_id, validator_id)
		VALUES (false, $1, $2)
		`, b.Height, missed)
		if err != nil {
			return wrapPgErr(err, "insert block participant")
		}
	}

	for _, transaction := range b.Transactions {
		_, err := tx.ExecContext(ctx, `
		INSERT INTO transactions(transaction_hash, block_id, message)
		VALUES($1, $2, $3)`, transaction.Hash, b.Height, transaction.Message)
		if err != nil {
			return wrapPgErr(err, "insert transaction")
		}
	}

	err = tx.Commit()
	return wrapPgErr(err, "commit block tx")
}

// LatestBlock returns the block with the greatest high value. This method
// returns ErrNotFound if no block exist.
// Note that it doesn't load the validators by default
func (s *Store) LatestBlock(ctx context.Context) (*Block, error) {
	var b Block

	err := s.db.QueryRowContext(ctx, `
		SELECT block_height, block_hash, block_time, proposer_id, messages, fee_frac
		FROM blocks
		ORDER BY block_height DESC
		LIMIT 1
	`).Scan(&b.Height, &b.Hash, &b.Time, &b.ProposerID, pq.Array(&b.Messages), &b.FeeFrac)

	if err == nil {
		// normalize it here, as not always stored like this in the db
		b.Time = b.Time.UTC()
		b.ParticipantIDs, b.MissingIDs, err = s.loadParticipants(ctx, b.Height)
		return &b, err
	}

	err = castPgErr(err)
	if ErrNotFound.Is(err) {
		return nil, errors.Wrap(err, "no blocks")
	}
	return nil, errors.Wrap(castPgErr(err), "cannot select block")
}

// LoadBlock returns the block with the given block height from the database.
// This method returns ErrNotFound if no block exist.
// Note that it doesn't load the validators by default
//
// TODO: de-duplicate LatestBlock() code
func (s *Store) LoadBlock(ctx context.Context, blockHeight int64) (*Block, error) {
	var b Block

	err := s.db.QueryRowContext(ctx, `
		SELECT block_height, block_hash, block_time, proposer_id, messages, fee_frac
		FROM blocks
		WHERE block_height = $1
	`, blockHeight).Scan(&b.Height, &b.Hash, &b.Time, &b.ProposerID, pq.Array(&b.Messages), &b.FeeFrac)

	if err == nil {
		// normalize it here, as not always stored like this in the db
		b.Time = b.Time.UTC()
		b.ParticipantIDs, b.MissingIDs, err = s.loadParticipants(ctx, b.Height)
		return &b, err
	}

	err = castPgErr(err)
	if ErrNotFound.Is(err) {
		return nil, errors.Wrap(err, "no blocks")
	}
	return nil, errors.Wrap(castPgErr(err), "cannot select block")
}

// loadParticipants will load the participants for the given block and update the structure.
// Automatically called as part of Load/LatestBlock to give you the full info
func (s *Store) loadParticipants(ctx context.Context, blockHeight int64) (participants []int64, missing []int64, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT validator_id, validated
		FROM block_participations
		WHERE block_id = $1
	`, blockHeight)
	if err != nil {
		err = wrapPgErr(err, "query participants")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var pid int64
		var validated bool
		if err = rows.Scan(&pid, &validated); err != nil {
			err = wrapPgErr(rows.Err(), "scanning participants")
			return
		}
		if validated {
			participants = append(participants, pid)
		} else {
			missing = append(missing, pid)
		}
	}

	err = wrapPgErr(rows.Err(), "scanning participants")
	return
}

type Block struct {
	Height         int64
	Hash           []byte
	Time           time.Time
	ProposerID     int64
	ParticipantIDs []int64
	MissingIDs     []int64
	Messages       []string
	FeeFrac        uint64
	Transactions   []Transaction
}

type Transaction struct {
	Hash    []byte
	Message string
}

var (
	// ErrNotFound is returned when an operation cannot be completed
	// because entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict is returned when an operation cannot be completed
	// because of database constraints.
	ErrConflict = errors.New("conflict")
)

func wrapPgErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	return errors.Wrap(castPgErr(err), msg)
}

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
