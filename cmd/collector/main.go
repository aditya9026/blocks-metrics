package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/iov-one/block-metrics/pkg/errors"
	"github.com/iov-one/block-metrics/pkg/metrics"
)

func main() {
	conf := configuration{
		PostgresURI:   env("POSTGRES_URI", "user=postgres dbname=postgres sslmode=disable"),
		TendermintURL: env("TENDERMINT_URL", "http://localhost:26657"),
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
	TendermintURL string
}

func run(conf configuration) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := sql.Open("postgres", conf.PostgresURI)
	if err != nil {
		return fmt.Errorf("cannot connect to postgres: %s", err)
	}
	defer db.Close()

	if err := metrics.EnsureSchema(db); err != nil {
		return fmt.Errorf("ensure schema: %s", err)
	}

	st := metrics.NewStore(db)

	tmc := &metrics.TendermintClient{
		BaseURL: conf.TendermintURL,
	}

	inserted, err := metrics.Sync(ctx, tmc, st)
	if err != nil {
		return errors.Wrap(err, "sync")
	}

	fmt.Println("inserted:", inserted)

	return nil
}
