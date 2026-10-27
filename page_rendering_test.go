package main

import (
	"runtime"
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
		{View{Page: "about-me", Title: "About"}, false},
		{View{Page: "posts", Title: "Blog", Home: true}, false},
		{View{Page: "posts", Title: "Posts"}, false},
		{View{Page: "guestbook", Title: "Guestbook"}, true},
		{View{Page: "guestbook", Title: "Guestbook", Admin: true}, false},
		{View{Page: "login", Title: "Login"}, false},
		{View{Page: "posts", Title: "Posts", Admin: true}, false},
	} {
		body, err := a.renderPage(tc.view)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(body), `class="generation-note"`); got != tc.want {
			t.Errorf("page %s admin=%v: rendering footer = %v, want %v", tc.view.Page, tc.view.Admin, got, tc.want)
		}
		if tc.want && !strings.Contains(string(body), runtime.Version()+" in ") {
			t.Fatal("guestbook does not show the actual Go runtime version")
		}
	}
}

func TestFormatRenderDuration(t *testing.T) {
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "1.00µs"},
		{856 * time.Microsecond, "856.00µs"},
		{17607 * time.Microsecond, "17.61ms"},
		{1234 * time.Millisecond, "1.23s"},
	} {
		if got := formatRenderDuration(tc.elapsed); got != tc.want {
			t.Errorf("formatRenderDuration(%v) = %q, want %q", tc.elapsed, got, tc.want)
		}
	}
}
