package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func testApp(t *testing.T, store *Store) *App {
	t.Helper()
	a, err := newApp(store, false, "Ravioli")
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestStaticPagesAndIsolation(t *testing.T) {
	a := testApp(t, nil)
	handler := a.handler()
	for _, path := range []string{"/about-me", "/projects", "/music", "/games", "/contact", "/donate", "/keys", "/login", "/css/custom.css", "/img/delegao.jpg", "/assets/delegao_public_key.asc"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 200 {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("missing CSP")
			}
			if path == "/games" && !strings.Contains(w.Body.String(), "let's arrange a game") {
				t.Fatal("page content was lost")
			}
		})
	}
	for _, path := range []string{"/config.php", "/go.mod", "/.env", "/design/refresh/index.html", "/css/", "/functions/delete-post-db.php?id=1", "/install.php", "/logout"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 && w.Code != 405 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/admin/posts", nil))
	if w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("admin is not protected")
	}
}
func TestMarkdownAndEscaping(t *testing.T) {
	a := testApp(t, nil)
	for _, source := range []string{"<script>alert(1)</script>", "<img src=x onerror=alert(1)>", "[link](javascript:alert%281%29)", "[link](data:text/html,test)"} {
		html := string(a.renderMarkdown(source))
		if strings.Contains(html, "<script") || strings.Contains(html, "onerror=") || strings.Contains(html, "href=\"javascript:") || strings.Contains(html, "href=\"data:") {
			t.Fatalf("unsafe markdown: %s", html)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	a.render(w, r, 200, View{Page: "posts", Single: true, Posts: []Post{{Title: "<script>alert(1)</script>", Body: "**hello**", Created: time.Now()}}})
	html := w.Body.String()
	if w.Code != 200 || strings.Contains(html, "<script>alert") || !strings.Contains(html, "<strong>hello</strong>") {
		t.Fatalf("escaping/rendering failed: %s", html)
	}
}
func TestCSRFSessionRotationAndRateLimit(t *testing.T) {
	s := newSecurity(true)
	r := httptest.NewRequest("GET", "https://example.com/login", nil)
	w := httptest.NewRecorder()
	sess, ok := s.issue(w, r, "")
	if !ok {
		t.Fatal("issue")
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("cookie flags")
	}
	post := func(token, origin string) bool {
		r := httptest.NewRequest("POST", "https://example.com/login", strings.NewReader(url.Values{"csrf": {token}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.AddCookie(cookie)
		return s.csrf(httptest.NewRecorder(), r)
	}
	if post("", "") || post("invalid", "") || post(sess.CSRF, "https://evil.example") {
		t.Fatal("invalid CSRF accepted")
	}
	if !post(sess.CSRF, "https://example.com") {
		t.Fatal("valid CSRF rejected")
	}
	r.AddCookie(cookie)
	if _, ok := s.issue(httptest.NewRecorder(), r, "admin"); !ok {
		t.Fatal("rotation")
	}
	if _, ok := s.current(r); ok {
		t.Fatal("old session survived login")
	}
	r2 := httptest.NewRequest("POST", "/login", nil)
	r2.RemoteAddr = "127.0.0.1:12345"
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() { defer wg.Done(); s.allow(r2, "login", 10) }()
	}
	wg.Wait()
	if s.allow(r2, "login", 10) {
		t.Fatal("rate limit failed")
	}
}
func TestSlugAndNamedRoutes(t *testing.T) {
	for source, want := range map[string]string{"¡Hola, Rubén!": "hola-ruben", "  Go & PHP  ": "go-php", "!!!": "post", "東京": "東京"} {
		if got := slug(source); got != want {
			t.Errorf("%q => %q, want %q", source, got, want)
		}
	}
	if got := route("posts.show", 42, slug("Hola Go")); got != "/posts/42/hola-go" {
		t.Fatal(got)
	}
}

func TestTrustedProxyAddress(t *testing.T) {
	proxies, err := parseProxies("127.0.0.1/32,10.0.0.0/8,::1/128")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{proxies: proxies}
	for _, test := range []struct{ remote, forwarded, want string }{
		{"198.51.100.1:1234", "203.0.113.7", "198.51.100.1:1234"},
		{"127.0.0.1:1234", "203.0.113.7", "203.0.113.7:0"},
		{"127.0.0.1:1234", "192.0.2.5, 203.0.113.7, 10.0.0.2", "203.0.113.7:0"},
		{"127.0.0.1:1234", "invalid", "127.0.0.1:1234"},
		{"[::1]:1234", "2001:db8::1", "[2001:db8::1]:0"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = test.remote
		r.Header.Set("X-Forwarded-For", test.forwarded)
		if got := a.clientAddress(r); got != test.want {
			t.Errorf("%+v: got %s", test, got)
		}
	}
	if _, err := parseProxies("*"); err == nil {
		t.Fatal("invalid trusted proxy accepted")
	}
}

// TEST_DATABASE_DSN must allow creating databases. Each run owns and drops only
// a uniquely named test database; it never modifies the database in the DSN.
func integrationStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN for real MySQL/MariaDB integration tests")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	cfg.MultiStatements = false
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("delegao_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.DBName = name
	store, err := openStore(cfg.FormatDSN())
	if err != nil {
		admin.Exec("DROP DATABASE " + name)
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		store.db.Close()
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return store
}
func TestIntegrationLegacyMigration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	source, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the original dump: ids have no AUTO_INCREMENT, tags no default.
	legacy := strings.ReplaceAll(string(source), " AUTO_INCREMENT", "")
	legacy = regexp.MustCompile(`,\s*INDEX news_public \(status,id\)`).ReplaceAllString(legacy, "")
	legacy = strings.ReplaceAll(legacy, "tags VARCHAR(1000) NOT NULL DEFAULT ''", "tags VARCHAR(1000) NOT NULL")
	for _, q := range strings.Split(legacy, ";") {
		if strings.TrimSpace(q) != "" {
			if _, err := store.db.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := store.db.Exec("INSERT INTO news (id,title,body,status,tags,posted_by) VALUES (42,'Hello &amp; Go','**legacy**',1,'','rub&amp;en')"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO guestbook (id,name,email,comment,country) VALUES (9,'A&amp;B','a@example.com','&lt;hi&gt;','Spain')"); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(ctx); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	p, err := store.post(ctx, 42, false)
	if err != nil || p.Title != "Hello & Go" || p.Author != "rub&en" || p.Updated != nil {
		t.Fatalf("legacy post: %+v, %v", p, err)
	}
	gs, err := store.guests(ctx, 1)
	if err != nil || gs[0].Comment != "<hi>" {
		t.Fatalf("legacy guests: %+v %v", gs, err)
	}
	if err := store.savePost(ctx, Post{Title: "A &amp; B", Body: "new", Author: "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ps, err := store.posts(ctx, true, 1)
	if err != nil || ps[0].Title != "A &amp; B" || ps[0].ID != 43 {
		t.Fatalf("new data changed or ids broken: %+v, %v", ps, err)
	}
}

func TestIntegrationHTTPFlows(t *testing.T) {
	testHTTPFlows(t, integrationStore(t))
}

func testHTTPFlows(t *testing.T, store *Store) {
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	phpHash := strings.Replace(string(hash), "$2a$", "$2y$", 1)
	if _, err := store.db.Exec("INSERT INTO users (username,password) VALUES (?,?)", "admin", phpHash); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, store)
	server := httptest.NewServer(a.handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, path string, values url.Values) (int, string, http.Header) {
		t.Helper()
		var body io.Reader
		if values != nil {
			body = strings.NewReader(values.Encode())
		}
		r, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if values != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(data), resp.Header
	}
	csrf := func(path string) string {
		t.Helper()
		code, body, _ := request("GET", path, nil)
		if code != 200 {
			t.Fatalf("%s: %d %s", path, code, body)
		}
		m := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(body)
		if len(m) < 2 {
			t.Fatalf("missing CSRF on %s", path)
		}
		return m[1]
	}
	token := csrf("/login")
	if code, _, _ := request("POST", "/login", url.Values{"username": {"admin"}, "password": {"correct-password"}}); code != 403 {
		t.Fatal("missing CSRF allowed")
	}
	if code, _, _ := request("POST", "/login", url.Values{"csrf": {token}, "username": {"admin"}, "password": {"wrong"}}); code != 401 {
		t.Fatalf("bad credentials: %d", code)
	}
	if code, body, _ := request("POST", "/login", url.Values{"csrf": {token}, "username": {"admin"}, "password": {"correct-password"}}); code != 303 {
		t.Fatalf("PHP bcrypt login: %d %s", code, body)
	}
	u, err := store.user(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := bcrypt.Cost([]byte(u.Password))
	if err != nil || cost != 12 {
		t.Fatal("legacy password was not upgraded")
	}
	token = csrf("/admin/posts/create")
	values := url.Values{"csrf": {token}, "title": {"¡Hola, Rubén!"}, "body": {"**Hello**\n\n<script>alert(1)</script>"}, "status": {"0"}, "postedby": {"forged"}}
	if code, body, _ := request("POST", "/admin/posts", values); code != 303 {
		t.Fatalf("create: %d %s", code, body)
	}
	posts, err := store.posts(ctx, true, 1)
	if err != nil || len(posts) != 1 || posts[0].Author != "admin" {
		t.Fatalf("author or create failed: %+v %v", posts, err)
	}
	p := posts[0]
	if code, _, _ := request("GET", p.URL(), nil); code != 404 {
		t.Fatal("draft is public")
	}
	values.Set("status", "1")
	if code, body, _ := request("POST", route("admin.posts.update", p.ID), values); code != 303 {
		t.Fatalf("update: %d %s", code, body)
	}
	if code, body, _ := request("GET", p.URL(), nil); code != 200 || !strings.Contains(body, "<strong>Hello</strong>") || strings.Contains(body, "<script>alert") {
		t.Fatalf("public post: %d %s", code, body)
	}
	if code, body, _ := request("GET", "/", nil); code != 200 || !strings.Contains(body, "<strong>Hello</strong>") || strings.Contains(body, "&lt; Go back") {
		t.Fatalf("latest post/home: %d %s", code, body)
	}
	for _, path := range []string{fmt.Sprintf("/index.php?id=%d", p.ID), fmt.Sprintf("/index?id=%d", p.ID), fmt.Sprintf("/?id=%d", p.ID), fmt.Sprintf("/posts/%d/old-title", p.ID)} {
		code, _, h := request("GET", path, nil)
		if code != 301 || h.Get("Location") != p.URL() {
			t.Fatalf("legacy redirect %s: %d %s", path, code, h.Get("Location"))
		}
	}
	if code, _, _ := request("GET", "/news.php", nil); code != 301 {
		t.Fatal("news alias")
	}
	token = csrf("/guestbook")
	guest := url.Values{"csrf": {token}, "name": {"<script>visitor</script>"}, "email": {"private@example.com"}, "comment": {strings.Repeat("<", 512)}, "country": {"Spain"}, "dishoftheday": {"wrong"}}
	if code, _, _ := request("POST", "/guestbook", guest); code != 422 {
		t.Fatal("spam accepted")
	}
	guest.Set("dishoftheday", "Ravioli")
	if code, body, _ := request("POST", "/guestbook", guest); code != 303 {
		t.Fatalf("guest create: %d %s", code, body)
	}
	if code, body, _ := request("GET", "/guestbook", nil); code != 200 || strings.Contains(body, "private@example.com") || strings.Contains(body, "<script>visitor") {
		t.Fatalf("guest privacy/escaping: %d", code)
	}
	if code, body, _ := request("GET", "/admin/guestbook", nil); code != 200 || !strings.Contains(body, "private@example.com") {
		t.Fatal("admin guestbook")
	}
	gs, err := store.guests(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, _ := request("POST", route("admin.guests.delete", gs[0].ID), url.Values{"csrf": {token}}); code != 303 {
		t.Fatal("delete guest")
	}
	if code, _, _ := request("GET", route("admin.posts.delete", p.ID), nil); code != 405 && code != 404 {
		t.Fatal("GET deletion")
	}
	if code, _, _ := request("POST", route("admin.posts.delete", p.ID), url.Values{"csrf": {token}}); code != 303 {
		t.Fatal("delete post")
	}
	if code, _, _ := request("GET", p.URL(), nil); code != 404 {
		t.Fatal("deleted post still exists")
	}
	if code, _, _ := request("POST", "/logout", url.Values{"csrf": {token}}); code != 303 {
		t.Fatal("logout")
	}
	if code, _, h := request("GET", "/admin", nil); code != 303 || h.Get("Location") != "/login" {
		t.Fatal("logout did not invalidate session")
	}
	// Pagination must keep drafts private, and fetch one extra row for Next.
	for i := range 22 {
		if err := store.savePost(ctx, Post{Title: fmt.Sprint("Post ", i), Body: "content", Author: "admin", Published: true}); err != nil {
			t.Fatal(err)
		}
	}
	if code, body, _ := request("GET", "/posts", nil); code != 200 || !strings.Contains(body, "?page=2") {
		t.Fatal("pagination missing next")
	}
	if code, body, _ := request("GET", "/posts?page=2", nil); code != 200 || !strings.Contains(body, "?page=1") || strings.Contains(body, "?page=3") {
		t.Fatal("pagination wrong boundary")
	}
}
