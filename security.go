package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type session struct {
	User           string
	CSRF           string
	Expires        time.Time
	EmailsUnlocked bool
}

func (s *security) unlockEmails(r *http.Request) bool {
	cookie, err := r.Cookie("delegao_session")
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[cookie.Value]
	if !ok || !time.Now().Before(sess.Expires) {
		return false
	}
	sess.EmailsUnlocked = true
	s.sessions[cookie.Value] = sess
	return true
}

type attempt struct {
	Count   int
	Expires time.Time
}
type security struct {
	mu       sync.Mutex
	sessions map[string]session
	attempts map[string]attempt
	secure   bool
}

func newSecurity(secure bool) *security {
	return &security{sessions: make(map[string]session), attempts: make(map[string]attempt), secure: secure}
}
func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *security) current(r *http.Request) (session, bool) {
	c, err := r.Cookie("delegao_session")
	if err != nil {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[c.Value]
	if ok && !time.Now().Before(sess.Expires) {
		delete(s.sessions, c.Value)
		return session{}, false
	}
	return sess, ok
}
func (s *security) issue(w http.ResponseWriter, r *http.Request, user string) (session, bool) {
	if user == "" && !s.allow(r, "session", 60) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many new sessions. Please try again later.", 429)
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sess := range s.sessions {
		if !now.Before(sess.Expires) {
			delete(s.sessions, id)
		}
	}
	if len(s.sessions) >= 5000 {
		http.Error(w, "Please try again later.", http.StatusServiceUnavailable)
		return session{}, false
	}
	if old, err := r.Cookie("delegao_session"); err == nil {
		delete(s.sessions, old.Value)
	}
	ttl := 30 * time.Minute
	if user != "" {
		ttl = 8 * time.Hour
	}
	sess := session{User: user, CSRF: token(), Expires: now.Add(ttl)}
	id := token()
	s.sessions[id] = sess
	http.SetCookie(w, &http.Cookie{Name: "delegao_session", Value: id, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(ttl.Seconds())})
	return sess, true
}
func (s *security) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("delegao_session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "delegao_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
}

func (s *security) revokeUser(user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.User == user {
			delete(s.sessions, id)
		}
	}
}
func (s *security) csrf(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := s.current(r)
	// Check Origin when sent, in addition to an unpredictable session-bound token.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if s.secure || r.TLS != nil {
			scheme = "https"
		}
		if err != nil || u.Host != r.Host || u.Scheme != scheme {
			http.Error(w, "Invalid request origin.", 403)
			return false
		}
	}
	if !ok || subtle.ConstantTimeCompare([]byte(sess.CSRF), []byte(r.PostFormValue("csrf"))) != 1 {
		http.Error(w, "Your form has expired. Reload the page and try again.", 403)
		return false
	}
	return true
}
func (s *security) allow(r *http.Request, scope string, limit int) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return s.allowKey(scope+":"+ip, limit)
}

func (s *security) allowKey(key string, limit int) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, a := range s.attempts {
		if !now.Before(a.Expires) {
			delete(s.attempts, k)
		}
	}
	a := s.attempts[key]
	if a.Expires.IsZero() {
		if len(s.attempts) >= 10000 {
			return false
		}
		a.Expires = now.Add(15 * time.Minute)
	}
	a.Count++
	s.attempts[key] = a
	return a.Count <= limit
}
