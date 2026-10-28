package main

import (
	"context"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGeneratedPagesHaveAccessiblePublicNavigation(t *testing.T) {
	posts := []Post{}
	publisher := publisherFixture(t, &posts)
	if err := publisher.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, page := range publicInformationPages {
		t.Run(page, func(t *testing.T) {
			response := readPublished(publisher, "GET", "/"+page, nil)
			html := response.Body.String()
			_, after, _ := strings.Cut(html, `<nav class="public-navigation" aria-label="Main navigation">`)
			menu, _, found := strings.Cut(after, "</nav>")
			if response.Code != 200 || !found || strings.Contains(menu, "<script") {
				t.Fatal("generated page lacks direct public navigation")
			}
			for _, path := range []string{"", "about-me", "projects", "services", "music", "games", "contact"} {
				if strings.Count(menu, `href="/`+path+`"`) != 1 {
					t.Fatalf("mobile menu must contain exactly one link to %s", path)
				}
			}
			activeArea := menu
			if page == "keys" || page == "donate" {
				_, after, _ = strings.Cut(html, `<footer class="public-footer">`)
				activeArea, _, _ = strings.Cut(after, "</footer>")
				if strings.Contains(menu, `aria-current="page"`) {
					t.Fatal("unrelated primary page marked current")
				}
			}
			if strings.Count(activeArea, `aria-current="page"`) != 1 || !regexp.MustCompile(`<a\b[^>]*href="/`+page+`"[^>]*aria-current="page"`).MatchString(activeArea) {
				t.Fatal("navigation does not identify the current page")
			}
			if !strings.Contains(html, `href="#main-content"`) || !strings.Contains(html, `<main id="main-content"`) {
				t.Fatal("skip link has no main-content target")
			}
			for _, removed := range []string{`data-site-menu`, `id="memo"`, "Climate activist. Archivist.", "Student."} {
				if strings.Contains(html, removed) {
					t.Fatalf("obsolete public profile or navigation remains: %s", removed)
				}
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
			t.Fatal("unmasked email or deletion controls leaked or disappeared")
		}
		if !admin && !strings.Contains(html, `data-email-url="/guestbook/13/email"`) {
			t.Fatal("masked public contact is missing")
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
