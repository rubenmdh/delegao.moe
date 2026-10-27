package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("unicode_fold", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if args[0] == nil {
			return nil, nil
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, errors.New("unicode_fold expects text")
		}
		var out strings.Builder
		for _, r := range norm.NFD.String(strings.ToLower(s)) {
			if !unicode.Is(unicode.Mn, r) {
				out.WriteRune(r)
			}
		}
		return out.String(), nil
	})
}

func openConfiguredStore(driverName, dsn string) (*Store, error) {
	switch strings.ToLower(strings.TrimSpace(driverName)) {
	case "", "mysql":
		return openStore(dsn)
	case "sqlite":
		return openSQLite(dsn)
	default:
		return nil, errors.New("DATABASE_DRIVER must be mysql or sqlite")
	}
}

// SQLite paths are files, not arbitrary connection URLs: callers cannot disable
// durability or point publication at an in-memory/unshared database by mistake.
func openSQLite(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" || path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil, errors.New("SQLite DATABASE_DSN must be a filesystem path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(abs); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("SQLite database must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// Create with private permissions before the driver creates its WAL sidecars.
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(abs, 0600); err != nil {
		return nil, err
	}
	slashPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	address := url.URL{Scheme: "file", Path: slashPath}
	query := url.Values{}
	for _, pragma := range []string{"busy_timeout(5000)", "journal_mode(WAL)", "synchronous(FULL)", "foreign_keys(ON)", "cache_size(-2048)", "wal_autocheckpoint(256)", "journal_size_limit(8388608)"} {
		query.Add("_pragma", pragma)
	}
	address.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", address.String())
	if err != nil {
		return nil, err
	}
	maxOpen, err := envInt("DB_MAX_OPEN_CONNS", 4, 2, 8)
	if err != nil {
		db.Close()
		return nil, err
	}
	maxIdle, err := envInt("DB_MAX_IDLE_CONNS", 1, 0, maxOpen)
	if err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("SQLite connection failed: %w", err)
	}
	return &Store{db: db, dialect: "sqlite"}, nil
}

func (s *Store) migrateSQLite(ctx context.Context) error {
	source, err := migrations.ReadFile("migrations/001_sqlite.sql")
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range strings.Split(string(source), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	// SQLite starts in the new raw-text format. The importer handles PHP decoding.
	for _, version := range []string{"001_schema", "002_decode_legacy"} {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO schema_migrations(version) VALUES (?)", version); err != nil {
			return err
		}
	}
	return tx.Commit()
}
