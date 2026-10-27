package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"
)

// Hash once at application startup. Updated HTML then refers to the new CSS/JS
// even when a visitor still has the previous files cached for a day.
func loadAssetURLs(source fs.FS) (map[string]string, error) {
	urls := make(map[string]string)
	for _, pattern := range []string{"web/static/css/*.css", "web/static/js/*.js", "web/static/img/favicon.*", "web/static/img/apple-touch-icon.png"} {
		paths, err := fs.Glob(source, pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			data, err := fs.ReadFile(source, path)
			if err != nil {
				return nil, err
			}
			name := strings.TrimPrefix(path, "web/static/")
			digest := sha256.Sum256(data)
			urls[name] = fmt.Sprintf("/%s?v=%x", name, digest[:8])
		}
	}
	return urls, nil
}
