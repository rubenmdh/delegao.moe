package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PublicationStatus struct {
	Ready, Building bool
	BuiltAt         time.Time
	Pages, Workers  int
	Error           string
}

type publicationFile struct {
	Name string `json:"name"`
	ETag string `json:"etag"`
}

type publicationManifest struct {
	Version  int                        `json:"version"`
	Name     string                     `json:"generation"`
	BuiltAt  time.Time                  `json:"built_at"`
	Files    map[string]publicationFile `json:"files"`
	PostURLs map[string]string          `json:"posts"`
}

// Publisher keeps MySQL as the editable source and serves immutable HTML files.
// Its mutation lock must wrap every post write, including unpublishing/deleting.
type Publisher struct {
	app       *App
	dir       string
	workers   int
	buildMu   sync.Mutex
	mu        sync.RWMutex
	active    *publicationManifest
	building  bool
	dirty     bool
	lastError string
	// A separate source function makes generation failure and streaming behavior
	// testable without mocking SQL's implementation details.
	source func(context.Context, func(Post) error) error
}

var publicInformationPages = []string{"about-me", "projects", "music", "games", "contact", "donate", "keys"}
var generationName = regexp.MustCompile(`^generation-[0-9a-f]{32}$`)
var publicationFilename = regexp.MustCompile(`^[0-9a-f]{64}\.html$`)
var publicationETag = regexp.MustCompile(`^"[0-9a-f]{64}"$`)

func newPublisher(a *App, dir string, workers int) (*Publisher, error) {
	if workers == 0 {
		workers = 2
	}
	if workers < 1 || workers > 4 {
		return nil, errors.New("BUILD_WORKERS must be between 1 and 4")
	}
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("PUBLIC_DIR must not be empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0750); err != nil {
		return nil, fmt.Errorf("create publication directory: %w", err)
	}
	p := &Publisher{app: a, dir: abs, workers: workers}
	if a.store != nil {
		p.source = a.store.publicationPosts
	}
	if _, err := os.Stat(filepath.Join(abs, "dirty")); err == nil {
		p.dirty = true
		p.lastError = "The previous publication did not finish. Rebuild before serving public pages."
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := p.restore(); err != nil {
		// Cache damage must not make the database-backed repair path unusable.
		// Startup performs a full rebuild before opening the listener.
		p.active = nil
		p.dirty = true
		p.lastError = "Cached publication needs rebuilding: " + err.Error()
	}
	return p, nil
}

func (p *Publisher) Status() PublicationStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := PublicationStatus{Ready: p.active != nil && !p.dirty, Building: p.building, Workers: p.workers, Error: p.lastError}
	if p.active != nil {
		s.BuiltAt = p.active.BuiltAt
		s.Pages = len(p.active.Files)
	}
	return s
}

func (p *Publisher) CurrentURL(id int64) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.active == nil || p.dirty {
		return "", false
	}
	url, ok := p.active.PostURLs[strconv.FormatInt(id, 10)]
	return url, ok
}

// Block is used when startup cannot reconcile the cached generation with
// MySQL. The dynamic repair/admin paths can still run, but public HTML cannot
// be served merely because an older generation was present on disk.
func (p *Publisher) Block(cause error) {
	p.buildMu.Lock()
	defer p.buildMu.Unlock()
	message := "The public site needs rebuilding."
	if cause != nil {
		message = cause.Error()
	}
	p.mu.Lock()
	p.dirty = true
	p.lastError = message
	p.mu.Unlock()
	if err := writePublicationFile(filepath.Join(p.dir, "dirty"), []byte("publication blocked\n")); err != nil {
		slog.Warn("could not persist blocked publication", "error", err)
		return
	}
	if err := syncPublicationDirectory(p.dir); err != nil {
		slog.Warn("could not persist blocked publication directory", "error", err)
	}
}

// Mutate records a durable dirty state before the write. If writing or building
// fails, public HTML fails closed until a successful rebuild; an unpublished or
// deleted post must never remain available from a previous generation.
func (p *Publisher) Mutate(ctx context.Context, mutate func(context.Context) error) error {
	p.buildMu.Lock()
	defer p.buildMu.Unlock()
	if err := writePublicationFile(filepath.Join(p.dir, "dirty"), []byte("publication pending\n")); err != nil {
		return fmt.Errorf("mark publication pending: %w", err)
	}
	if err := syncPublicationDirectory(p.dir); err != nil {
		return fmt.Errorf("persist pending publication: %w", err)
	}
	p.mu.Lock()
	p.dirty = true
	p.lastError = "Publication pending."
	p.mu.Unlock()
	if err := mutate(ctx); err != nil {
		p.recordError(err)
		return err
	}
	if err := p.rebuild(ctx); err != nil {
		return fmt.Errorf("saved in MySQL, but publication failed: %w", err)
	}
	return nil
}

