package main

import (
	"context"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGeneratedPagesHaveAccessibleMobileNavigation(t *testing.T) {
	posts := []Post{}
	publisher := publisherFixture(t, &posts)
	if err := publisher.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"about-me", "projects", "music", "games", "contact", "donate", "keys"} {
		t.Run(page, func(t *testing.T) {
			response := readPublished(publisher, "GET", "/"+page, nil)
			html := response.Body.String()
			_, after, _ := strings.Cut(html, `<details class="mobile-site-menu" data-site-menu>`)
			menu, _, found := strings.Cut(after, "</details>")
			if response.Code != 200 || !found || strings.Contains(menu, "<script") {
				t.Fatal("generated page lacks native mobile navigation")
			}
			for _, path := range []string{"about-me", "projects", "music", "games", "contact", "guestbook", "keys", "donate"} {
				if strings.Count(menu, `href="/`+path+`"`) != 1 {
					t.Fatalf("mobile menu must contain exactly one link to %s", path)
				}
			}
			if strings.Count(menu, `aria-current="page"`) != 1 || !regexp.MustCompile(`<a\b[^>]*href="/`+page+`"[^>]*aria-current="page"`).MatchString(menu) {
				t.Fatal("mobile navigation does not identify the current page")
			}
			if !strings.Contains(html, `href="#main-content"`) || !strings.Contains(html, `<main id="main-content"`) {
				t.Fatal("skip link has no main-content target")
			}
			if blog, memo := strings.Index(html, `<section id="blog"`), strings.Index(html, `<aside id="memo"`); memo >= 0 && (blog < 0 || blog > memo) {
				t.Fatal("page content must precede the profile on mobile and in keyboard order")
			}
		})
	}
}

func TestGuestbookResponsiveMarkupAndPrivateFields(t *testing.T) {
	a := testApp(t, nil)
	guest := Guest{ID: 13, Name: "Guest <name>", Email: "private@example.com", Comment: strings.Repeat("long-comment-", 30) + " 😀", Country: "A long country name", Date: time.Date(2026, 10, 27, 22, 57, 0, 0, time.UTC)}
	for _, admin := range []bool{false, true} {
		response := httptest.NewRecorder()
		path := "/guestbook"
		if admin {
			path = "/admin/guestbook"
		}
		request := httptest.NewRequest("GET", path, nil)
		issued := httptest.NewRecorder()
		session, ok := a.security.issue(issued, request, "admin")
		if !ok {
			t.Fatal("could not issue fixture session")
		}
		request.AddCookie(issued.Result().Cookies()[0])
		a.render(response, request, 200, View{Page: "guestbook", Guests: []Guest{guest}, PageNumber: 1, Next: 2})
		html := response.Body.String()
		if response.Code != 200 || strings.Count(html, guest.Comment) != 1 || !strings.Contains(html, `scope="col" role="columnheader">Comment`) || !strings.Contains(html, `class="guest-field-label" aria-hidden="true">Country`) || !strings.Contains(html, `datetime="2026-10-27T22:57:00Z"`) {
			t.Fatal("guestbook lost data or accessible field headings")
		}
		if strings.Contains(html, guest.Email) != admin || strings.Contains(html, `/admin/guestbook/13/delete`) != admin {
			t.Fatal("private email or deletion controls leaked or disappeared")
		}
		if !strings.Contains(html, `aria-label="Pagination"`) || !strings.Contains(html, `href="?page=2"`) {
			t.Fatal("guestbook pagination disappeared")
		}
		if admin {
			if !strings.Contains(html, `value="`+session.CSRF+`"`) || !strings.Contains(html, `aria-label="Delete guestbook entry by Guest &lt;name&gt;"`) {
				t.Fatal("admin deletion lost CSRF protection or its accessible label")
			}
		} else if !strings.Contains(html, `aria-labelledby="guest-form-title"`) || !strings.Contains(html, `data-dialog="guest-form"`) {
			t.Fatal("guestbook form is not available or has no accessible name")
		}
	}
}
