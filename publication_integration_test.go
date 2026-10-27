package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIntegrationMixedPublicationHTTP(t *testing.T) {
	testMixedPublicationHTTP(t, integrationStore(t))
}

func testMixedPublicationHTTP(t *testing.T, store *Store) {
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, store)
	p, err := newPublisher(a, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	a.publisher = p
	if err := p.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	issued := httptest.NewRecorder()
	sess, ok := a.security.issue(issued, httptest.NewRequest("GET", "/admin", nil), "admin")
	if !ok {
		t.Fatal("session")
	}
	cookie := issued.Result().Cookies()[0]
	handler := a.handler()
	request := func(method, path string, values url.Values, authenticated bool) *httptest.ResponseRecorder {
		t.Helper()
		body := ""
		if values != nil {
			body = values.Encode()
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if values != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if authenticated {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	public := request("GET", "/", nil, false)
	if public.Code != 200 || public.Header().Get("ETag") == "" || public.Header().Get("Set-Cookie") != "" || public.Header().Get("Cache-Control") != "public, no-cache" {
		t.Fatalf("public response: %d %v", public.Code, public.Header())
	}
	values := url.Values{"csrf": {sess.CSRF}, "title": {"Published from the panel"}, "body": {"**Generated content**"}, "status": {"1"}, "postedby": {"forged"}}
	if w := request("POST", route("admin.posts"), values, true); w.Code != 303 {
		t.Fatalf("publish: %d %s", w.Code, w.Body.String())
	}
	posts, err := store.posts(ctx, true, 1)
	if err != nil || len(posts) != 1 {
		t.Fatalf("post create: %+v %v", posts, err)
	}
	post := posts[0]
	if post.Author != "admin" {
		t.Fatal("forged author")
	}
	w := request("GET", post.URL(), nil, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<strong>Generated content</strong>") {
		t.Fatalf("generated page missing: %d %s", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	values.Set("title", "Renamed on the web")
	values.Set("body", "**Updated generated content**")
	if w := request("POST", route("admin.posts.update", post.ID), values, true); w.Code != 303 {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	renamed, err := store.post(ctx, post.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if w := request("GET", post.URL(), nil, false); w.Code != 301 || w.Header().Get("Location") != renamed.URL() {
		t.Fatal("old slug did not redirect")
	}
	if w := request("GET", renamed.URL(), nil, false); w.Code != 200 || w.Header().Get("ETag") == etag || !strings.Contains(w.Body.String(), "Updated generated content") {
		t.Fatal("modified page was not regenerated")
	}
	before := p.Status().BuiltAt
	guest := url.Values{"csrf": {sess.CSRF}, "name": {"Visitor"}, "comment": {"Hello"}, "country": {"Spain"}, "dishoftheday": {"Ravioli"}}
	if w := request("POST", route("guestbook"), guest, true); w.Code != 303 {
		t.Fatalf("guestbook: %d %s", w.Code, w.Body.String())
	}
	if !p.Status().BuiltAt.Equal(before) {
		t.Fatal("dynamic guestbook unnecessarily regenerated static pages")
	}
	if w := request("GET", route("guestbook"), nil, false); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), "Visitor") {
		t.Fatal("guestbook was not dynamic")
	}
	// A failed regeneration after unpublishing must never leak the old page.
	p.source = func(context.Context, func(Post) error) error { return errors.New("simulated publication failure") }
	values.Set("status", "0")
	if w := request("POST", route("admin.posts.update", post.ID), values, true); w.Code != 503 {
		t.Fatalf("failed generation: %d %s", w.Code, w.Body.String())
	}
	saved, err := store.post(ctx, post.ID, true)
	if err != nil || saved.Published {
		t.Fatalf("unpublish did not save: %+v %v", saved, err)
	}
	for _, path := range []string{"/", post.URL(), renamed.URL(), "/about-me"} {
		if w := request("GET", path, nil, false); w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("stale public content served at %s: %d", path, w.Code)
		}
	}
	if w := request("GET", route("admin"), nil, true); w.Code != 200 {
		t.Fatalf("repair administration unavailable: %d %s", w.Code, w.Body.String())
	}
	p.source = store.publicationPosts
	if w := request("POST", route("admin.rebuild"), url.Values{"csrf": {sess.CSRF}}, true); w.Code != 303 {
		t.Fatalf("repair: %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", renamed.URL(), nil, false); w.Code != 404 {
		t.Fatal("draft reappeared after repair")
	}
	values.Set("status", "1")
	if w := request("POST", route("admin.posts.update", post.ID), values, true); w.Code != 303 {
		t.Fatal("republish failed")
	}
	if w := request("POST", route("admin.posts.delete", post.ID), url.Values{"csrf": {sess.CSRF}}, true); w.Code != 303 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", renamed.URL(), nil, false); w.Code != 404 {
		t.Fatal("deleted page stayed public")
	}
	// Loss of MySQL must not affect static GET/HEAD requests or canonical aliases.
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/posts", "/about-me", "/contact"} {
		if w := request("GET", path, nil, false); w.Code != http.StatusOK {
			t.Fatalf("static page needs SQL: %s %d", path, w.Code)
		}
	}
	if w := request("GET", route("guestbook"), nil, false); w.Code != 500 {
		t.Fatal("guestbook unexpectedly stopped using MySQL")
	}
}
