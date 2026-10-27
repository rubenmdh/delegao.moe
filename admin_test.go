package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminSearchPaginationKeepsLiteralQuery(t *testing.T) {
	query := "100%_ & 東京"
	path := adminPaginationURL(query, "draft", 2)
	u, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/admin/posts" || u.Query().Get("q") != query || u.Query().Get("status") != "draft" || u.Query().Get("page") != "2" {
		t.Fatalf("search context was lost: %s", path)
	}
	r := httptest.NewRequest(http.MethodGet, "/admin/posts?q="+url.QueryEscape(strings.Repeat("é", 101))+"&status=invalid", nil)
	q, status := adminFilters(r)
	if len([]rune(q)) != 100 || status != "all" {
		t.Fatalf("invalid filter was not normalized: %q %q", q, status)
	}
}

func TestAdminPreviewRequiresSessionAndCSRF(t *testing.T) {
	a := testApp(t, nil)
	handler := a.handler()
	body := "**Hello**\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert%281%29)"
	values := url.Values{"body": {body}}
	r := httptest.NewRequest(http.MethodPost, "/admin/posts/preview", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("anonymous preview: %d", w.Code)
	}
	get := httptest.NewRequest(http.MethodGet, "/admin", nil)
	cookies := httptest.NewRecorder()
	sess, ok := a.security.issue(cookies, get, "admin")
	if !ok {
		t.Fatal("could not create test session")
	}
	values.Set("csrf", sess.CSRF)
	r = httptest.NewRequest(http.MethodPost, "/admin/posts/preview", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookies.Result().Cookies()[0])
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	result := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(result, "<strong>Hello</strong>") || strings.Contains(result, "<script>") || strings.Contains(result, `href="javascript:`) {
		t.Fatalf("unsafe or invalid preview: %d %s", w.Code, result)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("editor preview must not be cached")
	}
}

func TestIntegrationAdminDashboardAndFilters(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for i := range 22 {
		if err := store.savePost(ctx, Post{Title: fmt.Sprintf("100%%_ draft %d", i), Body: "private draft", Tags: "test", Author: "admin"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.savePost(ctx, Post{Title: "100%_ published", Body: "public", Author: "admin", Published: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.savePost(ctx, Post{Title: "Unrelated published", Body: "public", Author: "admin", Published: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.addGuest(ctx, Guest{Name: "Guest", Comment: "Hello", Country: "Spain"}); err != nil {
		t.Fatal(err)
	}
	stats, err := store.adminStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Published != 2 || stats.Drafts != 22 || stats.Guests != 1 || len(stats.Recent) != 5 {
		t.Fatalf("dashboard counts do not match MySQL: %+v", stats)
	}
	a := testApp(t, store)
	handler := a.handler()
	issue := httptest.NewRecorder()
	if _, ok := a.security.issue(issue, httptest.NewRequest(http.MethodGet, "/admin", nil), "admin"); !ok {
		t.Fatal("could not create session")
	}
	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(issue.Result().Cookies()[0])
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	result := get("/admin/posts?q=100%25_&status=draft").Body.String()
	if strings.Count(result, `class="admin-post-title"`) != 20 || strings.Contains(result, "100%_ published") || !strings.Contains(result, "page=2") || !strings.Contains(result, "status=draft") || !strings.Contains(result, "q=100%25_") {
		t.Fatalf("filter or pagination failed: %s", result)
	}
	result = get("/admin/posts?q=100%25_&status=published").Body.String()
	if strings.Count(result, `class="admin-post-title"`) != 1 || !strings.Contains(result, "100%_ published") || strings.Contains(result, "Unrelated published") {
		t.Fatalf("literal wildcard search failed: %s", result)
	}
	result = get("/admin/posts?q=" + url.QueryEscape("' OR 1=1 --")).Body.String()
	if !strings.Contains(result, "No matching posts.") {
		t.Fatal("search input was not treated as literal data")
	}
	result = get("/admin").Body.String()
	if !strings.Contains(result, "Static publication") || !strings.Contains(result, "Recent posts") || !strings.Contains(result, "Unrelated published") {
		t.Fatal("dashboard details are missing")
	}
}
