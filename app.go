package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/mail"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/crypto/bcrypt"
)

//go:embed web/templates/*.html web/templates/pages/*.html web/static
var web embed.FS

type App struct {
	publisher *Publisher
	proxies   []netip.Prefix
	store     *Store
	security  *security
	templates *template.Template
	markdown  goldmark.Markdown
	answer    string
	dummyHash []byte
}
type View struct {
	Dashboard                              AdminDashboard
	Publication                            AdminPublication
	Query, Filter, Notice                  string
	PreviousURL, NextURL                   string
	Title, Page, CSRF, User, Error, Action string
	Posts                                  []Post
	Post                                   Post
	Guests                                 []Guest
	Guest                                  Guest
	PageNumber, Previous, Next             int
	Admin, Wide, Single, Success, Home     bool
}

func newApp(store *Store, secure bool, answer string) (*App, error) {
	a := &App{store: store, security: newSecurity(secure), answer: answer, markdown: goldmark.New(goldmark.WithExtensions(extension.GFM))}
	funcs := template.FuncMap{"route": route, "markdown": a.renderMarkdown, "date": func(t time.Time) string { return t.Format("2006-01-02 15:04") }, "excerpt": func(s string) string {
		r := []rune(s)
		if len(r) > 250 {
			return string(r[:250]) + "…"
		}
		return s
	}}
	t, err := template.New("site").Funcs(funcs).ParseFS(web, "web/templates/*.html", "web/templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	a.templates = t
	a.dummyHash, err = bcrypt.GenerateFromPassword([]byte(token()), 12)
	return a, err
}
func (a *App) renderMarkdown(source string) template.HTML {
	var b bytes.Buffer
	// Goldmark's safe defaults omit raw HTML and unsafe URL schemes.
	if err := a.markdown.Convert([]byte(source), &b); err != nil {
		return ""
	}
	return template.HTML(b.String())
}
func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	bind := func(method, name string, h http.HandlerFunc) { mux.HandleFunc(method+" "+routes[name], h) }
	bind("GET", "home", a.home)
	bind("GET", "posts.index", a.postList)
	bind("GET", "posts.show", a.postShow)
	for _, p := range []string{"about-me", "projects", "music", "games", "contact", "donate", "keys"} {
		mux.HandleFunc("GET /"+p, func(w http.ResponseWriter, r *http.Request) {
			if a.publisher != nil {
				a.publisher.ServeHTTP(w, r)
				return
			}
			a.render(w, r, 200, View{Page: p, Title: strings.ToUpper(p[:1]) + p[1:], Wide: p == "donate" || p == "keys"})
		})
	}
	bind("GET", "guestbook", a.guestList)
	bind("POST", "guestbook", a.addGuest)
	bind("GET", "login", a.loginForm)
	bind("POST", "login", a.login)
	bind("POST", "logout", a.logout)
	bind("GET", "admin", a.auth(a.adminDashboard))
	bind("GET", "admin.posts", a.auth(a.adminPosts))
	bind("POST", "admin.preview", a.auth(a.adminPreview))
	bind("POST", "admin.rebuild", a.auth(a.rebuildPublic))
	bind("GET", "admin.posts.create", a.auth(a.postForm))
	bind("POST", "admin.posts", a.auth(a.savePost))
	bind("GET", "admin.posts.edit", a.auth(a.postForm))
	bind("POST", "admin.posts.update", a.auth(a.savePost))
	bind("POST", "admin.posts.delete", a.auth(a.deletePost))
	bind("GET", "admin.guests", a.auth(a.guestList))
	bind("POST", "admin.guests.delete", a.auth(a.deleteGuest))
	static, _ := fs.Sub(web, "web/static")
	files := http.FileServer(http.FS(static))
	for _, prefix := range []string{"css", "img", "webfonts", "js", "assets"} {
		mux.Handle("GET /"+prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(r.URL.Path, "/")
			info, err := fs.Stat(static, name)
			if err != nil || info.IsDir() {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=86400")
			files.ServeHTTP(w, r)
		}))
	}
	a.legacy(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = a.clientAddress(r)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Invalid or oversized form.", 400)
				return
			}
			if !a.security.csrf(w, r) {
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *App) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := a.security.current(r)
		if !ok || s.User == "" {
			http.Redirect(w, r, route("login"), http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
func (a *App) render(w http.ResponseWriter, r *http.Request, status int, v View) {
	if a.publisher != nil {
		p := a.publisher.Status()
		v.Publication = AdminPublication{Ready: p.Ready, Building: p.Building, BuiltAt: p.BuiltAt, Pages: p.Pages, Workers: p.Workers, Error: p.Error}
	}
	if v.Notice == "" {
		v.Notice = map[string]string{"saved": "Post saved and publication updated.", "deleted": "Post deleted and publication updated.", "rebuilt": "Public pages rebuilt successfully.", "guest-deleted": "Guestbook entry deleted."}[r.URL.Query().Get("notice")]
	}
	v.Admin = strings.HasPrefix(r.URL.Path, "/admin")
	v.Wide = v.Wide || v.Admin || v.Page == "login" || v.Page == "guestbook"
	s, ok := a.security.current(r)
	if v.Page == "login" || v.Page == "guestbook" || v.Admin {
		if !ok {
			s, ok = a.security.issue(w, r, "")
			if !ok {
				return
			}
		}
		v.CSRF = s.CSRF
	}
	v.User = s.User
	var b bytes.Buffer
	if err := a.templates.ExecuteTemplate(&b, "layout", v); err != nil {
		a.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func (a *App) fail(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, "Something went wrong. Please try again later.", 500)
}
func page(r *http.Request) int {
	p, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || p < 1 || p > 100000 {
		return 1
	}
	return p
}
func id(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n < 1 {
		return 0, errors.New("invalid id")
	}
	return n, nil
}
func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if a.publisher != nil {
		a.publisher.ServeHTTP(w, r)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.URL.Query().Has("id") {
		a.redirectPost(w, r, r.URL.Query().Get("id"))
		return
	}
	post, err := a.store.latestPost(r.Context())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, err)
		return
	}
	v := View{Page: "posts", Title: "Delegao.moe", Single: true, Home: true}
	if err == nil {
		v.Posts = []Post{post}
	}
	a.render(w, r, 200, v)
}
func (a *App) postList(w http.ResponseWriter, r *http.Request) {
	if a.publisher != nil && !strings.HasPrefix(r.URL.Path, "/admin") {
		a.publisher.ServeHTTP(w, r)
		return
	}
	admin := strings.HasPrefix(r.URL.Path, "/admin")
	p := page(r)
	posts, err := a.store.posts(r.Context(), admin, p)
	if err != nil {
		a.fail(w, err)
		return
	}
	v := View{Page: "posts", Title: "News", Posts: posts, PageNumber: p}
	if p > 1 {
		v.Previous = p - 1
	}
	if len(posts) > 20 {
		v.Next = p + 1
		v.Posts = posts[:20]
	}
	a.render(w, r, 200, v)
}
func (a *App) postShow(w http.ResponseWriter, r *http.Request) {
	if a.publisher != nil {
		a.publisher.ServeHTTP(w, r)
		return
	}
	n, err := id(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := a.store.post(r.Context(), n, false)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	if r.URL.Path != p.URL() {
		http.Redirect(w, r, p.URL(), 301)
		return
	}
	a.render(w, r, 200, View{Title: p.Title, Page: "posts", Posts: []Post{p}, Single: true})
}
func (a *App) guestList(w http.ResponseWriter, r *http.Request) {
	p := page(r)
	guests, err := a.store.guests(r.Context(), p)
	if err != nil {
		a.fail(w, err)
		return
	}
	v := View{Page: "guestbook", Title: "Guestbook", Guests: guests, Success: r.URL.Query().Get("signed") == "1", PageNumber: p}
	if p > 1 {
		v.Previous = p - 1
	}
	if len(guests) > 20 {
		v.Next = p + 1
		v.Guests = guests[:20]
	}
	a.render(w, r, 200, v)
}
func field(r *http.Request, key string) string { return strings.TrimSpace(r.PostFormValue(key)) }
func valid(s string, min, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) >= min && utf8.RuneCountInString(s) <= max
}
func (a *App) addGuest(w http.ResponseWriter, r *http.Request) {
	if !a.security.allow(r, "guestbook", 5) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many submissions. Please try again later.", 429)
		return
	}
	g := Guest{Name: field(r, "name"), Email: field(r, "email"), Comment: field(r, "comment"), Country: field(r, "country")}
	message := ""
	if !valid(g.Name, 1, 100) || !valid(g.Comment, 1, 512) || !valid(g.Country, 1, 100) || !valid(g.Email, 0, 128) {
		message = "Please provide a name, a comment (up to 512 characters) and a country."
	}
	if g.Email != "" {
		address, err := mail.ParseAddress(g.Email)
		if err != nil || address.Address != g.Email {
			message = "Please enter a valid email address."
		}
	}
	if field(r, "website") != "" || field(r, "dishoftheday") != a.answer {
		message = "Please check the dish of the day on the About me page."
	}
	if message != "" {
		a.render(w, r, 422, View{Page: "guestbook", Title: "Guestbook", Error: message, Guest: g})
		return
	}
	if err := a.store.addGuest(r.Context(), g); err != nil {
		a.fail(w, err)
		return
	}
	http.Redirect(w, r, route("guestbook")+"?signed=1", 303)
}
func (a *App) loginForm(w http.ResponseWriter, r *http.Request) {
	if s, ok := a.security.current(r); ok && s.User != "" {
		http.Redirect(w, r, route("admin"), 303)
		return
	}
	a.render(w, r, 200, View{Page: "login", Title: "Log in"})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !a.security.allow(r, "login", 10) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many login attempts. Please try again later.", 429)
		return
	}
	username, password := field(r, "username"), r.PostFormValue("password")
	if !valid(username, 1, 50) || len(password) > 72 {
		a.render(w, r, 401, View{Page: "login", Title: "Log in", Error: "Invalid username or password."})
		return
	}
	u, err := a.store.user(r.Context(), username)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, err)
		return
	}
	hash := a.dummyHash
	if err == nil {
		hash = []byte(strings.Replace(u.Password, "$2y$", "$2a$", 1))
	}
	check := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil || check != nil {
		a.render(w, r, 401, View{Page: "login", Title: "Log in", Error: "Invalid username or password."})
		return
	}
	if cost, err := bcrypt.Cost(hash); err == nil && cost < 12 {
		upgraded, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			a.fail(w, err)
			return
		}
		if _, err := a.store.db.ExecContext(r.Context(), "UPDATE users SET password=? WHERE id=? AND password=?", string(upgraded), u.ID, u.Password); err != nil {
			a.fail(w, err)
			return
		}
	}
	if _, ok := a.security.issue(w, r, u.Username); !ok {
		return
	}
	http.Redirect(w, r, route("admin"), 303)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	a.security.logout(w, r)
	http.Redirect(w, r, route("home"), 303)
}
func (a *App) postForm(w http.ResponseWriter, r *http.Request) {
	p := Post{}
	if r.PathValue("id") != "" {
		n, err := id(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		p, err = a.store.post(r.Context(), n, true)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			a.fail(w, err)
			return
		}
	}
	a.renderPostForm(w, r, 200, p, "")
}
func (a *App) renderPostForm(w http.ResponseWriter, r *http.Request, status int, p Post, message string) {
	action := route("admin.posts")
	if p.ID != 0 {
		action = route("admin.posts.update", p.ID)
	}
	a.render(w, r, status, View{Page: "editor", Title: "Edit post", Action: action, Post: p, Error: message})
}
func (a *App) savePost(w http.ResponseWriter, r *http.Request) {
	s, _ := a.security.current(r)
	p := Post{Title: field(r, "title"), Body: r.PostFormValue("body"), Tags: field(r, "tags"), Author: s.User, Published: field(r, "status") == "1"}
	if r.PathValue("id") != "" {
		n, err := id(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		original, err := a.store.post(r.Context(), n, true)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			a.fail(w, err)
			return
		}
		p.ID = n
		p.Author = original.Author
		p.Created = original.Created
		p.Updated = original.Updated
	}
	if !valid(p.Title, 1, 255) || !valid(p.Body, 1, 500000) || !valid(p.Tags, 0, 1000) || (field(r, "status") != "0" && field(r, "status") != "1") {
		a.renderPostForm(w, r, 422, p, "Please check the title, body, tags and publication status.")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	if err := a.mutatePublic(ctx, func(ctx context.Context) error {
		if p.ID != 0 {
			if _, err := a.store.post(ctx, p.ID, true); err != nil {
				return err
			}
		}
		savedID, err := a.store.savePostRecord(ctx, p)
		if err != nil {
			return err
		}
		p.ID = savedID
		p, err = a.store.post(ctx, savedID, true)
		return err
	}); err != nil {
		slog.Error("save/publication failed", "error", err)
		a.renderPostForm(w, r, 503, p, "The operation did not complete. Check the saved post in administration before trying again. If it is saved in the database, rebuild the public pages from the dashboard; public pages stay unavailable until publication succeeds.")
		return
	}
	http.Redirect(w, r, route("admin.posts")+"?notice=saved", 303)
}
func (a *App) deletePost(w http.ResponseWriter, r *http.Request) {
	a.remove(w, r, "news", "admin.posts")
}
func (a *App) deleteGuest(w http.ResponseWriter, r *http.Request) {
	a.remove(w, r, "guestbook", "admin.guests")
}
func (a *App) remove(w http.ResponseWriter, r *http.Request, table, target string) {
	n, err := id(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	if table == "news" {
		err = a.mutatePublic(ctx, func(ctx context.Context) error { return a.store.delete(ctx, table, n) })
	} else {
		err = a.store.delete(ctx, table, n)
	}
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("deletion/publication failed", "error", err)
		a.render(w, r, 503, View{Page: "dashboard", Title: "Publication needs attention", Error: "The operation did not complete. Check the current records before retrying. If the database changed, rebuild public pages from the dashboard to restore publication."})
		return
	}
	notice := "deleted"
	if table == "guestbook" {
		notice = "guest-deleted"
	}
	http.Redirect(w, r, route(target)+"?notice="+notice, 303)
}
func (a *App) redirectPost(w http.ResponseWriter, r *http.Request, value string) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 1 {
		http.NotFound(w, r)
		return
	}
	if a.publisher != nil {
		if !a.publisher.Status().Ready {
			http.Error(w, "Publication is temporarily unavailable.", 503)
			return
		}
		target, ok := a.publisher.CurrentURL(n)
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, target, 301)
		return
	}
	p, err := a.store.post(r.Context(), n, false)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	http.Redirect(w, r, p.URL(), 301)
}
func (a *App) legacy(mux *http.ServeMux) {
	aliases := map[string]string{"index": "/", "news": "/posts", "dashboard": "/admin", "manage-news": "/admin/posts", "new-post": "/admin/posts/create", "manage-guestbook": "/admin/guestbook"}
	for _, p := range []string{"about-me", "projects", "music", "games", "contact", "donate", "keys", "guestbook", "login"} {
		aliases[p] = "/" + p
	}
	for old, target := range aliases {
		for _, suffix := range []string{"", ".php"} {
			path := "/" + old + suffix
			if path == target {
				continue
			}
			mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
				if old == "index" && r.URL.Query().Has("id") {
					a.redirectPost(w, r, r.URL.Query().Get("id"))
					return
				}
				http.Redirect(w, r, target, 301)
			})
		}
	}
	for _, path := range []string{"/modify-post", "/modify-post.php"} {
		mux.HandleFunc("GET "+path, a.auth(func(w http.ResponseWriter, r *http.Request) {
			n, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			if err != nil || n < 1 {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, route("admin.posts.edit", n), 301)
		}))
	}
	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) { a.redirectPost(w, r, r.PathValue("id")) })
	// Legacy destructive GET endpoints intentionally have no aliases.
}
