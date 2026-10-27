package main

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func sqliteTestStore(t *testing.T) *Store {
	t.Helper()
	name := "site #% space.db"
	if runtime.GOOS != "windows" {
		name = "site ?#% space.db"
	}
	s, err := openSQLite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func TestSQLiteHTTPFlows(t *testing.T) { testHTTPFlows(t, sqliteTestStore(t)) }
func TestSQLiteAdminDashboardAndFilters(t *testing.T) {
	testAdminDashboardAndFilters(t, sqliteTestStore(t))
}
func TestSQLiteMixedPublicationHTTP(t *testing.T) { testMixedPublicationHTTP(t, sqliteTestStore(t)) }

func TestSQLiteDurabilityMigrationsAndDates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "site.db")
	s, err := openConfiguredStore("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	ctx := context.Background()
	for _, test := range []struct {
		pragma string
		want   int
	}{{"synchronous", 2}, {"busy_timeout", 5000}, {"foreign_keys", 1}, {"cache_size", -2048}} {
		var got int
		if err := s.db.QueryRow("PRAGMA " + test.pragma).Scan(&got); err != nil || got != test.want {
			t.Fatalf("%s=%d: %v", test.pragma, got, err)
		}
	}
	var journal string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("journal: %s %v", journal, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("database permissions are not private")
		}
	}
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := s.savePostRecord(ctx, Post{Title: "Literal &amp; entity", Body: "**Hello**", Author: "admin", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	post, err := s.post(ctx, id, true)
	if err != nil || post.Title != "Literal &amp; entity" || post.Created.IsZero() || post.Updated != nil {
		t.Fatalf("post: %+v %v", post, err)
	}
	post.Body = "Updated"
	if err := s.savePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	post, err = s.post(ctx, id, true)
	if err != nil || post.Updated == nil || post.Updated.IsZero() {
		t.Fatal("update timestamp missing")
	}
	if _, err := s.db.Exec("INSERT INTO users(username,password)VALUES('Admin','hash')"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO users(username,password)VALUES('admin','another')"); err == nil {
		t.Fatal("case-duplicate usernames accepted")
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.db.Close()
	p, err := restored.post(ctx, id, true)
	if err != nil || p.Body != "Updated" {
		t.Fatalf("restart data: %+v %v", p, err)
	}
}

func TestSQLiteUnicodeSearchAndSnapshot(t *testing.T) {
	s := sqliteTestStore(t)
	ctx := context.Background()
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Post{{Title: "ÁRBOL", Body: "first", Author: "admin", Published: true}, {Title: "Second", Body: "second", Author: "admin", Published: true}} {
		if err := s.savePost(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	var bodies []string
	if err := s.publicationPosts(ctx, func(p Post) error {
		bodies = append(bodies, p.Body)
		if len(bodies) == 1 {
			_, err := s.db.Exec("UPDATE news SET body='changed' WHERE id=1")
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[1] != "first" {
		t.Fatalf("inconsistent SQLite snapshot: %v", bodies)
	}
	a := testApp(t, s)
	issued := httptest.NewRecorder()
	a.security.issue(issued, httptest.NewRequest("GET", "/admin", nil), "admin")
	r := httptest.NewRequest("GET", "/admin/posts?q=arbol", nil)
	r.AddCookie(issued.Result().Cookies()[0])
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "ÁRBOL") || strings.Contains(w.Body.String(), ">Second</a>") {
		t.Fatalf("unicode search: %d %s", w.Code, w.Body.String())
	}
}

func TestDatabaseTimeVariants(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 34, 56, 123456000, time.UTC)
	for _, value := range []any{now, "2026-10-27 12:34:56.123456", []byte("2026-10-27T12:34:56.123456Z"), "2026-10-27 12:34:56.123456 +0000 UTC"} {
		var got databaseTime
		if err := got.Scan(value); err != nil || !got.Valid || !got.Time.Equal(now) {
			t.Fatalf("%v => %+v %v", value, got, err)
		}
	}
	var got databaseTime
	if err := got.Scan(nil); err != nil || got.Valid {
		t.Fatal("nullable date")
	}
	if err := got.Scan("invalid"); err == nil {
		t.Fatal("invalid date accepted")
	}
}

func TestSQLiteConfigurationRejectsUnsafeTargets(t *testing.T) {
	for _, path := range []string{"", ":memory:", "file:test?mode=memory", t.TempDir()} {
		s, err := openSQLite(path)
		if s != nil {
			s.db.Close()
		}
		if err == nil {
			t.Fatalf("unsafe target accepted: %q", path)
		}
	}
	if _, err := openConfiguredStore("unknown", "dsn"); err == nil {
		t.Fatal("unknown database driver accepted")
	}
}

var _ sql.Scanner = (*databaseTime)(nil)
