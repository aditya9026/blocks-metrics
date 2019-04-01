Requires go1.11+

You must also `export GO111MODULE=on` in your environment to use the go modules feature.

# Run locally

For local development you can use a local Postgres instance and any Tendermint
node address.

```sh
# Run local Postgres instance
$ docker run -it --rm -e POSTGRES_PASSWORD='' -p 5432:5432 postgres:alpine

# Run collector. Default configuration is expected to work for local
# development. If needed it can be changed via environment variables.
$ TENDERMINT_WS_URI="wss://bns.hugnet.iov.one/websocket" \
  POSTGRES_URI="postgresql://postgres@localhost:5432" \
    go run cmd/collector/main.go
```
