package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func generateToken(bytes int) string {
	b := make([]byte, bytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func sessionCookie(r *http.Request, baseURL, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "session",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  cookieExpiry(maxAge),
		HttpOnly: true,
		Secure:   shouldSecureCookie(r, baseURL),
		SameSite: http.SameSiteLaxMode,
	}
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, baseURL string) {
	http.SetCookie(w, sessionCookie(r, baseURL, "", -1))
}

func shouldSecureCookie(r *http.Request, baseURL string) bool {
	if r != nil {
		if r.TLS != nil {
			return true
		}
		if proto := forwardedProto(r.Header.Get("X-Forwarded-Proto")); strings.EqualFold(proto, "https") {
			return true
		}
	}

	if baseURL == "" {
		return false
	}

	u, err := url.Parse(baseURL)
	return err == nil && strings.EqualFold(u.Scheme, "https")
}

func forwardedProto(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	return strings.TrimSpace(parts[0])
}

func cookieExpiry(maxAge int) time.Time {
	switch {
	case maxAge < 0:
		return time.Unix(0, 0)
	case maxAge == 0:
		return time.Time{}
	default:
		return time.Now().Add(time.Duration(maxAge) * time.Second)
	}
}
