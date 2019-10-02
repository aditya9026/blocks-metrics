package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/aditya9026/blocks-metrics/pkg/errors"
	"github.com/aditya9026/blocks-metrics/pkg/metrics"
)

func main() {
	conf := configuration{
		PostgresURI:     env("POSTGRES_URI", "user=postgres dbname=postgres"),
		TendermintWsURI: env("TENDERMINT_WS_URI", "wss://bns.lovenet.iov.one/websocket"),
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
	PostgresURI     string
	TendermintWsURI string
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

	tmc, err := metrics.DialTendermint(conf.TendermintWsURI)
	if err != nil {
		return errors.Wrap(err, "dial tendermint")
	}
	defer tmc.Close()

	inserted, err := metrics.Sync(ctx, tmc, st)
	if err != nil {
		return errors.Wrap(err, "sync")
	}

	fmt.Println("inserted:", inserted)

	return nil
}
