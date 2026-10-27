package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func publisherFixture(t *testing.T, posts *[]Post) *Publisher {
	t.Helper()
	p, err := newPublisher(testApp(t, nil), t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	p.source = func(ctx context.Context, consume func(Post) error) error {
		for _, post := range *posts {
			if !post.Published {
				continue
			}
			if err := consume(post); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	return p
}

func readPublished(p *Publisher, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	return w
}

func TestPublisherBuildServeAndRestore(t *testing.T) {
	posts := []Post{{ID: 42, Title: "¡Hola, Rubén!", Body: "**Public** <script>bad()</script>", Published: true, Created: time.Now()}, {ID: 41, Title: "Secret draft", Body: "private", Published: false}}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := p.Status()
	if !status.Ready || status.Building || status.Workers != 2 || status.Pages != 11 || status.BuiltAt.IsZero() {
		t.Fatalf("unexpected status: %+v", status)
	}
	for _, path := range []string{"/", posts[0].URL(), "/posts", "/about-me", "/projects", "/music", "/games", "/contact", "/donate", "/keys"} {
		w := readPublished(p, "GET", path, nil)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "public, no-cache" || w.Header().Get("ETag") == "" {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Secret draft") || strings.Contains(w.Body.String(), `name="csrf"`) || strings.Contains(w.Body.String(), "<script>bad") {
			t.Fatalf("private or unsafe content generated on %s", path)
		}
		if !strings.Contains(w.Body.String(), "HTML generated in ") {
			t.Fatalf("rendering metadata missing from static page %s", path)
		}
	}
	for _, path := range []string{"/posts/41/secret-draft", "/admin", "/login", "/guestbook", "/current.json", "/manifest.json", "/posts/999/missing", "/?id="} {
		if w := readPublished(p, "GET", path, nil); w.Code != 404 {
			t.Fatalf("private/missing URL %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/posts/42", "/posts/42/old-title", "/?id=42"} {
		w := readPublished(p, "GET", path, nil)
		if w.Code != 301 || w.Header().Get("Location") != posts[0].URL() {
			t.Fatalf("canonical redirect %s: %d %s", path, w.Code, w.Header().Get("Location"))
		}
	}
	get := readPublished(p, "GET", posts[0].URL(), nil)
	conditional := readPublished(p, "GET", posts[0].URL(), map[string]string{"If-None-Match": get.Header().Get("ETag")})
	if conditional.Code != http.StatusNotModified || conditional.Body.Len() != 0 {
		t.Fatal("ETag did not avoid retransmitting the generated page")
	}
	if head := readPublished(p, "HEAD", posts[0].URL(), nil); head.Code != 200 || head.Body.Len() != 0 {
		t.Fatal("HEAD did not use the generated page")
	}
	if method := readPublished(p, "POST", "/", nil); method.Code != 405 {
		t.Fatal("public files accepted a write method")
	}
	// Existing public pages must remain readable without a source or SQL access.
	p.source = func(context.Context, func(Post) error) error { t.Fatal("public read accessed the source"); return nil }
	if w := readPublished(p, "GET", "/", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Public") || !strings.Contains(w.Body.String(), `href="`+posts[0].URL()+`"`) {
		t.Fatalf("static homepage: %d %s", w.Code, w.Body.String())
	}
	restored, err := newPublisher(p.app, p.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if w := readPublished(restored, "GET", posts[0].URL(), nil); w.Code != 200 || w.Body.String() != get.Body.String() {
		t.Fatal("restart did not restore disk-generated content")
	}
}

func TestPublisherRemovalRenameAndPagination(t *testing.T) {
	var posts []Post
	for i := range 22 {
		posts = append(posts, Post{ID: int64(22 - i), Title: fmt.Sprintf("Post %d", 22-i), Body: strings.Repeat("abc", 300), Published: true, Created: time.Now()})
	}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/posts", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "?page=2") {
		t.Fatal("first static archive page has no next link")
	}
	if w := readPublished(p, "GET", "/posts?page=2", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "?page=1") || strings.Contains(w.Body.String(), "?page=3") {
		t.Fatal("last static archive page has incorrect boundaries")
	}
	oldURL := posts[0].URL()
	if err := p.Mutate(context.Background(), func(context.Context) error { posts[0].Title = "A renamed title"; return nil }); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", oldURL, nil); w.Code != 301 || w.Header().Get("Location") != posts[0].URL() {
		t.Fatal("renamed slug was not canonicalized")
	}
	if err := p.Mutate(context.Background(), func(context.Context) error { posts = posts[20:]; return nil }); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", oldURL, nil); w.Code != 404 {
		t.Fatal("deleted post is still public")
	}
	if _, ok := p.CurrentURL(22); ok {
		t.Fatal("deleted post still has a canonical alias")
	}
	if w := readPublished(p, "GET", "/posts?page=2", nil); w.Code != 404 {
		t.Fatal("obsolete archive page is still public")
	}
	if err := p.Mutate(context.Background(), func(context.Context) error { posts[0].Published = false; posts[1].Published = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "No posts yet.") {
		t.Fatal("empty site was not generated")
	}
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if generationName.MatchString(entry.Name()) {
			count++
		}
		if strings.HasPrefix(entry.Name(), ".building-") {
			t.Fatal("temporary build directory leaked")
		}
	}
	if count != 2 {
		t.Fatalf("retained %d generations, expected current and previous", count)
	}
}

func TestPublisherFailureIsAtomicAndMutationFailsClosed(t *testing.T) {
	posts := []Post{{ID: 1, Title: "Public", Body: "old content", Published: true}}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous := p.active.Name
	goodSource := p.source
	p.source = func(context.Context, func(Post) error) error { return errors.New("database unavailable") }
	if err := p.Rebuild(context.Background()); err == nil {
		t.Fatal("failed source reported successful publication")
	}
	if !p.Status().Ready || p.active.Name != previous || readPublished(p, "GET", "/", nil).Code != 200 {
		t.Fatal("clean generation was replaced by an incomplete rebuild")
	}
	if err := p.Mutate(context.Background(), func(context.Context) error { posts[0].Published = false; return nil }); err == nil || !strings.Contains(err.Error(), "saved in the database") {
		t.Fatalf("missing actionable publication failure: %v", err)
	}
	if p.Status().Ready || p.Status().Error == "" || p.Status().Building {
		t.Fatalf("failed mutation status: %+v", p.Status())
	}
	if _, err := os.Stat(filepath.Join(p.dir, previous)); err != nil {
		t.Fatal("previous generation was not preserved for recovery")
	}
	w := readPublished(p, "GET", "/posts/1/public", nil)
	if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("failed unpublication served stale public content")
	}
	restored, err := newPublisher(p.app, p.dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status().Ready || readPublished(restored, "GET", "/", nil).Code != 503 {
		t.Fatal("restart forgot the durable dirty marker")
	}
	p.source = goodSource
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !p.Status().Ready || p.Status().Error != "" || readPublished(p, "GET", "/posts/1/public", nil).Code != 404 {
		t.Fatal("rebuild did not recover safely")
	}
}

func TestPublisherCancelledAndMissingCache(t *testing.T) {
	posts := []Post{{ID: 1, Title: "Public", Body: "content", Published: true}}
	p := publisherFixture(t, &posts)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Rebuild(ctx); !errors.Is(err, context.Canceled) || p.Status().Ready {
		t.Fatalf("cancelled rebuild: %v", err)
	}
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	file := p.active.Files["/"]
	if err := os.Remove(filepath.Join(p.dir, p.active.Name, file.Name)); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/", nil); w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing disk cache did not fail closed")
	}
	restored, err := newPublisher(p.app, p.dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status().Ready || restored.Status().Error == "" || readPublished(restored, "GET", "/", nil).Code != 503 {
		t.Fatal("restore accepted a missing generated file")
	}
	restored.source = p.source
	if err := restored.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !restored.Status().Ready {
		t.Fatal("damaged cache could not be repaired")
	}
}

func TestPublisherRejectsPrivateManifest(t *testing.T) {
	posts := []Post{}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	p.active.Files["/admin"] = p.active.Files["/"]
	data, err := json.Marshal(p.active)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.writeCurrent(data); err != nil {
		t.Fatal(err)
	}
	restored, err := newPublisher(p.app, p.dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status().Ready || readPublished(restored, "GET", "/admin", nil).Code != 503 {
		t.Fatal("restore accepted a private route in the public manifest")
	}
	if _, err := newPublisher(p.app, t.TempDir(), 8); err == nil {
		t.Fatal("unbounded build worker configuration accepted")
	}
}

func TestPublisherPointerFailureAndPendingReads(t *testing.T) {
	posts := []Post{{ID: 1, Title: "Public", Body: "content", Published: true}}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := p.active.Name
	if err := os.Remove(filepath.Join(p.dir, "current.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(p.dir, "current.json"), 0750); err != nil {
		t.Fatal(err)
	}
	err := p.Mutate(context.Background(), func(context.Context) error {
		if w := readPublished(p, "GET", "/", nil); w.Code != 503 {
			t.Fatal("pending mutation still serves public HTML")
		}
		posts = nil
		return nil
	})
	if err == nil || p.Status().Ready || p.active.Name != old {
		t.Fatalf("pointer failure activated incomplete publication: %v %+v", err, p.Status())
	}
	if err := os.Remove(filepath.Join(p.dir, "current.json")); err != nil {
		t.Fatal(err)
	}
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/posts/1/public", nil); w.Code != 404 {
		t.Fatal("repair republished deleted post")
	}
}

func TestPublisherStartupBlock(t *testing.T) {
	posts := []Post{{ID: 1, Title: "Old post", Body: "content", Published: true}}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	p.Block(errors.New("startup reconciliation failed"))
	if p.Status().Ready || p.Status().Error != "startup reconciliation failed" || readPublished(p, "GET", "/", nil).Code != 503 {
		t.Fatal("startup failure served stale restored content")
	}
	restored, err := newPublisher(p.app, p.dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status().Ready || readPublished(restored, "GET", "/", nil).Code != 503 {
		t.Fatal("startup block was not persisted")
	}
}

func TestIntegrationPublisherStreamsOneSnapshot(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, post := range []Post{{Title: "First", Body: "first body", Author: "admin", Published: true}, {Title: "Second", Body: "second body", Author: "admin", Published: true}, {Title: "Draft", Body: "secret", Author: "admin"}} {
		if err := store.savePost(ctx, post); err != nil {
			t.Fatal(err)
		}
	}
	var got []Post
	if err := store.publicationPosts(ctx, func(post Post) error {
		got = append(got, post)
		if len(got) == 1 {
			_, err := store.db.ExecContext(ctx, "UPDATE news SET body='changed concurrently' WHERE id=1")
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "Second" || got[1].Body != "first body" {
		t.Fatalf("source did not use one published snapshot: %+v", got)
	}
	p, err := newPublisher(testApp(t, store), t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/posts/3/draft", nil); w.Code != 404 {
		t.Fatal("SQL draft leaked into static generation")
	}
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	if w := readPublished(p, "GET", "/", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Second") {
		t.Fatal("static reads need the database")
	}
}
