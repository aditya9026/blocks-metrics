package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iov-one/block-metrics/pkg/errors"
)

type TendermintClient struct {
	idCnt uint64

	conn *websocket.Conn

	stop chan struct{}

	mu   sync.Mutex
	resp map[string]chan<- *jsonrpcResponse
}

// DialTendermint returns a client that is maintains a websocket connection to
// tendermint API. The websocket is used instead of standard HTTP connection to
// lower the latency, bypass throttling and to allow subscription requests.
func DialTendermint(websocketURL string) (*TendermintClient, error) {
	c, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "dial")
	}
	cli := &TendermintClient{
		conn: c,
		stop: make(chan struct{}),
		resp: make(map[string]chan<- *jsonrpcResponse),
	}
	go cli.readLoop()
	return cli, nil
}

func (c *TendermintClient) Close() error {
	close(c.stop)
	return c.conn.Close()
}

func (c *TendermintClient) readLoop() {
	for {
		select {
		case <-c.stop:
			return
		default:
		}

		var resp jsonrpcResponse
		if err := c.conn.ReadJSON(&resp); err != nil {
			log.Printf("cannot unmarshal JSONRPC message: %s", err)
			continue
		}

		c.mu.Lock()
		respc, ok := c.resp[resp.CorrelationID]
		delete(c.resp, resp.CorrelationID)
		c.mu.Unlock()

		if ok {
			// repc is expected to be a buffered channel so this
			// operation must never block.
			respc <- &resp
		}
	}
}

// Do makes a jsonrpc call. This method is safe for concurrent calls.
//
// Use API as described in https://tendermint.com/rpc/
func (c *TendermintClient) Do(method string, dest interface{}, args ...interface{}) error {
	params := make([]string, len(args))
	for i, v := range args {
		params[i] = fmt.Sprint(v)
	}
	req := jsonrpcRequest{
		ProtocolVersion: "2.0",
		CorrelationID:   fmt.Sprint(atomic.AddUint64(&c.idCnt, 1)),
		Method:          method,
		Params:          params,
	}

	respc := make(chan *jsonrpcResponse, 1)
	c.mu.Lock()
	c.resp[req.CorrelationID] = respc
	c.mu.Unlock()

	if err := c.conn.WriteJSON(req); err != nil {
		return errors.Wrap(err, "write JSON")
	}

	resp := <-respc

	if resp.Error != nil {
		return errors.Wrapf(ErrFailedResponse,
			"%d: %s",
			resp.Error.Code, resp.Error.Message)
	}
	if err := json.Unmarshal(resp.Result, dest); err != nil {
		return errors.Wrap(err, "cannot unmarshal result")
	}
	return nil
}

type jsonrpcRequest struct {
	ProtocolVersion string   `json:"jsonrpc"`
	CorrelationID   string   `json:"id"`
	Method          string   `json:"method"`
	Params          []string `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	ProtocolVersion string `json:"jsonrpc"`
	CorrelationID   string `json:"id"`
	Result          json.RawMessage
	Error           *struct {
		Code    int64
		Message string
	}
}

var (
	ErrFailedResponse = errors.New("failed response")
)

// Validators return all validators as represented on the block at given
// height.
func Validators(ctx context.Context, c *TendermintClient, blockHeight int64) ([]*TendermintValidator, error) {
	var payload struct {
		Validators []struct {
			Address hexstring
			PubKey  struct {
				Value []byte
			} `json:"pub_key"`
		}
	}
	if err := c.Do("validators", &payload, blockHeight); err != nil {
		return nil, errors.Wrap(err, "query tendermint")
	}
	var validators []*TendermintValidator
	for _, v := range payload.Validators {
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

// MissingValidators finds all validators who did not sign
func MissingValidators(vSet []*TendermintValidator, signers [][]byte) [][]byte {
	missing := ValidatorAddresses(vSet)
	// splice out all those who we find
	for _, signer := range signers {
		missing = removeSigner(missing, signer)
	}
	return missing
}

// removeSigner will remove it from the original set if present and return the remainder
func removeSigner(original [][]byte, toRemove []byte) [][]byte {
	for i, o := range original {
		if bytes.Equal(o, toRemove) {
			// Delete this element from original
			original[i] = original[len(original)-1]
			original = original[:len(original)-1]
			break
		}
	}
	return original
}

// ValidatorAddresses extracts just the addresses of out a signing set
func ValidatorAddresses(validators []*TendermintValidator) [][]byte {
	res := make([][]byte, len(validators))
	for i, v := range validators {
		res[i] = v.Address
	}
	return res
}

func Commit(ctx context.Context, c *TendermintClient, height int64) (*TendermintCommit, error) {
	var payload struct {
		SignedHeader struct {
			Header struct {
				Height          sint64    `json:"height"`
				Time            time.Time `json:"time"`
				ProposerAddress hexstring `json:"proposer_address"`
				ValidatorsHash  hexstring `json:"validators_hash"`
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
	}

	if err := c.Do("commit", &payload, height); err != nil {
		return nil, errors.Wrap(err, "query tendermint")
	}

	commit := TendermintCommit{
		Height:          payload.SignedHeader.Header.Height.Int64(),
		Hash:            payload.SignedHeader.Commit.BlockID.Hash,
		Time:            payload.SignedHeader.Header.Time.UTC(),
		ProposerAddress: payload.SignedHeader.Header.ProposerAddress,
		ValidatorsHash:  payload.SignedHeader.Header.ValidatorsHash,
	}

	for _, pc := range payload.SignedHeader.Commit.Precommits {
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
	ValidatorsHash       []byte
	ParticipantAddresses [][]byte
}
