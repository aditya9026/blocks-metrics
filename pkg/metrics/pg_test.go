package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/iov-one/block-metrics/pkg/errors"
	_ "github.com/lib/pq"
)

func TestLastBlock(t *testing.T) {
	db, cleanup := ensureDB(t)
	defer cleanup()

	ctx := context.Background()

	s := NewStore(db)

	if _, err := s.LatestBlock(ctx); !ErrNotFound.Is(err) {
		t.Fatalf("want ErrNotFound, got %q", err)
	}

	vID, err := s.InsertValidator(ctx, []byte{0x01, 0, 0xbe, 'a'}, []byte{0x02})
	if err != nil {
		t.Fatalf("cannot create a validator: %s", err)
	}

	for i := 5; i < 100; i += 20 {
		block := Block{
			Height: int64(i),
			Hash:   []byte{0, 1, byte(i)},
			// Postgres TIMESTAMPTZ precision is microseconds.
			Time:           time.Now().UTC().Round(time.Microsecond),
			ProposerID:     vID,
			ParticipantIDs: []int64{vID},
		}
		if err := s.InsertBlock(ctx, block); err != nil {
			t.Fatalf("cannot insert block: %s", err)
		}

		if got, err := s.LatestBlock(ctx); err != nil {
			t.Fatalf("cannot get latest block: %s", err)
		} else if !reflect.DeepEqual(got, &block) {
			t.Logf(" got %#v", got)
			t.Logf("want %#v", &block)
			t.Fatal("unexpected result")
		}
	}
}

func TestStoreInsertValidator(t *testing.T) {
	db, cleanup := ensureDB(t)
	defer cleanup()

	ctx := context.Background()

	s := NewStore(db)

	pubkeyA := []byte{0x01, 0, 0xbe, 'a'}
	addrA := []byte{0x02, 'a'}
	if _, err := s.InsertValidator(ctx, pubkeyA, addrA); err != nil {
		t.Fatalf("cannot create 'a' validator: %s", err)
	}

	pubkeyB := []byte{0x01, 0, 0xbe, 'b'}
	addrB := []byte{0x02, 'b'}
	if _, err := s.InsertValidator(ctx, pubkeyB, addrB); err != nil {
		t.Fatalf("cannot create 'b' validator: %s", err)
	}

	if _, err := s.InsertValidator(ctx, pubkeyA, []byte{0x99}); !ErrConflict.Is(err) {
		t.Fatalf("was able to create a validator with an existing public key: %q", err)
	}
	if _, err := s.InsertValidator(ctx, []byte{0x99}, addrA); !ErrConflict.Is(err) {
		t.Fatalf("was able to create a validator with an existing address: %q", err)
	}
}

func TestStoreInsertBlock(t *testing.T) {
	type validator struct {
		address []byte
		pubkey  []byte
	}
	cases := map[string]struct {
		validators []validator
		block      Block
		wantErr    *errors.Error
	}{
		"success": {
			validators: []validator{
				{address: []byte{0x01}, pubkey: []byte{0x01, 0, 0x01}},
				{address: []byte{0x02}, pubkey: []byte{0x02, 0, 0x02}},
				{address: []byte{0x03}, pubkey: []byte{0x03, 0, 0x03}},
			},
			block: Block{
				Height:         1,
				Hash:           []byte{0, 1, 2, 3},
				Time:           time.Now().UTC().Round(time.Millisecond),
				ProposerID:     2,
				ParticipantIDs: []int64{2, 3},
			},
		},
		"missing participant ids": {
			validators: []validator{
				{address: []byte{0x01}, pubkey: []byte{0x01, 0, 0x01}},
			},
			block: Block{
				Height:         1,
				Hash:           []byte{0, 1, 2, 3},
				Time:           time.Now().UTC().Round(time.Millisecond),
				ProposerID:     1,
				ParticipantIDs: nil,
			},
			wantErr: ErrConflict,
		},
		"invalid proposer ID": {
			validators: []validator{
				{address: []byte{0x01}, pubkey: []byte{0x01, 0, 0x01}},
				{address: []byte{0x02}, pubkey: []byte{0x02, 0, 0x02}},
				{address: []byte{0x03}, pubkey: []byte{0x03, 0, 0x03}},
			},
			block: Block{
				Height:         1,
				Hash:           []byte{0, 1, 2, 3},
				Time:           time.Now().UTC().Round(time.Millisecond),
				ProposerID:     4,
				ParticipantIDs: []int64{2, 3},
			},
			wantErr: ErrConflict,
		},
		// This is not implemented.
		//
		// "invalid participant ids ID": {
		// 	validators: []validator{
		// 		{address: []byte{0x01}, pubkey: []byte{0x01, 0, 0x01}},
		// 		{address: []byte{0x02}, pubkey: []byte{0x02, 0, 0x02}},
		// 		{address: []byte{0x03}, pubkey: []byte{0x03, 0, 0x03}},
		// 	},
		// 	block: Block{
		// 		Height:         1,
		// 		Hash:           []byte{0, 1, 2, 3},
		// 		Time:           time.Now().UTC().Round(time.Millisecond),
		// 		ProposerID:     2,
		// 		ParticipantIDs: []int64{666, 999},
		// 	},
		// 	wantErr: ErrConflict,
		// },
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			db, cleanup := ensureDB(t)
			defer cleanup()

			ctx := context.Background()
			s := NewStore(db)

			for _, v := range tc.validators {
				if _, err := s.InsertValidator(ctx, v.pubkey, v.address); err != nil {
					t.Fatalf("cannot ensure validator: %s", err)
				}
			}

			if err := s.InsertBlock(ctx, tc.block); !tc.wantErr.Is(err) {
				t.Errorf("want %q error, got %q", tc.wantErr, err)
			}
		})
	}
}

