package main

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AdminDashboard struct {
	Published, Drafts, Guests int
	Recent                    []Post
}

type AdminPublication struct {
	Ready, Building bool
	BuiltAt         time.Time
	Pages, Workers  int
	Error           string
}

func (s *Store) adminStats(ctx context.Context) (AdminDashboard, error) {
	var stats AdminDashboard
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(status=1),0), COALESCE(SUM(status<>1),0), (SELECT COUNT(*) FROM guestbook) FROM news").Scan(&stats.Published, &stats.Drafts, &stats.Guests)
	if err != nil {
		return stats, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,title,'',status,tags,posted_by,created_at,last_update FROM news ORDER BY id DESC LIMIT 5")
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return stats, err
		}
		stats.Recent = append(stats.Recent, post)
	}
	return stats, rows.Err()
}

func (a *App) adminDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := a.store.adminStats(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	a.render(w, r, http.StatusOK, View{Page: "dashboard", Title: "Overview", Dashboard: stats})
}

func adminFilters(r *http.Request) (string, string) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 100 {
		query = string([]rune(query)[:100])
	}
	filter := r.URL.Query().Get("status")
	if filter != "published" && filter != "draft" {
		filter = "all"
	}
	return query, filter
}

func adminPaginationURL(query, filter string, page int) string {
	values := url.Values{}
	values.Set("page", strconv.Itoa(page))
	if query != "" {
		values.Set("q", query)
	}
	if filter != "all" {
		values.Set("status", filter)
	}
	return route("admin.posts") + "?" + values.Encode()
}

func adminPostQuery(query, filter string, page int) (string, []any) {
	statement := "SELECT id,title,'',status,tags,posted_by,created_at,last_update FROM news WHERE 1=1"
	var args []any
	if query != "" {
		// INSTR searches literal text; '%' and '_' are not wildcard operators.
		statement += " AND (INSTR(LOWER(title),LOWER(?))>0 OR INSTR(LOWER(tags),LOWER(?))>0)"
		args = append(args, query, query)
	}
	switch filter {
	case "published":
		statement += " AND status=1"
	case "draft":
		statement += " AND status<>1"
	}
	statement += " ORDER BY id DESC LIMIT 21 OFFSET ?"
	args = append(args, (page-1)*20)
	return statement, args
}

func (a *App) adminPosts(w http.ResponseWriter, r *http.Request) {
	query, filter := adminFilters(r)
	number := page(r)
	statement, args := adminPostQuery(query, filter, number)
	if a.store.dialect == "sqlite" {
		statement = strings.ReplaceAll(statement, "LOWER(", "unicode_fold(")
	}
	rows, err := a.store.db.QueryContext(r.Context(), statement, args...)
	if err != nil {
		a.fail(w, err)
		return
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			a.fail(w, err)
			return
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		a.fail(w, err)
		return
	}
	view := View{Page: "posts", Title: "Posts", Posts: posts, Query: query, Filter: filter, PageNumber: number}
	if number > 1 {
		view.Previous = number - 1
		view.PreviousURL = adminPaginationURL(query, filter, number-1)
	}
	if len(posts) > 20 {
		view.Posts = posts[:20]
		view.Next = number + 1
		view.NextURL = adminPaginationURL(query, filter, number+1)
	}
	a.render(w, r, http.StatusOK, view)
}

func (a *App) adminPreview(w http.ResponseWriter, r *http.Request) {
	body := r.PostFormValue("body")
	if !valid(body, 0, 500000) {
		http.Error(w, "The preview is limited to 500,000 characters.", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(a.renderMarkdown(body)))
}
