package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/iov-one/block-metrics/pkg/metrics"
)

func main() {
	conf := configuration{
		PostgresURI:   env("POSTGRES_URI", "user=postgres dbname=postgres sslmode=disable"),
		TendermintRPC: env("TENDERMINT_RPC", "http://localhost:26657"),
	}

	if err := run(conf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}

type configuration struct {
	PostgresURI   string
	TendermintRPC string
}

func run(conf configuration) error {
	db, err := sql.Open("postgres", conf.PostgresURI)
	if err != nil {
		return fmt.Errorf("cannot connect to postgres: %s", err)
	}
	defer db.Close()

	if err := metrics.EnsureSchema(db); err != nil {
		return fmt.Errorf("ensure schema: %s", err)
	}

	return nil
}
