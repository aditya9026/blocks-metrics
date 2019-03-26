package metrics

import (
	"database/sql"
	"fmt"
	"strings"
)

func EnsureSchema(pg *sql.DB) error {
	tx, err := pg.Begin()
	if err != nil {
		return fmt.Errorf("transaction begin: %s", err)
	}
	defer tx.Rollback()

	for _, query := range strings.Split(schema, "\n---\n") {
		query = strings.TrimSpace(query)

		if _, err := tx.Exec(query); err != nil {
			return &QueryError{Query: query, Err: err}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit: %s", err)
	}
	return nil
}

const schema = `

CREATE TABLE IF NOT EXISTS validators (
	id SERIAL PRIMARY KEY,
	public_key TEXT NOT NULL UNIQUE,
	memo TEXT
);

---

CREATE TABLE IF NOT EXISTS blocks (
	block_height BIGINT NOT NULL PRIMARY KEY,
	block_hash TEXT NOT NULL,
	block_time TIMESTAMPTZ NOT NULL,
	proposer_id INT NOT NULL REFERENCES validators(id)
);

---

CREATE TABLE IF NOT EXISTS block_participations (
	id BIGSERIAL PRIMARY KEY,
	validated BOOLEAN NOT NULL,
	block_id BIGINT NOT NULL REFERENCES blocks(block_height),
	validator_id INT NOT NULL REFERENCES validators(id),

	UNIQUE (block_id, validator_id)
);

`

type QueryError struct {
	Query string
	Args  []interface{}
	Err   error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query error: %s\n%q", e.Err, e.Query)
}
