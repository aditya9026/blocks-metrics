package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/iov-one/block-metrics/pkg/errors"
)

type TendermintClient struct {
	BaseURL string
	Client  http.Client
}

// Get creates a GET request to the tendermint node and loads the JSON encoded
// response content into given destination structure.
// Requests that were unsuccessful because of throttling are retried before
// returning ErrThrottled error.
//
// Use API as described in https://tendermint.com/rpc/
func (c *TendermintClient) Get(ctx context.Context, path string, dest interface{}) error {
	var attempt int

	for {
		err := c.get(ctx, path, dest)
		if !ErrThrottled.Is(err) {
			return err
		}
		if attempt > 5 {
			return errors.Wrapf(err, "failed attempts=%d", attempt)
		}

		attempt++

		select {
		case <-time.After(time.Duration(attempt) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *TendermintClient) get(ctx context.Context, path string, dest interface{}) error {
	fullURL := c.BaseURL + path
	resp, err := c.Client.Get(fullURL)
	if err != nil {
		return errors.Wrapf(err, "cannot query %q", fullURL)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// All good.
	case http.StatusTooManyRequests:
		return ErrThrottled
	default:
		b, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1e6))
		return errors.Wrapf(ErrFailedResponse, "%d: %s", resp.StatusCode, b)
	}

	if err := json.NewDecoder(resp.Body).Decode(&dest); err != nil {
		return errors.Wrap(err, "cannot decode payload")
	}
	return nil
}

var (
	ErrFailedResponse = errors.New("failed response")

	// ErrThrottled is returned when a request is rejected because of
	// server throttling policy.
	ErrThrottled = errors.New("throttled")
)

// Blocks returns the next few blocks starting with the one of a given height.
func Blocks(ctx context.Context, c *TendermintClient, minHeight int64) ([]*TendermintBlock, error) {
	var payload struct {
		Error  json.RawMessage // Tendermint cannot decide on the type.
		Result struct {
			BlockMetas []struct {
				BlockID struct {
					Hash hexstring
				} `json:"block_id"`
				Header struct {
					Height          sint64    `json:"height"`
					Time            time.Time `json:"time"`
					ProposerAddress hexstring `json:"proposer_address"`
				}
			} `json:"block_metas"`
		}
	}
	path := fmt.Sprintf("/blockchain?minHeight=%d&maxHeight=%d", minHeight, minHeight+20)
	if err := c.Get(ctx, path, &payload); err != nil {
		return nil, errors.Wrap(err, "query tendermint")
	}

	if payload.Error != nil {
		return nil, errors.Wrapf(ErrFailedResponse, string(payload.Error))
	}

	var blocks []*TendermintBlock
	for _, meta := range payload.Result.BlockMetas {
		blocks = append(blocks, &TendermintBlock{
			Height:          meta.Header.Height.Int64(),
			Time:            meta.Header.Time,
			Hash:            meta.BlockID.Hash,
			ProposerAddress: meta.Header.ProposerAddress,
		})
	}

	return blocks, nil
}

type TendermintBlock struct {
	Height          int64
	Hash            []byte
	Time            time.Time
	ProposerAddress []byte
}

// Validators return all validators as represented on the block at given
// height.
func Validators(ctx context.Context, c *TendermintClient, blockHeight int64) ([]*TendermintValidator, error) {
	var payload struct {
		Result struct {
			Validators []struct {
				Address hexstring
				PubKey  struct {
					Value []byte
				} `json:"pub_key"`
			}
		}
	}
	path := fmt.Sprintf("/validators?height=%d", blockHeight)
	if err := c.Get(ctx, path, &payload); err != nil {
		return nil, errors.Wrap(err, "query tendermint")
	}
	var validators []*TendermintValidator
	for _, v := range payload.Result.Validators {
		validators = append(validators, &TendermintValidator{
			Address: v.Address,
			PubKey:  v.PubKey.Value,
		})
	}
	return validators, nil
}

type TendermintValidator struct {
	Address []byte
	PubKey  []byte
}
