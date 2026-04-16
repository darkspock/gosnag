package ai

import (
	"regexp"
	"strings"
)

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]+`)
	apiKeyPattern      = regexp.MustCompile(`(?i)\b(api[_-]?key)\s*[:=]\s*["']?[^"',\s]+["']?`)
	basicAuthURLPattern = regexp.MustCompile(`https?://[^/\s:@]+:[^/\s@]+@`)
)

func userVisibleAIError(err error) string {
	if err == nil {
		return "AI analysis temporarily unavailable"
	}

	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "AI analysis temporarily unavailable"
	}

	msg = bearerTokenPattern.ReplaceAllString(msg, "Bearer [redacted]")
	msg = apiKeyPattern.ReplaceAllString(msg, "$1=[redacted]")
	msg = basicAuthURLPattern.ReplaceAllString(msg, "https://[redacted]@")
	return msg
}