func (p *Publisher) Rebuild(ctx context.Context) error {
	p.buildMu.Lock()
	defer p.buildMu.Unlock()
	return p.rebuild(ctx)
}

func (p *Publisher) recordError(err error) {
	p.mu.Lock()
	p.lastError = err.Error()
	p.mu.Unlock()
}

type publicationJob struct {
	path string
	view View
	// The latest article produces the homepage using the same body, rather than
	// queueing a duplicate full-size Markdown body.
	home bool
}

func (p *Publisher) rebuild(ctx context.Context) (result error) {
	p.mu.Lock()
	p.building = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.building = false
		if result != nil {
			p.lastError = result.Error()
		}
		p.mu.Unlock()
	}()
	if p.source == nil {
		return errors.New("publication source is not configured")
	}
	staging, err := os.MkdirTemp(p.dir, ".building-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	nameToken := sha256.Sum256([]byte(token()))
	name := "generation-" + hex.EncodeToString(nameToken[:16])
	manifest := &publicationManifest{Version: 1, Name: name, BuiltAt: time.Now().UTC(), Files: make(map[string]publicationFile), PostURLs: make(map[string]string)}
	buildCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan publicationJob, p.workers)
	var wg sync.WaitGroup
	var fileMu sync.Mutex
	var firstError error
	var errorOnce sync.Once
	fail := func(err error) { errorOnce.Do(func() { firstError = err; cancel() }) }
	write := func(path string, v View) error {
		if err := buildCtx.Err(); err != nil {
			return err
		}
		var html bytes.Buffer
		if err := p.app.templates.ExecuteTemplate(&html, "layout", v); err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		key := sha256.Sum256([]byte(path))
		fileName := hex.EncodeToString(key[:]) + ".html"
		checksum := sha256.Sum256(html.Bytes())
		if err := writePublicationFile(filepath.Join(staging, fileName), html.Bytes()); err != nil {
			return err
		}
		fileMu.Lock()
		manifest.Files[path] = publicationFile{Name: fileName, ETag: `"` + hex.EncodeToString(checksum[:]) + `"`}
		fileMu.Unlock()
		return nil
	}
	for range p.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-buildCtx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					if err := write(job.path, job.view); err != nil {
						fail(err)
						return
					}
					if job.home {
						v := job.view
						v.Home = true
						v.Title = "Delegao.moe"
						if err := write("/", v); err != nil {
							fail(err)
							return
						}
					}
				}
			}
		}()
	}
	send := func(job publicationJob) error {
		select {
		case jobs <- job:
			return nil
		case <-buildCtx.Done():
			return buildCtx.Err()
		}
	}
	page := 1
	count := 0
	var excerpts []Post
	archive := func(next bool) error {
		v := View{Title: "News", Page: "posts", Posts: excerpts, PageNumber: page}
		if page > 1 {
			v.Previous = page - 1
		}
		if next {
			v.Next = page + 1
		}
		path := "/posts"
		if page > 1 {
			path += "?page=" + strconv.Itoa(page)
		}
		if err := send(publicationJob{path: path, view: v}); err != nil {
			return err
		}
		excerpts = nil
		page++
		return nil
	}
	err = p.source(buildCtx, func(post Post) error {
		if !post.Published || post.ID < 1 {
			return errors.New("publication source returned an unpublished or invalid post")
		}
		if len(excerpts) == 20 {
			if err := archive(true); err != nil {
				return err
			}
		}
		manifest.PostURLs[strconv.FormatInt(post.ID, 10)] = post.URL()
		if err := send(publicationJob{path: post.URL(), view: View{Title: post.Title, Page: "posts", Single: true, Posts: []Post{post}}, home: count == 0}); err != nil {
			return err
		}
		count++
		// Copy a short prefix so the archive cannot retain a full Markdown body.
		post.Body = publicationExcerpt(post.Body)
		excerpts = append(excerpts, post)
		return nil
	})
	if err == nil {
		err = archive(false)
	}
	if err == nil && count == 0 {
		err = send(publicationJob{path: "/", view: View{Title: "Delegao.moe", Page: "posts", Single: true, Home: true}})
	}
	for _, page := range publicInformationPages {
		if err != nil {
			break
		}
		err = send(publicationJob{path: "/" + page, view: View{Title: strings.ToUpper(page[:1]) + page[1:], Page: page, Wide: page == "donate" || page == "keys"}})
	}
	if err != nil {
		fail(err)
	}
	close(jobs)
	wg.Wait()
	if firstError != nil {
		return firstError
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := writePublicationFile(filepath.Join(staging, "manifest.json"), data); err != nil {
		return err
	}
	if err := syncPublicationDirectory(staging); err != nil {
		return err
	}
	finalDir := filepath.Join(p.dir, name)
	if err := os.Rename(staging, finalDir); err != nil {
		return err
	}
	// A generation is invisible until this durable, atomically replaced pointer.
	if err := p.writeCurrent(data); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(p.dir, "dirty")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear pending publication: %w", err)
	}
	if err := syncPublicationDirectory(p.dir); err != nil {
		return fmt.Errorf("persist completed publication: %w", err)
	}
	p.mu.Lock()
	previous := p.active
	p.active = manifest
	p.dirty = false
	p.lastError = ""
	p.mu.Unlock()
	p.prune(previous)
	return nil
}

