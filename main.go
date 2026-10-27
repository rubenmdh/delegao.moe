package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return errors.New("DATABASE_DSN is required")
	}
	store, err := openStore(dsn)
	if err != nil {
		return err
	}
	defer store.db.Close()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return store.migrate(context.Background())
	}
	var migrated int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version='002_decode_legacy'").Scan(&migrated); err != nil || migrated != 1 {
		return errors.New("database migrations are required; run delegao migrate")
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "build":
		case "create-admin":
			if len(os.Args) != 3 || strings.TrimSpace(os.Args[2]) == "" || len(os.Args[2]) > 50 {
				return errors.New("usage: delegao create-admin USERNAME")
			}
			fmt.Fprint(os.Stderr, "Password (12–72 bytes): ")
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			if len(password) < 12 || len(password) > 72 {
				return errors.New("password must be 12–72 bytes")
			}
			hash, err := bcrypt.GenerateFromPassword(password, 12)
			clear(password)
			if err != nil {
				return err
			}
			_, err = store.db.Exec("INSERT INTO users (username,password) VALUES (?,?)", os.Args[2], string(hash))
			return err
		default:
			return errors.New("unknown command; use migrate, build or create-admin USERNAME")
		}
	}
	secure := os.Getenv("COOKIE_SECURE") != "false"
	answer := os.Getenv("GUESTBOOK_ANSWER")
	if answer == "" {
		answer = "Ravioli"
	}
	app, err := newApp(store, secure, answer)
	if err != nil {
		return err
	}
	app.proxies, err = parseProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return err
	}
	workers, err := envInt("BUILD_WORKERS", 2, 1, 4)
	if err != nil {
		return err
	}
	publicDir := os.Getenv("PUBLIC_DIR")
	if publicDir == "" {
		publicDir = "public"
	}
	app.publisher, err = newPublisher(app, publicDir, workers)
	if err != nil {
		return err
	}
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	err = app.publisher.Rebuild(buildCtx)
	buildCancel()
	if err != nil {
		if len(os.Args) > 1 && os.Args[1] == "build" {
			return fmt.Errorf("static publication failed: %w", err)
		}
		app.publisher.Block(err)
		slog.Error("public publication unavailable; administration can rebuild it", "error", err)
	} else {
		slog.Info("public pages generated", "directory", publicDir, "pages", app.publisher.Status().Pages, "workers", workers)
	}
	if len(os.Args) > 1 && os.Args[1] == "build" {
		return nil
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	srv := &http.Server{Addr: addr, Handler: app.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 3 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("listening", "address", addr, "secure_cookies", secure)
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 130*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			srv.Close()
			return err
		}
		return nil
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
