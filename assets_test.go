package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestChangedAssetsInvalidateTheirURLs(t *testing.T) {
	source := fstest.MapFS{
		"web/static/css/custom.css": {Data: []byte("body {color: white}")},
		"web/static/js/site.js":     {Data: []byte("'use strict';")},
	}
	before, err := loadAssetURLs(source)
	if err != nil {
		t.Fatal(err)
	}
	source["web/static/css/custom.css"].Data = []byte("body {color: white; margin: 0}")
	after, err := loadAssetURLs(source)
	if err != nil {
		t.Fatal(err)
	}
	if before["css/custom.css"] == after["css/custom.css"] || before["js/site.js"] != after["js/site.js"] {
		t.Fatal("CSS update must refresh its URL while preserving unchanged script URLs")
	}
}

func TestVersionedStylesheetIsRenderedAndServed(t *testing.T) {
	a := testApp(t, nil)
	assets, err := loadAssetURLs(web)
	if err != nil {
		t.Fatal(err)
	}
	address := assets["css/custom.css"]
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/about-me", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `href="`+address+`"`) {
		t.Fatal("page does not refer to the versioned stylesheet")
	}
	w = httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", address, nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "max-age=") {
		t.Fatal("versioned stylesheet is not served with browser caching")
	}
}

func TestFaviconsAreLinkedAndServed(t *testing.T) {
	a := testApp(t, nil)
	assets, err := loadAssetURLs(web)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/about-me", "/login"} {
		page := httptest.NewRecorder()
		a.handler().ServeHTTP(page, httptest.NewRequest("GET", path, nil))
		for _, name := range []string{"img/favicon.ico", "img/favicon.png", "img/apple-touch-icon.png"} {
			address := assets[name]
			if address == "" || !strings.Contains(page.Body.String(), `href="`+address+`"`) {
				t.Fatalf("%s does not link favicon %s", path, name)
			}
			icon := httptest.NewRecorder()
			a.handler().ServeHTTP(icon, httptest.NewRequest("GET", address, nil))
			if icon.Code != 200 || icon.Body.Len() == 0 || !strings.HasPrefix(icon.Header().Get("Content-Type"), "image/") {
				t.Fatalf("favicon %s is unavailable or has the wrong content type", name)
			}
		}
	}
}
