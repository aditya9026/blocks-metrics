Requires go1.11+

# Run locally

For local development you can use a local Postgres instance and any Tendermint
node address.

```sh
# Run local Postgres instance
$ docker run -it --rm -e POSTGRES_PASSWORD='' -p 5432:5432 postgres:alpine

# Run collector. Default configuration is expected to work for local
# development. If needed it can be changed via environment variables.
$ TENDERMINT_RPC="TODO" \
    go run cmd/collector/main.go
```
