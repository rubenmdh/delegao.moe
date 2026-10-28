package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGuestEmailMaskDoesNotEmbedAddress(t *testing.T) {
	for _, tc := range []struct{ email, mask string }{
		{"contact@example.com", "c••• [at] •••"}, {"", ""}, {"not-an-address", ""}, {"Name <a@example.com>", ""},
	} {
		if got := maskGuestEmail(tc.email); got != tc.mask {
			t.Errorf("mask %q = %q want %q", tc.email, got, tc.mask)
		}
	}
}

func TestSQLiteGuestEmailReveal(t *testing.T)      { testGuestEmailReveal(t, sqliteTestStore(t)) }
func TestIntegrationGuestEmailReveal(t *testing.T) { testGuestEmailReveal(t, integrationStore(t)) }

func testGuestEmailReveal(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO guestbook(id,name,email,comment,country)VALUES(7,?,?,?,?)", "Visitor", "contact@example.com", "Hello", "Spain"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO guestbook(id,name,email,comment,country)VALUES(8,?,?,?,?)", "No contact", "", "Hello", "Spain"); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, store)
	a.loginRateLimitWhitelist, _ = parseLoginRateLimitWhitelist("198.51.100.20")
	handler := a.handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/guestbook", nil))
	if page.Code != 200 || strings.Contains(page.Body.String(), "contact@example.com") || strings.Contains(page.Body.String(), "mailto:") || !strings.Contains(page.Body.String(), "c••• [at] •••") {
		t.Fatal("initial HTML exposes unmasked contact or lacks public mask")
	}
	cookie := page.Result().Cookies()[0]
	get := httptest.NewRequest("GET", "/guestbook", nil)
	get.AddCookie(cookie)
	sess, ok := a.security.current(get)
	if !ok {
		t.Fatal("missing public session")
	}
	request := func(method, path, csrf, origin, remote string, dish ...string) *httptest.ResponseRecorder {
		values := url.Values{"csrf": {csrf}}
		if len(dish) > 0 {
			values.Set("dishoftheday", dish[0])
		}
		r := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.RemoteAddr = remote
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct{ csrf, origin string }{{"", ""}, {"wrong", ""}, {sess.CSRF, "https://evil.example"}} {
		if w := request("POST", "/guestbook/7/email", tc.csrf, tc.origin, "198.51.100.20:1234"); w.Code != 403 {
			t.Fatal("unprotected email reveal accepted")
		}
	}
	if w := request("GET", "/guestbook/7/email", "", "", "198.51.100.20:1234"); w.Code == 200 || strings.Contains(w.Body.String(), "contact@example.com") {
		t.Fatal("contact exposed via GET")
	}
	if w := request("POST", "/guestbook/7/email", sess.CSRF, "", "198.51.100.20:1234"); w.Code != 403 || !strings.Contains(w.Body.String(), "dish_required") {
		t.Fatal("contact revealed without dish challenge")
	}
	if w := request("POST", "/guestbook/unlock-emails", "", "", "198.51.100.20:1234", "Ravioli"); w.Code != 403 {
		t.Fatal("dish accepted without CSRF")
	}
	if w := request("POST", "/guestbook/unlock-emails", sess.CSRF, "https://evil.example", "198.51.100.20:1234", "Ravioli"); w.Code != 403 {
		t.Fatal("cross-origin dish accepted")
	}
	if w := request("POST", "/guestbook/unlock-emails", sess.CSRF, "", "198.51.100.20:1234", "wrong"); w.Code != 422 {
		t.Fatal("wrong dish accepted")
	}
	if w := request("POST", "/guestbook/7/email", sess.CSRF, "", "198.51.100.20:1234"); w.Code != 403 {
		t.Fatal("wrong dish unlocked contact")
	}
	if w := request("POST", "/guestbook/unlock-emails", sess.CSRF, "", "198.51.100.20:1234", "Ravioli"); w.Code != 204 {
		t.Fatal("correct dish did not unlock session")
	}
	pageRequest := httptest.NewRequest("GET", "/guestbook?page=2", nil)
	pageRequest.AddCookie(cookie)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, pageRequest)
	if current, ok := a.security.current(pageRequest); !ok || !current.EmailsUnlocked {
		t.Fatal("unlock was not preserved across guestbook pages")
	}
	for _, id := range []string{"8", "999", "bad"} {
		if w := request("POST", "/guestbook/"+id+"/email", sess.CSRF, "", "198.51.100.21:1234"); w.Code != 404 {
			t.Fatalf("missing contact %s: %d", id, w.Code)
		}
	}
	for i := 0; i < 21; i++ {
		w := request("POST", "/guestbook/7/email", sess.CSRF, "", "198.51.100.20:1234")
		if i == 20 {
			if w.Code != 429 || w.Header().Get("Retry-After") == "" {
				t.Fatal("bulk contact requests were not limited")
			}
			continue
		}
		var data struct {
			Email string `json:"email"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &data)
		if w.Code != 200 || err != nil || data.Email != "contact@example.com" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("contact reveal failed or was cacheable")
		}
	}
	if w := request(http.MethodPost, "/guestbook/7/email", sess.CSRF, "", "198.51.100.22:1234"); w.Code != 200 {
		t.Fatal("one IP's reveal requests blocked another visitor")
	}
	other := httptest.NewRecorder()
	otherSession, ok := a.security.issue(other, httptest.NewRequest("GET", "/guestbook", nil), "")
	if !ok {
		t.Fatal("new session issue failed")
	}
	cookie = other.Result().Cookies()[0]
	if w := request("POST", "/guestbook/7/email", otherSession.CSRF, "", "198.51.100.22:1234"); w.Code != 403 {
		t.Fatal("unlock leaked to another session")
	}
}

func TestGuestEmailDishAnswersAreLimited(t *testing.T) {
	a := testApp(t, nil)
	handler := a.handler()
	issued := httptest.NewRecorder()
	sess, ok := a.security.issue(issued, httptest.NewRequest("GET", "/guestbook", nil), "")
	if !ok {
		t.Fatal("session issue failed")
	}
	for attempt := 0; attempt < 11; attempt++ {
		r := httptest.NewRequest("POST", "/guestbook/unlock-emails", strings.NewReader(url.Values{"csrf": {sess.CSRF}, "dishoftheday": {"wrong"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(issued.Result().Cookies()[0])
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		want := 422
		if attempt == 10 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("dish attempt %d: %d want %d", attempt, w.Code, want)
		}
	}
}
