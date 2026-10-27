package main

import (
	"bytes"
	"html"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPostExcerptReadsMarkdownAsPlainText(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"formatting", "# Hello\n\n**Strong** and *soft* [words](https://example.org/private/path).", "Hello Strong and soft words."},
		{"inline code stays literal", "Use `go build` and `&amp;` with `\\*`.", "Use go build and &amp; with \\*."},
		{"entities and escaped punctuation", `Rock &amp; roll &#x1F600; &amp;lt; \*literal\*`, "Rock & roll 😀 &lt; *literal*"},
		{"unicode whitespace", "  Rubén\t&\nEspaña\n\n😀\u00a0東京  ", "Rubén & España 😀 東京"},
		{"image alt without location", "A ![tiny **avatar**](https://example.org/avatar.png) stays.", "A tiny avatar stays."},
		{"inline html", "one <b>bold</b> word <img src=x onerror=alert(1)> here", "one bold word here"},
		{"inline script contents", "Before <script>alert('private')</script> after <style>.secret{}</style> done.", "Before after done."},
		{"nested raw tag inside script", "Before <script>one<style>two</style>three</script> after.", "Before after."},
		{"script block", "<script>\nalert('private')\n</script>\n\nVisible.", "Visible."},
		{"code blocks", "Before.\n\n```html\n<script>secret</script>\n```\n\nAfter.", "Before. After."},
		{"list and table boundaries", "- one\n- two\n\n| Name | State |\n| --- | --- |\n| demo | good |", "one two Name State demo good"},
		{"empty", " \n\t ", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := postExcerpt(test.source, 400); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestPostExcerptBoundsUnicodeAndOnlyMarksRealTruncation(t *testing.T) {
	for _, test := range []struct {
		source string
		limit  int
		want   string
	}{
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"", 3, ""},
		{"hello", 5, "hello"},
		{"hello   \n ", 5, "hello"},
		{"hello world", 6, "hello…"},
		{"hello world", 9, "hello wo…"},
		{"😀😀😀", 3, "😀😀😀"},
		{"😀😀😀😀", 3, "😀😀…"},
		{"東京にようこそ", 5, "東京によ…"},
		{"long text", 1, "…"},
		{"invalid \xff text", 100, "invalid � text"},
	} {
		t.Run(test.source, func(t *testing.T) {
			got := postExcerpt(test.source, test.limit)
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			if !utf8.ValidString(got) || (test.limit > 0 && utf8.RuneCountInString(got) > test.limit) {
				t.Fatalf("invalid or oversized excerpt: %q", got)
			}
		})
	}
	long := strings.Repeat("**word** ", 10000)
	if got := postExcerpt(long, 400); !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) > 400 {
		t.Fatal("long Markdown did not produce a bounded excerpt")
	}
}

func TestPostExcerptResolvesEntitiesExactlyOnceLikeArticleRendering(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`AT\&T`, "AT&T"},
		{`\&amp;`, "&amp;"},
		{`&amp;amp;`, "&amp;"},
		{`&#38;amp;`, "&amp;"},
		{`&#x26;amp;`, "&amp;"},
		{`&#x26;#38;`, "&#38;"},
		{`\&#38;amp;`, "&#38;amp;"},
		{`&amp;#38;`, "&#38;"},
		{`&notit;`, "&notit;"},
		{`&copy`, "&copy"},
	} {
		t.Run(test.source, func(t *testing.T) {
			got := postExcerpt(test.source, 400)
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			var rendered bytes.Buffer
			if err := excerptMarkdown.Convert([]byte(test.source), &rendered); err != nil {
				t.Fatal(err)
			}
			// These fixtures contain one plain paragraph: only the renderer's
			// known paragraph wrapper and its output escaping are removed.
			article := strings.TrimSuffix(strings.TrimPrefix(rendered.String(), "<p>"), "</p>\n")
			if visible := html.UnescapeString(article); got != visible {
				t.Fatalf("excerpt %q differs from rendered article text %q", got, visible)
			}
		})
	}
	code := `AT\&T &amp;amp; &#38;amp; &#x26;amp;`
	if got := postExcerpt("`"+code+"`", 400); got != code {
		t.Fatalf("code-span references changed: %q", got)
	}
}
