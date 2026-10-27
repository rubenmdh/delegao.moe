package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct{ db *sql.DB }

func openStore(dsn string) (*Store, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.MultiStatements = false
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["time_zone"] = "'+00:00'"
	if err := cfg.Apply(mysql.Charset("utf8mb4", "utf8mb4_unicode_ci")); err != nil {
		return nil, err
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database connection failed: %w", err)
	}
	return &Store{db}, nil
}

func (s *Store) migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(), '.delegao.migrate'),30)").Scan(&locked); err != nil {
		return err
	}
	if locked != 1 {
		return fmt.Errorf("could not acquire migration lock")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(), '.delegao.migrate'))")
	if _, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(100) PRIMARY KEY) ENGINE=InnoDB"); err != nil {
		return err
	}
	var applied int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version='001_schema'").Scan(&applied); err != nil {
		return err
	}
	// DDL is deliberately explicit and idempotent: MySQL implicitly commits DDL.
	if applied == 0 {
		source, err := migrations.ReadFile("migrations/001_initial.sql")
		if err != nil {
			return err
		}
		for _, statement := range strings.Split(string(source), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		// The original SQL dump omitted AUTO_INCREMENT and a default for tags.
		for _, table := range []string{"news", "guestbook", "users"} {
			if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+table+" MODIFY id INT NOT NULL AUTO_INCREMENT"); err != nil {
				return err
			}
		}
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE news MODIFY tags VARCHAR(1000) NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		var indexCount int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='news' AND index_name='news_public'").Scan(&indexCount); err != nil {
			return err
		}
		if indexCount == 0 {
			if _, err := s.db.ExecContext(ctx, "CREATE INDEX news_public ON news (status,id)"); err != nil {
				return err
			}
		}
		if _, err := s.db.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ('001_schema')"); err != nil {
			return err
		}
	}
	return s.normalizeLegacy(ctx)
}

func (s *Store) normalizeLegacy(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var applied int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version='002_decode_legacy'").Scan(&applied); err != nil {
		return err
	}
	if applied > 0 {
		return nil
	}
	// Normalize the old PHP storage format once. Templates escape at output time.
	for _, spec := range []struct {
		table   string
		columns []string
	}{{"news", []string{"title", "posted_by"}}, {"guestbook", []string{"name", "email", "comment", "country"}}} {
		rows, err := tx.QueryContext(ctx, "SELECT id,"+strings.Join(spec.columns, ",")+" FROM "+spec.table)
		if err != nil {
			return err
		}
		type record struct {
			id     int64
			values []string
		}
		var records []record
		for rows.Next() {
			rec := record{values: make([]string, len(spec.columns))}
			dest := []any{&rec.id}
			for i := range rec.values {
				dest = append(dest, &rec.values[i])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return err
			}
			records = append(records, rec)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		sets := make([]string, len(spec.columns))
		for i, c := range spec.columns {
			sets[i] = c + "=?"
		}
		// Preserve the timestamp: normalizing entities is not an editorial update.
		if spec.table == "news" {
			sets = append(sets, "last_update=last_update")
		}
		for _, rec := range records {
			args := make([]any, 0, len(rec.values)+1)
			for _, value := range rec.values {
				args = append(args, html.UnescapeString(value))
			}
			args = append(args, rec.id)
			if _, err := tx.ExecContext(ctx, "UPDATE "+spec.table+" SET "+strings.Join(sets, ",")+" WHERE id=?", args...); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ('002_decode_legacy')"); err != nil {
		return err
	}
	return tx.Commit()
}

const postColumns = "id, title, body, status, tags, posted_by, created_at, last_update"

type scanner interface{ Scan(...any) error }

func scanPost(row scanner) (Post, error) {
	var p Post
	var status int
	err := row.Scan(&p.ID, &p.Title, &p.Body, &status, &p.Tags, &p.Author, &p.Created, &p.Updated)
	p.Published = status == 1
	return p, err
}
func (s *Store) posts(ctx context.Context, admin bool, page int) ([]Post, error) {
	// Lists need only an excerpt; the admin table does not need the body at all.
	body := "LEFT(body,250)"
	if admin {
		body = "''"
	}
	columns := strings.Replace(postColumns, "body", body, 1)
	query := "SELECT " + columns + " FROM news"
	if !admin {
		query += " WHERE status=1"
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY id DESC LIMIT 21 OFFSET ?", (page-1)*20)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) latestPost(ctx context.Context) (Post, error) {
	return scanPost(s.db.QueryRowContext(ctx, "SELECT "+postColumns+" FROM news WHERE status=1 ORDER BY id DESC LIMIT 1"))
}
func (s *Store) post(ctx context.Context, id int64, admin bool) (Post, error) {
	query := "SELECT " + postColumns + " FROM news WHERE id=?"
	if !admin {
		query += " AND status=1"
	}
	return scanPost(s.db.QueryRowContext(ctx, query, id))
}
func (s *Store) savePost(ctx context.Context, p Post) error {
	if p.ID == 0 {
		_, err := s.db.ExecContext(ctx, "INSERT INTO news (title,body,status,tags,posted_by) VALUES (?,?,?,?,?)", p.Title, p.Body, p.Published, p.Tags, p.Author)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE news SET title=?,body=?,status=?,tags=?,last_update=CURRENT_TIMESTAMP WHERE id=?", p.Title, p.Body, p.Published, p.Tags, p.ID)
	return err
}
func (s *Store) guests(ctx context.Context, page int) ([]Guest, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,email,comment,country,date FROM guestbook ORDER BY id DESC LIMIT 21 OFFSET ?", (page-1)*20)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.Name, &g.Email, &g.Comment, &g.Country, &g.Date); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
func (s *Store) addGuest(ctx context.Context, g Guest) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO guestbook (name,email,comment,country) VALUES (?,?,?,?)", g.Name, g.Email, g.Comment, g.Country)
	return err
}
func (s *Store) user(ctx context.Context, name string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, "SELECT id,username,password FROM users WHERE username=?", name).Scan(&u.ID, &u.Username, &u.Password)
	return u, err
}
func (s *Store) delete(ctx context.Context, table string, id int64) error {
	if table != "news" && table != "guestbook" {
		return fmt.Errorf("invalid table")
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}
