package metrics

import (
	"bytes"
	"context"
	"time"

	"github.com/iov-one/block-metrics/pkg/errors"
)

const syncRetryTimeout = 3 * time.Second

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
		syncedHeight = 0
	case err == nil:
		syncedHeight = block.Height
	default:
		return inserted, errors.Wrap(err, "latest block")
	}

	// Keep the mapping for validator address to their numeric ID in memory
	// to avoid querying the database for every insert.
	validatorIDs := newValidatorsCache(tmc, st)
	var vSet []*TendermintValidator
	var vHash []byte

	for {
		nextHeight := syncedHeight + 1
		info, err := AbciInfo(tmc)
		if err != nil {
			return inserted, errors.Wrap(err, "info")
		}

		if info.LastBlockHeight < nextHeight {
			select {
			case <-ctx.Done():
				return inserted, ctx.Err()
			case <-time.After(syncRetryTimeout):
			}
			// make sure we don't run into the bug where we try to retrieve a commit for non-existent height
			continue
		}

		c, err := Commit(ctx, tmc, nextHeight)
		if err != nil {

			// BUG this can happen when the commit does not exist.
			// There is no sane way to distinguish this case from
			// any other tendermint API error.
			return inserted, errors.Wrapf(err, "blocks for %d", syncedHeight+1)
		}
		syncedHeight = c.Height

		propID, err := validatorIDs.DatabaseID(ctx, c.ProposerAddress, c.Height)
		if err != nil {
			return inserted, errors.Wrap(err, "validator ID")
		}

		participantIDs, err := validatorIDs.DatabaseIDs(ctx, c.ParticipantAddresses, c.Height)
		if err != nil {
			return inserted, errors.Wrap(err, "validator ID")
		}

		// only query when validator hash changes
		if !bytes.Equal(c.ValidatorsHash, vHash) {
			var err error
			vSet, err = Validators(ctx, tmc, c.Height)
			if err != nil {
				return inserted, errors.Wrap(err, "cannot get validator set")
			}
			vHash = c.ValidatorsHash
		}

		missing := SubtractSets(ValidatorAddresses(vSet), c.ParticipantAddresses)
		missingIDs, err := validatorIDs.DatabaseIDs(ctx, missing, c.Height)
		if err != nil {
			return inserted, errors.Wrap(err, "validator ID")
		}

		block := Block{
			Height:         c.Height,
			Hash:           c.Hash,
			Time:           c.Time.UTC(),
			ProposerID:     propID,
			ParticipantIDs: participantIDs,
			MissingIDs:     missingIDs,
		}
		if err := st.InsertBlock(ctx, block); err != nil {
			return inserted, errors.Wrapf(err, "insert block %d", c.Height)
		}
		inserted++
	}
}

// validatorsCache maintain a cache for the mapping of validator address to
// that validator database ID.
type validatorsCache struct {
	cache map[string]int64
	tmc   *TendermintClient
	st    *Store
}

func newValidatorsCache(tmc *TendermintClient, st *Store) *validatorsCache {
	return &validatorsCache{
		cache: make(map[string]int64),
		tmc:   tmc,
		st:    st,
	}
}

// DatabaseIDs is a helper of DatabaseID to query a whole set at once
func (vc *validatorsCache) DatabaseIDs(ctx context.Context, addresses [][]byte, blockHeight int64) ([]int64, error) {
	res := make([]int64, len(addresses))
	for i, addr := range addresses {
		id, err := vc.DatabaseID(ctx, addr, blockHeight)
		if err != nil {
			return nil, err
		}
		res[i] = id
	}
	return res, nil
}

// DatabaseID will return an ID of a validator with given address. If not
// present in the database it will query tendermint for the information,
// register that validator in the database and return its ID.
func (vc *validatorsCache) DatabaseID(ctx context.Context, address []byte, blockHeight int64) (int64, error) {
	id, ok := vc.cache[string(address)]
	if ok {
		return id, nil
	}

	if len(address) == 0 {
		return 0, errors.Wrap(ErrNotFound, "empty validator address")
	}

	switch id, err := vc.st.ValidatorAddressID(ctx, address); {
	case err == nil:
		vc.cache[string(address)] = id
		return id, nil
	case ErrNotFound.Is(err):
		// Not in the database yet.
	default:
		return 0, errors.Wrap(err, "query validator ID")
	}

	vs, err := Validators(ctx, vc.tmc, blockHeight)
	if err != nil {
		return 0, errors.Wrap(err, "fetch validators")
	}

	for _, v := range vs {
		if !bytes.Equal(v.Address, address) {
			continue
		}
		id, err := vc.st.InsertValidator(ctx, v.PubKey, v.Address)
		if err != nil {
			return 0, errors.Wrap(err, "insert validator")
		}

		vc.cache[string(address)] = id
		return id, nil
	}
	return 0, errors.Wrapf(ErrNotFound, "validator %x not present at height %d", address, blockHeight)
}

// StreamSync is expected to work similar to Sync but to use websocket and
// never quit unless context was cancelled.
func StreamSync(ctx context.Context) error {
	return errors.New("not implemented")
}
