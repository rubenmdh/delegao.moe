package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatSystemLoad(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"0.23 0.17 0.12 1/64 234", "0.23"},
		{"  2.5\t1.0 0.5\n", "2.50"},
		{"0.00 0.00 0.00 1/12 123", "0.00"},
		{"", ""}, {"invalid", ""}, {"-0.1", ""},
		{"NaN", ""}, {"+Inf", ""}, {"1e999", ""},
	} {
		if got := formatSystemLoad(tc.source); got != tc.want {
			t.Errorf("formatSystemLoad(%q) = %q; want %q", tc.source, got, tc.want)
		}
	}
}

func TestRenderingFooterScope(t *testing.T) {
	a := testApp(t, nil)
	for _, tc := range []struct {
		view View
		want bool
	}{
		{View{Page: "about-me", Title: "About"}, true},
		{View{Page: "guestbook", Title: "Guestbook"}, true},
		{View{Page: "login", Title: "Login"}, false},
		{View{Page: "posts", Title: "Posts", Admin: true}, false},
	} {
		body, err := a.renderPage(tc.view)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(body), "HTML generated in "); got != tc.want {
			t.Errorf("page %s admin=%v: rendering footer = %v, want %v", tc.view.Page, tc.view.Admin, got, tc.want)
		}
	}
}

func TestRenderingElapsed(t *testing.T) {
	p := &PageRendering{started: time.Now().Add(-2 * time.Millisecond)}
	duration, err := time.ParseDuration(p.Elapsed())
	if err != nil || duration < 2*time.Millisecond {
		t.Fatalf("invalid elapsed duration: %v, %v", duration, err)
	}
}
