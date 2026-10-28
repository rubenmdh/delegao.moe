package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishedServiceIconsAreLocalAndAvailable(t *testing.T) {
	posts := []Post{}
	p := publisherFixture(t, &posts)
	if err := p.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	assets, err := loadAssetURLs(web)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/", "/services"} {
		response := readPublished(p, "GET", route, nil)
		for _, service := range publicServices() {
			if !strings.Contains(response.Body.String(), `href="`+service.URL+`"`) || !strings.Contains(response.Body.String(), `src="`+assets[service.Icon]+`"`) {
				t.Fatalf("%s missing service or icon for %s", route, service.Name)
			}
		}
	}
	for _, service := range publicServices() {
		w := httptest.NewRecorder()
		p.app.handler().ServeHTTP(w, httptest.NewRequest("GET", assets[service.Icon], nil))
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/") {
			t.Fatalf("missing local favicon for %s", service.Name)
		}
	}
}
