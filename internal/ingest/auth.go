package ingest

import (
	"net/http"
	"strings"
)

// ExtractPublicKey gets the sentry public key from the request.
// Supports X-Sentry-Auth header and sentry_key query param.
func ExtractPublicKey(r *http.Request) string {
	// Method 1: X-Sentry-Auth header
	auth := r.Header.Get("X-Sentry-Auth")
	if auth != "" {
		for _, part := range strings.Split(auth, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "sentry_key=") {
				return strings.TrimPrefix(part, "sentry_key=")
			}
			// Handle "Sentry sentry_key=..." format (first pair)
			if strings.HasPrefix(part, "Sentry ") {
				for _, subpart := range strings.Split(part, ",") {
					subpart = strings.TrimSpace(subpart)
					subpart = strings.TrimPrefix(subpart, "Sentry ")
					if strings.HasPrefix(subpart, "sentry_key=") {
						return strings.TrimPrefix(subpart, "sentry_key=")
					}
				}
			}
		}
	}

	// Method 2: query param
	if key := r.URL.Query().Get("sentry_key"); key != "" {
		return key
	}

	return ""
}
