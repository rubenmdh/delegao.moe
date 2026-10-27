package main

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// PageRendering is sampled while generating HTML, never while serving a cached
// publication. Load is the system's one-minute load average, not a CPU percentage.
type PageRendering struct {
	started time.Time
	Load    string
}

// Elapsed is evaluated by the footer after the page body has been rendered.
// It excludes database queries, file synchronization and network delivery.
func (p *PageRendering) Elapsed() string {
	elapsed := time.Since(p.started).Round(time.Microsecond)
	if elapsed < time.Microsecond {
		elapsed = time.Microsecond
	}
	return elapsed.String()
}

func (a *App) renderPage(v View) ([]byte, error) {
	if !v.Admin && v.Page != "login" {
		v.Rendering = &PageRendering{started: time.Now(), Load: systemLoad()}
	}
	var html bytes.Buffer
	if err := a.templates.ExecuteTemplate(&html, "layout", v); err != nil {
		return nil, err
	}
	return html.Bytes(), nil
}

func formatSystemLoad(source string) string {
	fields := strings.Fields(source)
	if len(fields) == 0 {
		return ""
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || load < 0 || math.IsNaN(load) || math.IsInf(load, 0) {
		return ""
	}
	return fmt.Sprintf("%.2f", load)
}
