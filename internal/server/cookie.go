package server

import (
	"net/http"
	"time"
)

// writeCookie sets a secure HTTP-only cookie
func (s *Server) writeCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Expires:  expires,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   !s.cfg.DevMode,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// readCookie safely reads a cookie by name
func readCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// clearCookie removes a cookie by setting its expiration to the past
func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	s.writeCookie(w, name, "", time.Unix(0, 0))
}
