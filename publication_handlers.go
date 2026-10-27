package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func (a *App) mutatePublic(ctx context.Context, change func(context.Context) error) error {
	if a.publisher == nil {
		return change(ctx)
	}
	return a.publisher.Mutate(ctx, change)
}

func (a *App) rebuildPublic(w http.ResponseWriter, r *http.Request) {
	if a.publisher == nil {
		http.Error(w, "Static publication is not configured.", 503)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	if err := a.publisher.Rebuild(ctx); err != nil {
		slog.Error("publication rebuild failed", "error", err)
		a.render(w, r, 503, View{Page: "dashboard", Title: "Publication needs attention", Error: "Public pages could not be rebuilt. Check the server logs and available disk space, then retry."})
		return
	}
	http.Redirect(w, r, route("admin")+"?notice=rebuilt", 303)
}
