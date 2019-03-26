package metrics

import (
	tmclient "github.com/tendermint/tendermint/abci/client"
)

func ConnectTendermint(addr string) {
	_, _ = tmclient.NewClient(addr, "grpc", true)
}
