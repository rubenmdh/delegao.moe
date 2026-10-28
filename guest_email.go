package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func validGuestEmail(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func maskGuestEmail(value string) string {
	if !validGuestEmail(value) {
		return ""
	}
	local, _, _ := strings.Cut(value, "@")
	first, _ := utf8.DecodeRuneInString(local)
	return string(first) + "••• [at] •••"
}

func (a *App) guestEmail(w http.ResponseWriter, r *http.Request) {
	// The common POST middleware has already verified session-bound CSRF and
	// Origin. This is intentionally public contact data, not an admin endpoint.
	sess, ok := a.security.current(r)
	if !ok || !sess.EmailsUnlocked {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "dish_required"})
		return
	}
	if !a.security.allow(r, "guest-email", 20) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many contact requests. Please try again later.", http.StatusTooManyRequests)
		return
	}
	n, err := id(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var email string
	err = a.store.db.QueryRowContext(r.Context(), "SELECT email FROM guestbook WHERE id=?", n).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !validGuestEmail(email) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// no-store is also set by the shared middleware; never cache contact data.
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Email string `json:"email"`
	}{Email: email})
}

func (a *App) unlockGuestEmails(w http.ResponseWriter, r *http.Request) {
	if sess, ok := a.security.current(r); ok && sess.EmailsUnlocked {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !a.security.allow(r, "guest-email-unlock", 10) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many answers. Please try again later.", http.StatusTooManyRequests)
		return
	}
	if answer := field(r, "dishoftheday"); !valid(answer, 1, 100) || answer != a.answer {
		http.Error(w, "That is not the dish of the day. Check the About me page.", http.StatusUnprocessableEntity)
		return
	}
	if !a.security.unlockEmails(r) {
		http.Error(w, "Your session expired. Reload the page.", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
