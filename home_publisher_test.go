package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Observe the publication data independently of visual markup, which can change
// without weakening the homepage's bounded-data and snapshot guarantees.
func observeHome(t *testing.T, a *App) <-chan View {
	t.Helper()
	homes := make(chan View, 1)
	a.templates = template.Must(template.New("layout").Funcs(template.FuncMap{
		"observe": func(v View) string {
			if v.Home {
				homes <- v
			}
			return "published"
		},
	}).Parse(`{{observe .}}`))
	return homes
}

func takeHome(t *testing.T, homes <-chan View) View {
	t.Helper()
	select {
	case home := <-homes:
		return home
	default:
		t.Fatal("homepage was not generated")
		return View{}
	}
}

func seedHomePosts(t *testing.T, store *Store, count int) []Post {
	t.Helper()
	if err := store.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	posts := make([]Post, 0, count)
	for i := 1; i <= count; i++ {
		p := Post{ID: int64(i), Title: fmt.Sprintf("Entry %d", i), Body: fmt.Sprintf("Body %d ", i) + strings.Repeat("ñ😀", 1600), Author: "Rubén", Published: true, Created: time.Date(2024, 1, i, 8, 30, 0, 0, time.UTC)}
		if _, err := store.db.Exec("INSERT INTO news(id,title,body,status,tags,posted_by,created_at) VALUES(?,?,?,1,'',?,?)", p.ID, p.Title, p.Body, p.Author, p.Created.Format("2006-01-02 15:04:05")); err != nil {
			t.Fatal(err)
		}
		posts = append(posts, p)
	}
	if _, err := store.db.Exec("INSERT INTO news(id,title,body,status,tags,posted_by) VALUES(100,'Unpublished draft','draft secret',0,'','admin')"); err != nil {
		t.Fatal(err)
	}
	return posts
}

func checkHomePosts(t *testing.T, home View, posts []Post) {
	t.Helper()
	if !home.Home || !home.Single || home.Page != "posts" {
		t.Fatal("homepage flags are missing")
	}
	if len(posts) == 0 {
		if len(home.Posts) != 0 || len(home.Earlier) != 0 {
			t.Fatal("empty publication has homepage posts")
		}
		return
	}
	latest := posts[len(posts)-1]
	if len(home.Posts) != 1 || home.Posts[0].ID != latest.ID || home.Posts[0].Title != latest.Title || home.Posts[0].Author != latest.Author || !home.Posts[0].Created.Equal(latest.Created) {
		t.Fatalf("wrong latest post: %+v", home.Posts)
	}
	if home.Posts[0].Body != publicationPrefix(latest.Body, homeExcerptRunes) || utf8.RuneCountInString(home.Posts[0].Body) > homeExcerptRunes {
		t.Fatal("homepage retained a full post body or a different excerpt")
	}
	wantEarlier := min(3, len(posts)-1)
	if len(home.Earlier) != wantEarlier {
		t.Fatalf("got %d earlier posts, want %d", len(home.Earlier), wantEarlier)
	}
	for i, earlier := range home.Earlier {
		want := posts[len(posts)-2-i]
		if earlier.ID != want.ID || earlier.Title != want.Title || earlier.Author != want.Author || !earlier.Created.Equal(want.Created) || earlier.URL() != want.URL() || earlier.Body != "" {
			t.Fatalf("earlier post is not canonical metadata: %+v", earlier)
		}
	}
}

func TestHomePublicationAndDynamicFallbackHaveLatestFour(t *testing.T) {
	for _, count := range []int{0, 1, 2, 4, 5} {
		t.Run(fmt.Sprintf("published=%d", count), func(t *testing.T) {
			store := sqliteTestStore(t)
			posts := seedHomePosts(t, store, count)
			a := testApp(t, store)
			homes := observeHome(t, a)
			publisher, err := newPublisher(a, t.TempDir(), 2)
			if err != nil {
				t.Fatal(err)
			}
			if err := publisher.Rebuild(context.Background()); err != nil {
				t.Fatal(err)
			}
			published := takeHome(t, homes)
			checkHomePosts(t, published, posts)
			if _, ok := publisher.CurrentURL(100); ok {
				t.Fatal("unpublished draft has a public URL")
			}
			response := httptest.NewRecorder()
			a.home(response, httptest.NewRequest("GET", "/", nil))
			if response.Code != 200 {
				t.Fatal("dynamic homepage failed")
			}
			dynamic := takeHome(t, homes)
			checkHomePosts(t, dynamic, posts)
			if !reflect.DeepEqual(published.Posts, dynamic.Posts) {
				t.Fatal("static and dynamic latest posts differ")
			}
			for i := range published.Earlier {
				if !reflect.DeepEqual(published.Earlier[i], dynamic.Earlier[i]) {
					t.Fatal("static and dynamic earlier-post metadata differ")
				}
			}
			if count > 0 {
				full, err := store.post(context.Background(), posts[count-1].ID, false)
				if err != nil || full.Body != posts[count-1].Body {
					t.Fatal("homepage truncation modified stored article content")
				}
			}
			// Public reads remain disk-only after the source becomes unavailable.
			if err := store.db.Close(); err != nil {
				t.Fatal(err)
			}
			publisher.source = func(context.Context, func(Post) error) error {
				t.Fatal("static homepage queried its source")
				return nil
			}
			if response := readPublished(publisher, "GET", "/", nil); response.Code != 200 {
				t.Fatal("cached homepage is not available without its database")
			}
		})
	}
}

func TestHomePublicationKeepsOneSnapshotDuringConcurrentEdits(t *testing.T) {
	store := sqliteTestStore(t)
	posts := seedHomePosts(t, store, 5)
	a := testApp(t, store)
	homes := observeHome(t, a)
	publisher, err := newPublisher(a, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	publisher.source = func(ctx context.Context, consume func(Post) error) error {
		return store.publicationPosts(ctx, func(post Post) error {
			if err := consume(post); err != nil {
				return err
			}
			if !changed {
				changed = true
				if _, err := store.db.ExecContext(ctx, "UPDATE news SET title='Changed during build' WHERE id=4"); err != nil {
					return err
				}
				_, err := store.db.ExecContext(ctx, "INSERT INTO news(id,title,body,status,tags,posted_by) VALUES(6,'New during build','new body',1,'','admin')")
				return err
			}
			return nil
		})
	}
	if err := publisher.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkHomePosts(t, takeHome(t, homes), posts)
	if _, ok := publisher.CurrentURL(6); ok {
		t.Fatal("new post leaked into an already-open snapshot")
	}
	publisher.source = store.publicationPosts
	if err := publisher.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated := takeHome(t, homes)
	if updated.Posts[0].ID != 6 || len(updated.Earlier) != 3 || updated.Earlier[1].Title != "Changed during build" {
		t.Fatal("next publication did not pick up the committed edits")
	}
}

func TestIntegrationHomePostsBounded(t *testing.T) {
	store := integrationStore(t)
	posts := seedHomePosts(t, store, 5)
	read, err := store.homePosts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 4 {
		t.Fatalf("got %d homepage records, want four", len(read))
	}
	checkHomePosts(t, View{Page: "posts", Single: true, Home: true, Posts: read[:1], Earlier: read[1:]}, posts)
}
