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
		syncedHeight = 1
	case err == nil:
		syncedHeight = block.Height
	default:
		return inserted, errors.Wrap(err, "latest block")
	}

	// Keep the mapping for validator address to their numeric ID in memory
	// to avoid querying the database for every insert.
	validatorIDs := make(map[string]int64)

	for {
		c, err := Commit(ctx, tmc, syncedHeight+1)
		if err != nil {

			// BUG this can happen when the commit does not exist.
			// There is no sane way to distinguish this case from
			// any other tendermint API error.
			// Commit(ctx, tmc, -1) can be used to get the latest
			// commit.

			return inserted, errors.Wrapf(err, "blocks for %d", syncedHeight+1)
		}

		pid, ok := validatorIDs[string(c.ProposerAddress)]
		if !ok {
			pid, err = validatorID(ctx, tmc, st, c.ProposerAddress, c.Height)
			if err != nil {
				return inserted, errors.Wrapf(err, "proposer address %x", c.ProposerAddress)
			}
			validatorIDs[string(c.ProposerAddress)] = pid
		}

		if err := st.InsertBlock(ctx, c.Height, c.Hash, c.Time.UTC(), pid); err != nil {
			return inserted, errors.Wrapf(err, "insert block %d", c.Height)
		}
		inserted++

		for _, addr := range c.ParticipantAddresses {
			vid, ok := validatorIDs[string(addr)]
			if !ok {
				vid, err = validatorID(ctx, tmc, st, addr, c.Height)
				if err != nil {
					return inserted, errors.Wrapf(err, "validator address %x", addr)
				}
				validatorIDs[string(addr)] = vid
			}
			if err := st.MarkBlock(ctx, c.Height, vid, true); err != nil {
				return inserted, errors.Wrapf(err, "cannot mark %d block for %d", c.Height, vid)
			}
		}

		syncedHeight = c.Height
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
	if len(address) == 0 {
		return 0, errors.Wrap(ErrNotFound, "empty validator address")
	}

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
	return 0, errors.Wrapf(ErrNotFound, "validator %x not present at height %d", address, blockHeight)
}

// StreamSync is expected to work similar to Sync but to use websocket and
// never quit unless context was cancelled.
func StreamSync(ctx context.Context) error {
	return errors.New("not implemented")
}
