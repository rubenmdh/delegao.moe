package main

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PageRendering is sampled for the dynamic public guestbook only.
// Load is the system's one-minute load average, not a CPU percentage.
type PageRendering struct {
	started   time.Time
	Load      string
	GoVersion string
}

// Elapsed is evaluated by the footer after the page body has been rendered.
// It excludes database queries, file synchronization and network delivery.
func (p *PageRendering) Elapsed() string {
	return formatRenderDuration(time.Since(p.started))
}

func formatRenderDuration(elapsed time.Duration) string {
	if elapsed < time.Microsecond {
		elapsed = time.Microsecond
	}
	unit, suffix := time.Microsecond, "µs"
	if elapsed >= time.Second {
		unit, suffix = time.Second, "s"
	} else if elapsed >= time.Millisecond {
		unit, suffix = time.Millisecond, "ms"
	}
	return fmt.Sprintf("%.2f%s", float64(elapsed)/float64(unit), suffix)
}

func (a *App) renderPage(v View) ([]byte, error) {
	v.Rendering = nil
	if !v.Admin && v.Page == "guestbook" {
		v.Rendering = &PageRendering{started: time.Now(), Load: systemLoad(), GoVersion: runtime.Version()}
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
