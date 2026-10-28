package main

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Post struct {
	ID                        int64
	Title, Body, Tags, Author string
	Published                 bool
	Created                   time.Time
	Updated                   *time.Time
}

func (p Post) URL() string { return route("posts.show", p.ID, slug(p.Title)) }

type Guest struct {
	ID                            int64
	Name, Email, Comment, Country string
	Date                          time.Time
}

type User struct {
	ID                 int64
	Username, Password string
}

func slug(s string) string {
	var out strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	if out.Len() == 0 {
		return "post"
	}
	return out.String()
}

// Named routes keep URL generation and registration in one place, like Laravel.
var routes = map[string]string{
	"home": "/", "posts.index": "/posts", "posts.show": "/posts/{id}/{slug}",
	"login": "/login", "logout": "/logout", "guestbook": "/guestbook",
	"guestbook.email":        "/guestbook/{id}/email",
	"guestbook.email-unlock": "/guestbook/unlock-emails",
	"services":               "/services",
	"admin":                  "/admin", "admin.posts": "/admin/posts", "admin.posts.create": "/admin/posts/create",
	"admin.posts.edit": "/admin/posts/{id}/edit", "admin.posts.update": "/admin/posts/{id}",
	"admin.posts.delete": "/admin/posts/{id}/delete", "admin.guests": "/admin/guestbook",
	"admin.guests.delete": "/admin/guestbook/{id}/delete",
	"admin.preview":       "/admin/posts/preview", "admin.rebuild": "/admin/rebuild",
	"admin.password": "/admin/password",
}

func route(name string, values ...any) string {
	path, ok := routes[name]
	if !ok {
		panic("unknown route: " + name)
	}
	for _, value := range values {
		start := strings.IndexByte(path, '{')
		end := strings.IndexByte(path, '}')
		if start < 0 || end < start {
			panic("too many route parameters")
		}
		path = path[:start] + fmt.Sprint(value) + path[end+1:]
	}
	if strings.Contains(path, "{") {
		panic("missing route parameter")
	}
	return path
}
