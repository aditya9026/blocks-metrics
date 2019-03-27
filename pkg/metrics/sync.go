package metrics

import (
	"bytes"
	"context"

	"github.com/iov-one/block-metrics/pkg/errors"
)

// Sync uploads to local store all blocks that are not present yet, starting
// with the blocks with the lowest hight first. It always returns the number of
// blocks inserted, even if returning an error.
func Sync(ctx context.Context, tmc *TendermintClient, st *Store) (uint, error) {
	var (
		inserted     uint
		syncedHeight int64
	)

	switch block, err := st.LatestBlock(ctx); {
	case ErrNotFound.Is(err):
		syncedHeight = -1
	case err == nil:
		syncedHeight = block.Height
	default:
		return inserted, errors.Wrap(err, "latest block")
	}

	// Keep the mapping for validator address to their numeric ID in memory
	// to avoid querying the database for every insert.
	validatorIDs := make(map[string]int64)

	for {
		blocks, err := Blocks(ctx, tmc, syncedHeight+1)
		if err != nil {

			// BUG this can happen if no block was created since
			// the last sync. We ask for blocks with height greater
			// than the existing one which makes tendermint fail
			// internally.

			return inserted, errors.Wrapf(err, "blocks for %d", syncedHeight+1)
		}

		if len(blocks) == 0 {
			return inserted, nil
		}

		for _, b := range blocks {
			pid, ok := validatorIDs[string(b.ProposerAddress)]
			if !ok {
				pid, err = validatorID(ctx, tmc, st, b.ProposerAddress, b.Height)
				if err != nil {
					return inserted, errors.Wrapf(err, "proposer address %x", b.ProposerAddress)
				}
				validatorIDs[string(b.ProposerAddress)] = pid
			}

			if err := st.InsertBlock(ctx, b.Height, b.Hash, b.Time.UTC(), pid); err != nil {
				return inserted, errors.Wrapf(err, "insert block %d", b.Height)
			}
			inserted++

			// BUG because blocks are not ordered if this loop is
			// not finished a corrupted database state is created.
			// A block with higher value might be inserted skipping
			// the blocks with lower height. Because next sync will
			// start from the highest block value, missing blocks
			// will never be synced.

			// Blocks are not returned in any order.
			if b.Height > syncedHeight {
				syncedHeight = b.Height
			}
		}
	}
}

// validatorID will return an ID of a validator with given address. If not
// present in the database it will query tendermint for the information,
// register that validator in the database and return its ID.
func validatorID(
	ctx context.Context,
	tm *TendermintClient,
	st *Store,
	address []byte,
	blockHeight int64,
) (int64, error) {
	switch id, err := st.ValidatorAddressID(ctx, address); {
	case err == nil:
		return id, nil
	case ErrNotFound.Is(err):
		// Not in the database yet.
	default:
		return 0, errors.Wrap(err, "query validator ID")
	}

	vs, err := Validators(ctx, tm, blockHeight)
	if err != nil {
		return 0, errors.Wrap(err, "fetch validators")
	}

	for _, v := range vs {
		if !bytes.Equal(v.Address, address) {
			continue
		}
		id, err := st.InsertValidator(ctx, v.PubKey, v.Address)
		if err != nil {
			return 0, errors.Wrap(err, "insert validator")
		}
		return id, nil
	}
	return 0, errors.Wrapf(ErrNotFound, "validator not present at height %d", blockHeight)
}

// StreamSync is expected to work similar to Sync but to use websocket and
// never quit unless context was cancelled.
func StreamSync(ctx context.Context) error {
	return errors.New("not implemented")
}
