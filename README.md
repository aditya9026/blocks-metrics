Requires go1.11+

You must also `export GO111MODULE=on` in your environment to use the go modules feature.

# Run locally

For local development you can use a local Postgres instance and any Tendermint
node address.

```sh
# Run local Postgres instance
$ docker run -it --rm -e POSTGRES_PASSWORD='postgres' -p 5432:5432 postgres:alpine

# Run collector. Default configuration is expected to work for local
# development. If needed it can be changed via environment variables.
$ TENDERMINT_WS_URI="wss://rpc-private-a-vip-babynet.iov.one/websocket" \
  POSTGRES_URI="postgresql://postgres@localhost:5432/postgres?sslmode=disable" \
    go run cmd/collector/main.go
```

# Sample queries

First run the above command to fill the database with all the sample hugnet data, then:

Find active validators at height h:

```sql
SELECT v.address 
    FROM validators v 
    INNER JOIN block_participations p ON v.id = p.validator_id 
    WHERE p.block_id = 57;
```

Find all missing precommits (over all validators and blocks)

```sql
SELECT * FROM block_participations WHERE validated = false;

SELECT b.block_height, b.proposer_id, p.validator_id, b.block_time 
    FROM block_participations p 
    INNER JOIN blocks b ON p.block_id = b.block_height 
    WHERE p.validated = false;
```

Find total counts for each validator:

```sql
SELECT COUNT(NULLIF(validated, false)) as signed, COUNT(NULLIF(validated, true)) as missed, validator_id 
    FROM block_participations 
    GROUP BY validator_id 
    ORDER BY validator_id;

SELECT v.address, COUNT(NULLIF(p.validated, false)) as signed, COUNT(NULLIF(p.validated, true)) as missed 
    FROM block_participations p 
    INNER JOIN validators v ON p.validator_id = v.id 
    GROUP BY v.address 
    ORDER BY v.address;
```

Find missed by block proposer:

(by validator id)
```sql
SELECT b.proposer_id, COUNT(*) 
    FROM blocks b 
    INNER JOIN block_participations p ON b.block_height = p.block_id 
    WHERE p.validated = false 
    GROUP BY b.proposer_id
    ORDER BY count DESC;
```

(or with full address)
```sql
SELECT v.address, COUNT(*) 
    FROM validators v 
    INNER JOIN blocks b ON b.proposer_id = v.id 
    INNER JOIN block_participations p ON b.block_height = p.block_id 
    WHERE p.validated = false 
    GROUP BY v.address
    ORDER BY count DESC;
```

Find misses by proposer and signer:

```sql
SELECT b.proposer_id, p.validator_id, COUNT(*) 
    FROM blocks b 
    INNER JOIN block_participations p ON b.block_height = p.block_id 
    WHERE p.validated = false 
    GROUP BY b.proposer_id, p.validator_id;
```

Find misses by **next** proposer and signer: 
(next proposer makes the canonical commits, and note how this ensures no more self-censorship)


```sql
SELECT b.proposer_id, p.validator_id, COUNT(*) 
    FROM blocks b 
    INNER JOIN block_participations p ON b.block_height = p.block_id + 1
    WHERE p.validated = false 
    GROUP BY b.proposer_id, p.validator_id;
```
