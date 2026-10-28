package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginWhitelistParsingAndOverride(t *testing.T) {
	for _, tc := range []struct {
		value string
		bad   bool
		count int
	}{
		{"", false, 0}, {"198.51.100.10, 2001:db8::10", false, 2},
		{"198.51.100.10,::ffff:198.51.100.10", false, 1},
		{"example.com", true, 0}, {"2001:db8::/64", true, 0}, {"fe80::1%eth0", true, 0}, {"198.51.100.10,", true, 0},
	} {
		got, err := parseLoginRateLimitWhitelist(tc.value)
		if (err != nil) != tc.bad || err == nil && len(got) != tc.count {
			t.Errorf("whitelist %q: %v %v", tc.value, got, err)
		}
	}
	old := buildLoginRateLimitWhitelist
	buildLoginRateLimitWhitelist = "2001:db8::10"
	t.Cleanup(func() { buildLoginRateLimitWhitelist = old })
	t.Setenv("LOGIN_RATE_LIMIT_WHITELIST", "")
	got, err := configuredLoginRateLimitWhitelist()
	if err != nil || len(got) != 0 {
		t.Fatal("explicit empty env must disable the private build default")
	}
	if err := os.Unsetenv("LOGIN_RATE_LIMIT_WHITELIST"); err != nil {
		t.Fatal(err)
	}
	got, err = configuredLoginRateLimitWhitelist()
	if err != nil || len(got) != 1 {
		t.Fatal("private build default was not loaded when environment was absent")
	}
	t.Setenv("LOGIN_RATE_LIMIT_WHITELIST", "198.51.100.10")
	got, err = configuredLoginRateLimitWhitelist()
	if err != nil || len(got) != 1 {
		t.Fatal("environment override not loaded")
	}
}

func TestLoginBotDoesNotLockAccountFromAnotherIP(t *testing.T) {
	store := sqliteTestStore(t)
	if err := store.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("valid-test-passphrase"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO users(username,password) VALUES(?,?)", "admin", string(hash)); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, store)
	issued := httptest.NewRecorder()
	sess, ok := a.security.issue(issued, httptest.NewRequest("GET", "/login", nil), "")
	if !ok {
		t.Fatal("session issue failed")
	}
	cookie := issued.Result().Cookies()[0]
	request := func(remote, password string) int {
		r := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"csrf": {sess.CSRF}, "username": {"admin"}, "password": {password}}.Encode()))
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		return w.Code
	}
	for attempt := 0; attempt < 11; attempt++ {
		want := http.StatusUnauthorized
		if attempt == 10 {
			want = http.StatusTooManyRequests
		}
		if got := request("198.51.100.20:1234", "wrong-password"); got != want {
			t.Fatalf("bot attempt %d: %d want %d", attempt, got, want)
		}
	}
	if got := request("198.51.100.21:1234", "valid-test-passphrase"); got != http.StatusSeeOther {
		t.Fatalf("bot blocked legitimate account from another IP: %d", got)
	}
}

func TestLoginLimitIsPerIPAndCannotBeSpoofed(t *testing.T) {
	a := testApp(t, nil)
	var err error
	a.loginRateLimitWhitelist, err = parseLoginRateLimitWhitelist("2001:db8::10")
	if err != nil {
		t.Fatal(err)
	}
	a.proxies, err = parseProxies("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	request := func(remote, forwarded string, csrf bool) int {
		t.Helper()
		get := httptest.NewRequest("GET", "/login", nil)
		get.RemoteAddr = remote
		cookies := httptest.NewRecorder()
		sess, ok := a.security.issue(cookies, get, "")
		if !ok {
			t.Fatal("session issue failed")
		}
		values := url.Values{"username": {""}, "password": {"wrong"}}
		if csrf {
			values.Set("csrf", sess.CSRF)
		}
		r := httptest.NewRequest("POST", "/login", strings.NewReader(values.Encode()))
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("X-Forwarded-For", forwarded)
		r.AddCookie(cookies.Result().Cookies()[0])
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		return w.Code
	}
	for attempt := 0; attempt < 11; attempt++ {
		want := 401
		if attempt == 10 {
			want = 429
		}
		if got := request("198.51.100.20:1234", "2001:db8::10", true); got != want {
			t.Fatalf("spoofed whitelist attempt %d: %d want %d", attempt, got, want)
		}
	}
	if got := request("198.51.100.21:1234", "", true); got != 401 {
		t.Fatal("one IP's attempts blocked a different IP")
	}
	for attempt := 0; attempt < 11; attempt++ {
		if got := request("127.0.0.1:1234", "2001:db8::10", true); got != 401 {
			t.Fatalf("whitelisted proxy client was blocked: %d", got)
		}
	}
	if got := request("127.0.0.1:1234", "2001:db8::10", false); got != 403 {
		t.Fatal("whitelist bypassed CSRF")
	}
}
