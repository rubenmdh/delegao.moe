package main

import (
	"bufio"
	"bytes"
	"html"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var excerptMarkdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// postExcerpt extracts visible prose rather than exposing Markdown syntax or
// link destinations. The limit includes the ellipsis when text is shortened.
// Its result is ordinary text: templates must keep escaping it at output time.
func postExcerpt(source string, limit int) string {
	if limit <= 0 || source == "" {
		return ""
	}
	input := []byte(source)
	document := excerptMarkdown.Parser().Parse(text.NewReader(input))
	plain := make([]rune, 0, min(limit, 1023)+1)
	space := false
	suppressedTag := ""
	var escaped bytes.Buffer
	textWriter := bufio.NewWriterSize(&escaped, 256)
	appendText := func(value []byte, resolve bool) bool {
		if resolve {
			// Resolve Markdown escapes and references in the same single pass as
			// the article renderer. Then remove only its HTML output escaping;
			// resolving numeric and named entities sequentially would double-decode
			// inputs such as "&#38;amp;" and lose escaped ampersands.
			escaped.Reset()
			goldmarkhtml.DefaultWriter.Write(textWriter, value)
			_ = textWriter.Flush() // bytes.Buffer writes cannot fail.
			value = []byte(html.UnescapeString(escaped.String()))
		}
		for _, r := range string(value) {
			if unicode.IsSpace(r) {
				space = true
				continue
			}
			if space && len(plain) > 0 {
				plain = append(plain, ' ')
				if len(plain) > limit {
					return false
				}
			}
			space = false
			plain = append(plain, r)
			if len(plain) > limit {
				return false
			}
		}
		return true
	}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if node.Type() == ast.TypeBlock {
				space = true
			}
			return ast.WalkContinue, nil
		}
		if raw, ok := node.(*ast.RawHTML); ok {
			markup := strings.ToLower(strings.TrimSpace(string(raw.Text(input))))
			name := strings.TrimLeft(markup, "</")
			if end := strings.IndexAny(name, " \t\r\n/>"); end >= 0 {
				name = name[:end]
			}
			if suppressedTag != "" && strings.HasPrefix(markup, "</") && name == suppressedTag {
				suppressedTag = ""
				space = true
			} else if suppressedTag == "" && !strings.HasPrefix(markup, "</") && (name == "script" || name == "style") {
				suppressedTag = name
				space = true
			}
			return ast.WalkSkipChildren, nil
		}
		if suppressedTag != "" {
			return ast.WalkContinue, nil
		}
		var complete = true
		switch n := node.(type) {
		case *ast.HTMLBlock, *ast.CodeBlock, *ast.FencedCodeBlock:
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				if !appendText(child.(*ast.Text).Value(input), false) {
					return ast.WalkStop, nil
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			complete = appendText(n.Value(input), !n.IsRaw())
			if n.SoftLineBreak() || n.HardLineBreak() {
				space = true
			}
		case *ast.String:
			complete = appendText(n.Value, !n.IsRaw() && !n.IsCode())
		case *ast.AutoLink:
			complete = appendText(n.Label(input), true)
		}
		if !complete {
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if len(plain) <= limit {
		return string(plain)
	}
	cut := limit - 1
	// Prefer a nearby word boundary, but keep scripts without spaces useful and
	// avoid dropping most of the preview merely because its last word is long.
	for i := cut; i >= cut*3/4 && i > 0; i-- {
		if unicode.IsSpace(plain[i]) {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(plain[:cut])) + "…"
}
