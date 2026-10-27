package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
)

// ImportSummary reports committed records, never credentials or password hashes.
type ImportSummary struct {
	Posts, Guests, Users int64
}

// importMySQL copies a consistent, read-only MySQL snapshot into an empty SQLite
// database. The caller migrates the SQLite schema first and keeps the web service
// stopped while importing. No schema or data is ever changed on the source.
func importMySQL(ctx context.Context, source, target *Store) (ImportSummary, error) {
	var empty ImportSummary
	if source == nil || source.db == nil || source.dialect != "mysql" {
		return empty, errors.New("the import source must be MySQL")
	}
	if target == nil || target.db == nil || target.dialect != "sqlite" {
		return empty, errors.New("the import destination must be SQLite")
	}
	src, err := source.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return empty, fmt.Errorf("open read-only import snapshot: %w", err)
	}
	defer src.Rollback()
	if err := checkImportSourceTables(ctx, src); err != nil {
		return empty, err
	}
	decode, err := sourceNeedsEntityDecoding(ctx, src)
	if err != nil {
		return empty, err
	}
	expected, err := importCounts(ctx, src)
	if err != nil {
		return empty, fmt.Errorf("count source records: %w", err)
	}
	dst, err := target.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, fmt.Errorf("open destination transaction: %w", err)
	}
	defer dst.Rollback()
	before, err := importCounts(ctx, dst)
	if err != nil {
		return empty, fmt.Errorf("check destination records: %w", err)
	}
	if before != empty {
		return empty, errors.New("SQLite destination contains data; import requires empty news, guestbook and users tables")
	}
	var imported ImportSummary
	if imported.Users, err = importUsers(ctx, src, dst); err != nil {
		return empty, err
	}
	if imported.Posts, err = importPosts(ctx, src, dst, decode); err != nil {
		return empty, err
	}
	if imported.Guests, err = importGuests(ctx, src, dst, decode); err != nil {
		return empty, err
	}
	after, err := importCounts(ctx, dst)
	if err != nil {
		return empty, fmt.Errorf("verify imported records: %w", err)
	}
	if imported != expected || after != expected {
		return empty, errors.New("imported record counts do not match the source snapshot")
	}
	if err := src.Commit(); err != nil {
		return empty, fmt.Errorf("finish source snapshot: %w", err)
	}
	if err := dst.Commit(); err != nil {
		return empty, fmt.Errorf("commit SQLite import: %w", err)
	}
	return imported, nil
}

// A consistent snapshot needs transactional tables. The PHP schema used InnoDB;
// silently accepting nontransactional tables could lose concurrent writes.
func checkImportSourceTables(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT table_name,engine FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('news','guestbook','users')")
	if err != nil {
		return fmt.Errorf("inspect source tables: %w", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var name string
		var engine sql.NullString
		if err := rows.Scan(&name, &engine); err != nil {
			return fmt.Errorf("inspect source table: %w", err)
		}
		if !engine.Valid || !strings.EqualFold(engine.String, "InnoDB") {
			return fmt.Errorf("source table %s must use InnoDB for a consistent import", name)
		}
		found++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect source tables: %w", err)
	}
	if found != 3 {
		return errors.New("source must contain news, guestbook and users tables")
	}
	return nil
}

func sourceNeedsEntityDecoding(ctx context.Context, tx *sql.Tx) (bool, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='schema_migrations'").Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect source migration metadata: %w", err)
	}
	if exists == 0 {
		return true, nil
	}
	var decoded int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version='002_decode_legacy'").Scan(&decoded); err != nil {
		return false, fmt.Errorf("read source migration metadata: %w", err)
	}
	return decoded == 0, nil
}

func importCounts(ctx context.Context, tx *sql.Tx) (ImportSummary, error) {
	var counts ImportSummary
	err := tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM news),(SELECT COUNT(*) FROM guestbook),(SELECT COUNT(*) FROM users)").Scan(&counts.Posts, &counts.Guests, &counts.Users)
	return counts, err
}

func importedTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC().Format("2006-01-02 15:04:05.999999999")
}

func importUsers(ctx context.Context, src, dst *sql.Tx) (int64, error) {
	rows, err := src.QueryContext(ctx, "SELECT id,username,password,created_at FROM users ORDER BY id")
	if err != nil {
		return 0, fmt.Errorf("read source users: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id int64
		var username, password string
		var created sql.NullTime
		if err := rows.Scan(&id, &username, &password, &created); err != nil {
			return 0, fmt.Errorf("read source user: %w", err)
		}
		if id < 1 {
			return 0, errors.New("source contains an invalid user id")
		}
		if _, err := dst.ExecContext(ctx, "INSERT INTO users (id,username,password,created_at) VALUES (?,?,?,?)", id, username, password, importedTime(created)); err != nil {
			return 0, fmt.Errorf("import user %d: %w", id, err)
		}
		count++
	}
	return count, rows.Err()
}

func importPosts(ctx context.Context, src, dst *sql.Tx, decode bool) (int64, error) {
	rows, err := src.QueryContext(ctx, "SELECT "+postColumns+" FROM news ORDER BY id")
	if err != nil {
		return 0, fmt.Errorf("read source posts: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id int64
		var title, body, tags, author string
		var status int
		var created time.Time
		var updated sql.NullTime
		if err := rows.Scan(&id, &title, &body, &status, &tags, &author, &created, &updated); err != nil {
			return 0, fmt.Errorf("read source post: %w", err)
		}
		if id < 1 || created.IsZero() {
			return 0, fmt.Errorf("source post %d has an invalid id or creation date", id)
		}
		if decode {
			title = html.UnescapeString(title)
			author = html.UnescapeString(author)
		}
		if _, err := dst.ExecContext(ctx, "INSERT INTO news (id,title,body,status,tags,posted_by,created_at,last_update) VALUES (?,?,?,?,?,?,?,?)", id, title, body, status, tags, author, created.UTC().Format("2006-01-02 15:04:05.999999999"), importedTime(updated)); err != nil {
			return 0, fmt.Errorf("import post %d: %w", id, err)
		}
		count++
	}
	return count, rows.Err()
}

func importGuests(ctx context.Context, src, dst *sql.Tx, decode bool) (int64, error) {
	rows, err := src.QueryContext(ctx, "SELECT id,name,email,comment,country,date FROM guestbook ORDER BY id")
	if err != nil {
		return 0, fmt.Errorf("read source guestbook: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id int64
		var name, email, comment, country string
		var date time.Time
		if err := rows.Scan(&id, &name, &email, &comment, &country, &date); err != nil {
			return 0, fmt.Errorf("read source guestbook entry: %w", err)
		}
		if id < 1 || date.IsZero() {
			return 0, fmt.Errorf("source guestbook entry %d has an invalid id or date", id)
		}
		if decode {
			name = html.UnescapeString(name)
			email = html.UnescapeString(email)
			comment = html.UnescapeString(comment)
			country = html.UnescapeString(country)
		}
		if _, err := dst.ExecContext(ctx, "INSERT INTO guestbook (id,name,email,comment,country,date) VALUES (?,?,?,?,?,?)", id, name, email, comment, country, date.UTC().Format("2006-01-02 15:04:05.999999999")); err != nil {
			return 0, fmt.Errorf("import guestbook entry %d: %w", id, err)
		}
		count++
	}
	return count, rows.Err()
}