func publicationExcerpt(source string) string {
	end, count := len(source), 0
	for offset := range source {
		if count == 251 {
			end = offset
			break
		}
		count++
	}
	return strings.Clone(source[:end])
}

func writePublicationFile(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (p *Publisher) writeCurrent(data []byte) error {
	temp, err := os.CreateTemp(p.dir, ".current-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0640); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return replacePublicationPointer(name, filepath.Join(p.dir, "current.json"))
}

func (p *Publisher) restore() error {
	f, err := os.Open(filepath.Join(p.dir, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var manifest publicationManifest
	if err := json.NewDecoder(io.LimitReader(f, 16<<20)).Decode(&manifest); err != nil {
		return fmt.Errorf("invalid publication manifest: %w", err)
	}
	if manifest.Version != 1 || !generationName.MatchString(manifest.Name) || manifest.BuiltAt.IsZero() || len(manifest.Files) < 9 {
		return errors.New("invalid publication manifest metadata")
	}
	info, err := os.Lstat(filepath.Join(p.dir, manifest.Name))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("publication generation is missing or unsafe")
	}
	for route, file := range manifest.Files {
		if !validPublicationRoute(route) || !publicationFilename.MatchString(file.Name) || !publicationETag.MatchString(file.ETag) {
			return errors.New("unsafe publication manifest entry")
		}
		info, err := os.Lstat(filepath.Join(p.dir, manifest.Name, file.Name))
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("publication file is missing or not regular")
		}
	}
	for id, path := range manifest.PostURLs {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || n < 1 || !strings.HasPrefix(path, "/posts/"+id+"/") {
			return errors.New("invalid canonical publication URL")
		}
		if _, ok := manifest.Files[path]; !ok {
			return errors.New("canonical publication page is missing")
		}
	}
	p.active = &manifest
	return nil
}

func validPublicationRoute(path string) bool {
	if path == "/" || path == "/posts" {
		return true
	}
	for _, page := range publicInformationPages {
		if path == "/"+page {
			return true
		}
	}
	if strings.HasPrefix(path, "/posts?page=") {
		n, err := strconv.Atoi(strings.TrimPrefix(path, "/posts?page="))
		return err == nil && n > 1
	}
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "posts" || parts[3] == "" {
		return false
	}
	n, err := strconv.ParseInt(parts[2], 10, 64)
	return err == nil && n > 0 && slug(parts[3]) == parts[3]
}

func (p *Publisher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	p.mu.RLock()
	manifest, dirty := p.active, p.dirty
	p.mu.RUnlock()
	if manifest == nil || dirty {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "5")
		http.Error(w, "The site is being published. Please try again shortly.", http.StatusServiceUnavailable)
		return
	}
	path := r.URL.Path
	id := ""
	if path == "/" && r.URL.Query().Has("id") {
		id = r.URL.Query().Get("id")
		if id == "" {
			http.NotFound(w, r)
			return
		}
	} else if strings.HasPrefix(path, "/posts/") {
		parts := strings.Split(path, "/")
		if len(parts) == 3 || len(parts) == 4 {
			id = parts[2]
		}
	}
	if id != "" {
		n, err := strconv.ParseInt(id, 10, 64)
		canonical, ok := manifest.PostURLs[strconv.FormatInt(n, 10)]
		if err != nil || n < 1 || !ok {
			http.NotFound(w, r)
			return
		}
		if path != canonical {
			w.Header().Set("Cache-Control", "public, no-cache")
			http.Redirect(w, r, canonical, http.StatusMovedPermanently)
			return
		}
	}
	if path == "/posts" {
		if n := page(r); n > 1 {
			path += "?page=" + strconv.Itoa(n)
		}
	}
	file, ok := manifest.Files[path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(p.dir, manifest.Name, file.Name))
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "Published content is temporarily unavailable.", 503)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Header().Set("ETag", file.ETag)
	// Strong content ETags avoid sub-second rebuild ambiguity in Last-Modified.
	http.ServeContent(w, r, file.Name, time.Time{}, f)
}

func (p *Publisher) prune(previous *publicationManifest) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !generationName.MatchString(name) || name == p.active.Name || (previous != nil && name == previous.Name) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(p.dir, name)); err != nil {
			slog.Warn("could not remove retired publication", "error", err)
		}
	}
}