// ensureDB connects to a Postgres instance creates a database and returns a
// connection to it. If the connection to Postres cannot be established, the
// test is skipped.
//
// Each database is initialized with the schema.
//
// Unless an option is provided, defaults are used:
//   * Database name: test_database_<creation time in unix ns>
//   * Host: localhost
//   * Port: 5432
//   * SSLMode: disable
//   * User: postgres
//
// Function connects to the 'postgres' database first to create a new database.
func ensureDB(t *testing.T) (testdb *sql.DB, cleanup func()) {
	t.Helper()

	var opts = struct {
		User     string
		Password string
		Port     string
		Host     string
		SSLMode  string
		DBName   string
	}{
		User:     env("POSTGRES_TEST_USER", "postgres"),
		Password: env("POSTGRES_TEST_PASSWORD", ""),
		Port:     env("POSTGRES_TEST_PORT", "5432"),
		Host:     env("POSTGRES_TEST_HOST", "localhost"),
		SSLMode:  env("POSTGRES_TEST_SSLMODE", "disable"),
		DBName: env("POSTGRES_TEST_DATABASE",
			fmt.Sprintf("test_database_%d", time.Now().UnixNano())),
	}

	rootDsn := fmt.Sprintf(
		"host='%s' port='%s' user='%s' password='%s' dbname='postgres' sslmode='%s'",
		opts.Host, opts.Port, opts.User, opts.Password, opts.SSLMode)
	rootdb, err := sql.Open("postgres", rootDsn)
	if err != nil {
		t.Skipf("cannot connect to postgres: %s", err)
	}
	if err := rootdb.Ping(); err != nil {
		t.Skipf("cannot ping postgres: %s", err)
	}
	if _, err := rootdb.Exec("CREATE DATABASE " + opts.DBName); err != nil {
		t.Fatalf("cannot create database: %s", err)
		rootdb.Close()
	}

	testDsn := fmt.Sprintf(
		"host='%s' port='%s' user='%s' password='%s' dbname='%s' sslmode='%s'",
		opts.Host, opts.Port, opts.User, opts.Password, opts.DBName, opts.SSLMode)
	testdb, err = sql.Open("postgres", testDsn)
	if err != nil {
		t.Fatalf("cannot connect to created database: %s", err)
	}
	if err := testdb.Ping(); err != nil {
		t.Fatalf("cannot ping test database: %s", err)
	}

	if err := EnsureSchema(testdb); err != nil {
		t.Fatalf("cannot ensure schema: %s", err)
	}

	cleanup = func() {
		testdb.Close()
		if _, err := rootdb.Exec("DROP DATABASE " + opts.DBName); err != nil {
			t.Logf("cannot delete test database %q: %s", opts.DBName, err)
		}
		rootdb.Close()
	}
	return testdb, cleanup
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}
