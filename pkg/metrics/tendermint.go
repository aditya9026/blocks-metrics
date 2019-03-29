package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iov-one/block-metrics/pkg/errors"
)

type TendermintClient struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

// DialTendermint returns a client that is maintains a websocket connection to
// tendermint API. The websocket is used instead of standard HTTP connection to
// lower the latency, bypass throttling and to allow subscription requests.
func DialTendermint(websocketURL string) (*TendermintClient, error) {
	c, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "dial")
	}
	return &TendermintClient{conn: c}, nil
}

func (c *TendermintClient) Close() error {
	return c.conn.Close()
}

// Get sends a request to the tendermint node and loads the JSON encoded
// response content into given destination structure.
// Requests that were unsuccessful because of throttling are retried before
// returning ErrThrottled error.
//
// Use API as described in https://tendermint.com/rpc/
func (c *TendermintClient) Get(ctx context.Context, dest interface{}, method string, args ...interface{}) error {
	params := make([]string, len(args))
	for i, v := range args {
		params[i] = fmt.Sprint(v)
	}
	req := struct {
		Method string   `json:"method"`
		Params []string `json:"params"`
	}{
		Method: method,
		Params: params,
	}

	// Usually because this is a single socket connection a correlation ID
	// should be used to match messages. In this case this is a sequential
	// read-write call without concurrent use. It is easier to sequentially
	// process data. Use lock to ensure no request-response is being mixed.
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.conn.WriteJSON(req); err != nil {
		return errors.Wrap(err, "write JSON")
	}

	if err := c.conn.ReadJSON(dest); err != nil {
		return errors.Wrap(err, "read JSON")
	}
	return nil
}

var (
	ErrFailedResponse = errors.New("failed response")

	// ErrThrottled is returned when a request is rejected because of
	// server throttling policy.
	ErrThrottled = errors.New("throttled")
)

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
	if err := c.Get(ctx, &payload, "validators", blockHeight); err != nil {
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

func Commit(ctx context.Context, c *TendermintClient, height int64) (*TendermintCommit, error) {
	var payload struct {
		Error  json.RawMessage `json:"error"` // Tendermint cannot decide on the type.
		Result struct {
			SignedHeader struct {
				Header struct {
					Height          sint64    `json:"height"`
					Time            time.Time `json:"time"`
					ProposerAddress hexstring `json:"proposer_address"`
				} `json:"header"`
				Commit struct {
					BlockID struct {
						Hash hexstring `json:"hash"`
					} `json:"block_id"`
					Precommits []*struct {
						ValidatorAddress hexstring `json:"validator_address"`
					} `json:"precommits"`
				} `json:"commit"`
			} `json:"signed_header"`
		} `json:"result"`
	}

	if err := c.Get(ctx, &payload, "commit", height); err != nil {
		return nil, errors.Wrap(err, "query tendermint")
	}

	if payload.Error != nil {
		return nil, errors.Wrapf(ErrFailedResponse, string(payload.Error))
	}

	commit := TendermintCommit{
		Height:          payload.Result.SignedHeader.Header.Height.Int64(),
		Hash:            payload.Result.SignedHeader.Commit.BlockID.Hash,
		Time:            payload.Result.SignedHeader.Header.Time.UTC(),
		ProposerAddress: payload.Result.SignedHeader.Header.ProposerAddress,
	}

	for _, pc := range payload.Result.SignedHeader.Commit.Precommits {
		if pc == nil {
			continue
		}
		commit.ParticipantAddresses = append(commit.ParticipantAddresses, pc.ValidatorAddress)
	}

	return &commit, nil
}

type TendermintCommit struct {
	Height               int64
	Hash                 []byte
	Time                 time.Time
	ProposerAddress      []byte
	ParticipantAddresses [][]byte
}
