package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSQLitePasswordChange(t *testing.T)      { testPasswordChange(t, sqliteTestStore(t)) }
func TestIntegrationPasswordChange(t *testing.T) { testPasswordChange(t, integrationStore(t)) }

func testPasswordChange(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	const oldPassword = "original-test-password"
	const newPassword = "a new test passphrase"
	hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"admin", "other"} {
		if _, err := store.db.Exec("INSERT INTO users(username,password) VALUES(?,?)", name, strings.Replace(string(hash), "$2a$", "$2y$", 1)); err != nil {
			t.Fatal(err)
		}
	}
	a := testApp(t, store)
	handler := a.handler()
	issue := func(user string) (*http.Cookie, session) {
		t.Helper()
		w := httptest.NewRecorder()
		sess, ok := a.security.issue(w, httptest.NewRequest("GET", "/admin/password", nil), user)
		if !ok {
			t.Fatal("session issue failed")
		}
		return w.Result().Cookies()[0], sess
	}
	cookie, sess := issue("admin")
	second, _ := issue("admin")
	other, _ := issue("other")
	request := func(method string, values url.Values, c *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/admin/password", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", nil, nil); w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("anonymous password form accessible")
	}
	if w := request("POST", nil, nil); w.Code != 403 {
		t.Fatal("anonymous password POST accepted")
	}
	if w := request("GET", nil, cookie); w.Code != 200 || strings.Count(w.Body.String(), `type="password"`) != 3 || !strings.Contains(w.Body.String(), `autocomplete="current-password"`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("password form missing safe inputs or cache policy")
	}
	values := func(current, next, confirm, csrf string) url.Values {
		return url.Values{"csrf": {csrf}, "current_password": {current}, "new_password": {next}, "confirm_password": {confirm}}
	}
	if w := request("POST", values(oldPassword, newPassword, newPassword, ""), cookie); w.Code != 403 {
		t.Fatal("password change accepted without CSRF")
	}
	original, err := store.user(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ current, next, confirm string }{
		{oldPassword, newPassword, "different confirmation"},
		{"wrong-current-secret", newPassword, newPassword},
		{oldPassword, "short", "short"},
		{oldPassword, strings.Repeat("é", 37), strings.Repeat("é", 37)},
	} {
		w := request("POST", values(tc.current, tc.next, tc.confirm, sess.CSRF), cookie)
		if w.Code != 422 {
			t.Fatalf("invalid password change status: %d", w.Code)
		}
		if strings.Contains(w.Body.String(), `value="`+tc.current+`"`) || strings.Contains(w.Body.String(), `value="`+tc.next+`"`) {
			t.Fatal("password reflected in form")
		}
		u, err := store.user(ctx, "admin")
		if err != nil || u.Password != original.Password {
			t.Fatal("invalid form changed credentials")
		}
	}
	w := request("POST", values(oldPassword, newPassword, newPassword, sess.CSRF), cookie)
	if w.Code != 303 || w.Header().Get("Location") != "/login?notice=password-changed" {
		t.Fatalf("password change failed: %d %s", w.Code, w.Body.String())
	}
	u, err := store.user(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(newPassword)) != nil {
		t.Fatal("new password not stored")
	}
	if cost, err := bcrypt.Cost([]byte(u.Password)); err != nil || cost != 12 {
		t.Fatal("new password uses unexpected bcrypt cost")
	}
	for _, c := range []*http.Cookie{cookie, second} {
		r := httptest.NewRequest("GET", "/admin", nil)
		r.AddCookie(c)
		if _, ok := a.security.current(r); ok {
			t.Fatal("old session survived password change")
		}
	}
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(other)
	if _, ok := a.security.current(r); !ok {
		t.Fatal("another user's session was revoked")
	}
	if changed, err := store.replacePassword(ctx, original, "must-not-be-written"); err != nil || changed {
		t.Fatal("stale update overwrote new credentials")
	}
	// A login may have verified the old hash just before the password change.
	// Its final session issue must recheck credentials and reject that snapshot.
	staleRequest := httptest.NewRequest("POST", "/login", nil)
	staleResponse := httptest.NewRecorder()
	a.finishLogin(staleResponse, staleRequest, original, nil)
	if staleResponse.Code != 401 || len(staleResponse.Result().Cookies()) != 1 || staleResponse.Result().Cookies()[0].MaxAge > 1800 {
		t.Fatal("in-flight old-password login revived an authenticated session")
	}
	anon, anonymous := issue("")
	for _, tc := range []struct {
		password string
		status   int
	}{{oldPassword, 401}, {newPassword, 303}} {
		r := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"csrf": {anonymous.CSRF}, "username": {"admin"}, "password": {tc.password}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(anon)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("login after password change: %d; want %d", w.Code, tc.status)
		}
	}
}

func TestPasswordChangeAttemptsAreLimited(t *testing.T) {
	a := testApp(t, nil)
	w := httptest.NewRecorder()
	sess, ok := a.security.issue(w, httptest.NewRequest("GET", "/admin/password", nil), "admin")
	if !ok {
		t.Fatal("session issue failed")
	}
	cookie := w.Result().Cookies()[0]
	for attempt := 0; attempt < 6; attempt++ {
		r := httptest.NewRequest("POST", "/admin/password", strings.NewReader(url.Values{"csrf": {sess.CSRF}, "new_password": {"short"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		want := 422
		if attempt == 5 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d want %d", attempt+1, w.Code, want)
		}
	}
}
