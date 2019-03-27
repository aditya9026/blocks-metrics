package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

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

	vID, err := s.EnsureValidator(ctx, []byte{0x01, 0, 0xbe, 'a'})
	if err != nil {
		t.Fatalf("cannot create a validator: %s", err)
	}

	for i := 5; i < 100; i += 20 {
		block := &Block{
			Height: int64(i),
			Hash:   []byte{0, 1, byte(i)},
			// Postgres TIMESTAMPTZ precision is microseconds.
			Time:       time.Now().UTC().Round(time.Microsecond),
			ProposerID: vID,
		}
		if err := s.InsertBlock(ctx, block.Height, block.Hash, block.Time, block.ProposerID); err != nil {
			t.Fatalf("cannot inser block: %s", err)
		}

		if got, err := s.LatestBlock(ctx); err != nil {
			t.Fatalf("cannot get latest block: %s", err)
		} else if !reflect.DeepEqual(got, block) {
			t.Logf(" got %#v", got)
			t.Logf("want %#v", block)
			t.Fatal("unexpected result")
		}
	}
}

func TestStoreEnsureValidator(t *testing.T) {
	db, cleanup := ensureDB(t)
	defer cleanup()

	ctx := context.Background()

	s := NewStore(db)

	pubkeyA := []byte{0x01, 0, 0xbe, 'a'}
	aID, err := s.EnsureValidator(ctx, pubkeyA)
	if err != nil {
		t.Fatalf("cannot create 'a' validator: %s", err)
	}

	pubkeyB := []byte{0x01, 0, 0xbe, 'b'}
	bID, err := s.EnsureValidator(ctx, pubkeyB)
	if err != nil {
		t.Fatalf("cannot create 'b' validator: %s", err)
	}

	if aID2, err := s.EnsureValidator(ctx, pubkeyA); err != nil {
		t.Fatalf("cannot ensure 'a' validator: %s", err)
	} else if aID != aID2 {
		t.Fatalf("'a' validator ID missmatch %d != %d", aID, aID2)
	}

	if bID2, err := s.EnsureValidator(ctx, pubkeyB); err != nil {
		t.Fatalf("cannot ensure 'b' validator: %s", err)
	} else if bID != bID2 {
		t.Fatalf("'b' validator ID missmatch %d != %d", bID, bID2)
	}
}

func TestStoreInsertBlock(t *testing.T) {
	db, cleanup := ensureDB(t)
	defer cleanup()

	ctx := context.Background()

	s := NewStore(db)

	vid, err := s.EnsureValidator(ctx, []byte{0x01, 0, 0xbe})
	if err != nil {
		t.Fatalf("cannot ensure validator: %s", err)
	}

	if err := s.InsertBlock(ctx, 1, []byte{0, 1, 2}, time.Now(), vid); err != nil {
		t.Errorf("cannot inser block: %s", err)
	}

	if err := s.InsertBlock(ctx, 1, []byte{0, 1, 2}, time.Now(), vid); !ErrConflict.Is(err) {
		t.Errorf("was able to create a block duplicate: %s", err)
	}
	if err := s.InsertBlock(ctx, 2, []byte{0, 1, 2}, time.Now(), 1491249); !ErrConflict.Is(err) {
		t.Errorf("was able to create a block with a non existing proposer: %s", err)
	}

	if err := s.InsertBlock(ctx, 2, []byte{0, 1, 3}, time.Now(), vid); err != nil {
		t.Error("cannot inser block")
	}
}

func TestStoreMarkBlock(t *testing.T) {
	db, cleanup := ensureDB(t)
	defer cleanup()

	ctx := context.Background()

	s := NewStore(db)

	vid1, err := s.EnsureValidator(ctx, []byte{0x01, 0, 0xbe, 1, 1, 1})
	if err != nil {
		t.Fatalf("cannot ensure validator 1: %s", err)
	}

	vid2, err := s.EnsureValidator(ctx, []byte{0x01, 0, 0xbe, 2, 2, 2})
	if err != nil {
		t.Fatalf("cannot ensure validator 2: %s", err)
	}

	if err := s.InsertBlock(ctx, 1, []byte{0, 1, 2}, time.Now(), vid1); err != nil {
		t.Fatalf("cannot inser block for validator 1")
	}

	if err := s.InsertBlock(ctx, 2, []byte{0, 2, 1}, time.Now(), vid2); err != nil {
		t.Fatalf("cannot inser block for validator 2")
	}

	if err := s.MarkBlock(ctx, 1, vid1, true); err != nil {
		t.Fatalf("cannot mark a block: %s", err)
	}
	if err := s.MarkBlock(ctx, 1, vid1, true); err != nil {
		t.Fatalf("cannot re-mark a block: %s", err)
	}
	if err := s.MarkBlock(ctx, 1, vid1, false); err != nil {
		t.Fatalf("cannot re-mark a block: %s", err)
	}

	if err := s.MarkBlock(ctx, 4129, vid1, true); !ErrConflict.Is(err) {
		t.Errorf("was able to create mark a non existing block: %q", err)
	}
	if err := s.MarkBlock(ctx, 1, 29144192, true); !ErrConflict.Is(err) {
		t.Errorf("was able to make a block for a non existing validator: %q", err)
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
		User    string
		Port    string
		Host    string
		SSLMode string
		DBName  string
	}{
		User:    env("POSTGRES_TEST_USER", "postgres"),
		Port:    env("POSTGRES_TEST_PORT", "5432"),
		Host:    env("POSTGRES_TEST_HOST", "localhost"),
		SSLMode: env("POSTGRES_TEST_SSLMODE", "disable"),
		DBName: env("POSTGRES_TEST_DATABASE",
			fmt.Sprintf("test_database_%d", time.Now().UnixNano())),
	}

	rootDsn := fmt.Sprintf(
		"host='%s' port='%s' user='%s' dbname='postgres' sslmode='%s'",
		opts.Host, opts.Port, opts.User, opts.SSLMode)
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
		"host='%s' port='%s' user='%s' dbname='%s' sslmode='%s'",
		opts.Host, opts.Port, opts.User, opts.DBName, opts.SSLMode)
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
