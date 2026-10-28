package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func (s *Store) replacePassword(ctx context.Context, user User, hash string) (bool, error) {
	result, err := s.db.ExecContext(ctx, "UPDATE users SET password=? WHERE id=? AND password=?", hash, user.ID, user.Password)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (a *App) passwordForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, View{Page: "password", Title: "Change password"})
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	sess, _ := a.security.current(r)
	if !a.security.allow(r, "password", 5) || !a.security.allowKey("password-user:"+sess.User, 5) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many password change attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}
	fail := func(message string) {
		a.render(w, r, http.StatusUnprocessableEntity, View{Page: "password", Title: "Change password", Error: message})
	}
	current, next, confirmation := r.PostFormValue("current_password"), r.PostFormValue("new_password"), r.PostFormValue("confirm_password")
	if !valid(next, 15, 72) || len(next) > 72 {
		fail("Use at least 15 characters, up to 72 UTF-8 bytes, for the new password.")
		return
	}
	if next != confirmation {
		fail("The new passwords do not match.")
		return
	}
	if current == next {
		fail("Choose a password different from your current one.")
		return
	}
	u, err := a.store.user(r.Context(), sess.User)
	if errors.Is(err, sql.ErrNoRows) {
		a.security.logout(w, r)
		http.Redirect(w, r, route("login"), http.StatusSeeOther)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	if len(current) > 72 || bcrypt.CompareHashAndPassword([]byte(strings.Replace(u.Password, "$2y$", "$2a$", 1)), []byte(current)) != nil {
		fail("The current password is incorrect.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), 12)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.credentialsMu.Lock()
	defer a.credentialsMu.Unlock()
	// Another password change may have revoked this request's session while
	// bcrypt ran. Do not permit a revoked form to change credentials again.
	active, ok := a.security.current(r)
	if !ok || active.User != u.Username {
		http.Redirect(w, r, route("login"), http.StatusSeeOther)
		return
	}
	changed, err := a.store.replacePassword(r.Context(), u, string(hash))
	if err != nil {
		a.fail(w, err)
		return
	}
	if !changed {
		fail("The account changed while saving. Reload the form and try again.")
		return
	}
	a.security.revokeUser(u.Username)
	a.security.logout(w, r)
	http.Redirect(w, r, route("login")+"?notice=password-changed", http.StatusSeeOther)
}
