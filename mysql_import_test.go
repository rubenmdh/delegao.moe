package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func importSQLiteTarget(t *testing.T) *Store {
	t.Helper()
	target, err := openSQLite(filepath.Join(t.TempDir(), "import.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.db.Close() })
	if err := target.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return target
}

func seedImportSource(t *testing.T, legacy bool) (*Store, string) {
	t.Helper()
	source := integrationStore(t)
	if err := source.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The original PHP schema permits NULL here, unlike the fresh Go schema.
	if _, err := source.db.Exec("ALTER TABLE users MODIFY created_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		t.Fatal(err)
	}
	if legacy {
		if _, err := source.db.Exec("DROP TABLE schema_migrations"); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("existing-php-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	phpHash := strings.Replace(string(hash), "$2a$", "$2y$", 1)
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users(id,username,password,created_at) VALUES (7,?,?,?)", []any{"admin", phpHash, "2020-01-02 03:04:05"}},
		{"INSERT INTO users(id,username,password,created_at) VALUES (9,?,?,NULL)", []any{"another", phpHash}},
		{"INSERT INTO news(id,title,body,status,tags,posted_by,created_at,last_update) VALUES (42,?,?,1,?,?,?,?)", []any{"Rock &amp; Roll &amp;amp;", "**Markdown** &amp; <raw html>", "literal &amp; tag", "author&amp;name", "2020-01-02 03:04:05", "2021-02-03 04:05:06"}},
		{"INSERT INTO news(id,title,body,status,tags,posted_by,created_at,last_update) VALUES (43,?,?,0,'',?,?,NULL)", []any{"A private draft", "private body", "admin", "2020-02-03 04:05:06"}},
		{"INSERT INTO guestbook(id,name,email,comment,country,date) VALUES (15,?,?,?,?,?)", []any{"Visitor &amp; Friend", "visitor@example.com", "&lt;hi&gt; &amp;amp;", "Spain", "2020-04-05 06:07:08"}},
	}
	for _, statement := range statements {
		if _, err := source.db.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	return source, phpHash
}

func TestImportRequiresCorrectDatabaseEngines(t *testing.T) {
	target := importSQLiteTarget(t)
	if _, err := importMySQL(context.Background(), nil, target); err == nil {
		t.Fatal("missing MySQL source was accepted")
	}
	if _, err := importMySQL(context.Background(), target, target); err == nil {
		t.Fatal("SQLite source was accepted as MySQL")
	}
	if _, err := importMySQL(context.Background(), &Store{db: target.db, dialect: "mysql"}, nil); err == nil {
		t.Fatal("missing SQLite target was accepted")
	}
}

func TestIntegrationMySQLImportPreservesLegacyData(t *testing.T) {
	source, password := seedImportSource(t, true)
	target := importSQLiteTarget(t)
	ctx := context.Background()
	counts, err := importMySQL(ctx, source, target)
	if err != nil {
		t.Fatal(err)
	}
	if counts != (ImportSummary{Posts: 2, Guests: 1, Users: 2}) {
		t.Fatalf("wrong committed counts: %+v", counts)
	}
	p, err := target.post(ctx, 42, true)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
	if p.ID != 42 || p.Title != "Rock & Roll &amp;" || p.Author != "author&name" || p.Body != "**Markdown** &amp; <raw html>" || p.Tags != "literal &amp; tag" || !p.Published || !p.Created.Equal(created) || p.Updated == nil || !p.Updated.Equal(updated) {
		t.Fatalf("post content or dates changed unexpectedly: %+v", p)
	}
	draft, err := target.post(ctx, 43, true)
	if err != nil || draft.Published || draft.Updated != nil {
		t.Fatalf("draft not preserved: %+v %v", draft, err)
	}
	if _, err := target.post(ctx, 43, false); err != sql.ErrNoRows {
		t.Fatalf("draft became public: %v", err)
	}
	guests, err := target.guests(ctx, 1)
	if err != nil || len(guests) != 1 || guests[0].ID != 15 || guests[0].Name != "Visitor & Friend" || guests[0].Comment != "<hi> &amp;" || !guests[0].Date.Equal(time.Date(2020, 4, 5, 6, 7, 8, 0, time.UTC)) {
		t.Fatalf("guest not preserved: %+v %v", guests, err)
	}
	u, err := target.user(ctx, "admin")
	if err != nil || u.ID != 7 || u.Password != password || bcrypt.CompareHashAndPassword([]byte(strings.Replace(u.Password, "$2y$", "$2a$", 1)), []byte("existing-php-password")) != nil {
		t.Fatalf("existing PHP password was not preserved: user id=%d, error=%v", u.ID, err)
	}
	var nullable any
	if err := target.db.QueryRow("SELECT created_at FROM users WHERE id=9").Scan(&nullable); err != nil || nullable != nil {
		t.Fatalf("nullable user creation date changed: %v %v", nullable, err)
	}
	var sourceTitle, sourceAuthor, sourcePassword, sourceComment string
	if err := source.db.QueryRow("SELECT title,posted_by FROM news WHERE id=42").Scan(&sourceTitle, &sourceAuthor); err != nil {
		t.Fatal(err)
	}
	if err := source.db.QueryRow("SELECT password FROM users WHERE id=7").Scan(&sourcePassword); err != nil {
		t.Fatal(err)
	}
	if err := source.db.QueryRow("SELECT comment FROM guestbook WHERE id=15").Scan(&sourceComment); err != nil {
		t.Fatal(err)
	}
	if sourceTitle != "Rock &amp; Roll &amp;amp;" || sourceAuthor != "author&amp;name" || sourcePassword != password || sourceComment != "&lt;hi&gt; &amp;amp;" {
		t.Fatal("read-only import changed the source")
	}
	var sourceMetadataTables int
	if err := source.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='schema_migrations'").Scan(&sourceMetadataTables); err != nil || sourceMetadataTables != 0 {
		t.Fatal("import created migration metadata in the source")
	}
	if _, err := importMySQL(ctx, source, target); err == nil {
		t.Fatal("repeated import overwrote a populated target")
	}
	id, err := target.savePostRecord(ctx, Post{Title: "After import", Body: "new", Author: "admin"})
	if err != nil || id != 44 {
		t.Fatalf("post id sequence did not advance: %d %v", id, err)
	}
	if err := target.addGuest(ctx, Guest{Name: "Next visitor", Comment: "hello", Country: "Spain"}); err != nil {
		t.Fatal(err)
	}
	var nextGuestID int64
	if err := target.db.QueryRow("SELECT MAX(id) FROM guestbook").Scan(&nextGuestID); err != nil || nextGuestID != 16 {
		t.Fatalf("guest id sequence did not advance: %d %v", nextGuestID, err)
	}
}

func TestIntegrationMySQLImportDoesNotDecodeNewGoData(t *testing.T) {
	source, _ := seedImportSource(t, false)
	target := importSQLiteTarget(t)
	if _, err := importMySQL(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
	p, err := target.post(context.Background(), 42, true)
	if err != nil || p.Title != "Rock &amp; Roll &amp;amp;" || p.Author != "author&amp;name" {
		t.Fatalf("already-normalized Go data was decoded again: %+v %v", p, err)
	}
	guests, err := target.guests(context.Background(), 1)
	if err != nil || guests[0].Comment != "&lt;hi&gt; &amp;amp;" {
		t.Fatalf("already-normalized guest data was decoded again: %+v %v", guests, err)
	}
}

func TestIntegrationMySQLImportRollsBackEveryTable(t *testing.T) {
	source, _ := seedImportSource(t, true)
	target := importSQLiteTarget(t)
	// Both users and the first post have been copied when the trigger rejects
	// the second post. All of them must roll back together.
	if _, err := target.db.Exec("CREATE TRIGGER reject_import BEFORE INSERT ON news WHEN NEW.id=43 BEGIN SELECT RAISE(ABORT,'simulated import failure'); END"); err != nil {
		t.Fatal(err)
	}
	if summary, err := importMySQL(context.Background(), source, target); err == nil || summary != (ImportSummary{}) {
		t.Fatalf("write failure did not fail the complete import: %+v %v", summary, err)
	}
	for _, table := range []string{"users", "news", "guestbook"} {
		var count int64
		if err := target.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial data remained in %s: %d %v", table, count, err)
		}
	}
}

func TestIntegrationMySQLImportPreservesNonPublicLegacyStatus(t *testing.T) {
	source, _ := seedImportSource(t, true)
	target := importSQLiteTarget(t)
	if _, err := source.db.Exec("UPDATE news SET status=2 WHERE id=43"); err != nil {
		t.Fatal(err)
	}
	if _, err := importMySQL(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
	var status int
	if err := target.db.QueryRow("SELECT status FROM news WHERE id=43").Scan(&status); err != nil || status != 2 {
		t.Fatalf("legacy status changed: %d %v", status, err)
	}
	if _, err := target.post(context.Background(), 43, false); err != sql.ErrNoRows {
		t.Fatalf("nonpublic legacy status became public: %v", err)
	}
}

func TestIntegrationMySQLImportRejectsNonTransactionalSource(t *testing.T) {
	source, _ := seedImportSource(t, true)
	target := importSQLiteTarget(t)
	if _, err := source.db.Exec("ALTER TABLE guestbook ENGINE=MyISAM"); err != nil {
		t.Fatal(err)
	}
	if _, err := importMySQL(context.Background(), source, target); err == nil || !strings.Contains(err.Error(), "InnoDB") {
		t.Fatalf("nontransactional snapshot was accepted: %v", err)
	}
}

func TestIntegrationMySQLImportRefusesExistingAdministrator(t *testing.T) {
	source, _ := seedImportSource(t, true)
	target := importSQLiteTarget(t)
	if _, err := target.db.Exec("INSERT INTO users(id,username,password) VALUES(1,'local-admin','existing-hash')"); err != nil {
		t.Fatal(err)
	}
	if _, err := importMySQL(context.Background(), source, target); err == nil || !strings.Contains(err.Error(), "contains data") {
		t.Fatalf("partially populated target was accepted: %v", err)
	}
	var username, hash string
	if err := target.db.QueryRow("SELECT username,password FROM users WHERE id=1").Scan(&username, &hash); err != nil || username != "local-admin" || hash != "existing-hash" {
		t.Fatal("refused import changed the existing administrator")
	}
	var count int64
	if err := target.db.QueryRow("SELECT COUNT(*) FROM news").Scan(&count); err != nil || count != 0 {
		t.Fatal("refused import inserted posts")
	}
}
