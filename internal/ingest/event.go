package ingest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// SentryEvent represents the JSON structure sent by Sentry SDKs.
type SentryEvent struct {
	EventID     string            `json:"event_id"`
	Timestamp   any               `json:"timestamp"` // string or float
	Platform    string            `json:"platform"`
	Level       string            `json:"level"`
	Logger      string            `json:"logger"`
	Transaction string            `json:"transaction"`
	ServerName  string            `json:"server_name"`
	Release     string            `json:"release"`
	Dist        string            `json:"dist"`
	Environment string            `json:"environment"`
	Message     string            `json:"message"`
	Logentry    *LogEntry         `json:"logentry"`
	Exception   *ExceptionData    `json:"exception"`
	Tags        map[string]string `json:"tags"`
	Extra       map[string]any    `json:"extra"`
	Fingerprint []string          `json:"fingerprint"`
	User        map[string]any    `json:"user"`
	Request     map[string]any    `json:"request"`
	Contexts    map[string]any    `json:"contexts"`
	Breadcrumbs json.RawMessage   `json:"breadcrumbs"`
	SDK         map[string]any    `json:"sdk"`
	Modules     map[string]string `json:"modules"`
	Raw         map[string]any    `json:"-"` // full raw event for storage
}

type LogEntry struct {
	Message   string   `json:"message"`
	Formatted string   `json:"formatted"`
	Params    []string `json:"params"`
}

type ExceptionData struct {
	Values []ExceptionValue `json:"values"`
}

type ExceptionValue struct {
	Type       string      `json:"type"`
	Value      string      `json:"value"`
	Module     string      `json:"module"`
	ThreadID   any         `json:"thread_id"`
	Mechanism  *Mechanism  `json:"mechanism"`
	Stacktrace *Stacktrace `json:"stacktrace"`
}

type Mechanism struct {
	Type    string `json:"type"`
	Handled *bool  `json:"handled"`
}

type Stacktrace struct {
	Frames []Frame `json:"frames"`
}

type Frame struct {
	Filename    string `json:"filename"`
	Function    string `json:"function"`
	Module      string `json:"module"`
	Lineno      int    `json:"lineno"`
	Colno       int    `json:"colno"`
	AbsPath     string `json:"abs_path"`
	InApp       *bool  `json:"in_app"`
	ContextLine string `json:"context_line"`
}

// ParseEvent parses a raw JSON payload into a SentryEvent.
func ParseEvent(data []byte) (*SentryEvent, error) {
	var event SentryEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("parsing event: %w", err)
	}

	// Store raw data for full event storage
	var raw map[string]any
	json.Unmarshal(data, &raw)
	event.Raw = raw

	if event.Level == "" {
		event.Level = "error"
	}
	if event.Platform == "" {
		event.Platform = "other"
	}

	return &event, nil
}

// Title returns a human-readable title for the event.
func (e *SentryEvent) Title() string {
	if e.Exception != nil && len(e.Exception.Values) > 0 {
		last := e.Exception.Values[len(e.Exception.Values)-1]
		if last.Value != "" {
			return last.Type + ": " + last.Value
		}
		return last.Type
	}

	if e.Logentry != nil {
		if e.Logentry.Formatted != "" {
			return e.Logentry.Formatted
		}
		return e.Logentry.Message
	}

	if e.Message != "" {
		return e.Message
	}

	return "(no message)"
}

// IssueTitle returns the normalized issue title used for grouping.
func (e *SentryEvent) IssueTitle() string {
	if hint, ok := e.groupingHint(); ok && hint.Title != "" {
		return hint.Title
	}
	return e.Title()
}

// Culprit returns a concise location string like "POST /api/v2/bookings".
func (e *SentryEvent) Culprit() string {
	// Prefer request method + URL
	if e.Request != nil {
		method, _ := e.Request["method"].(string)
		url, _ := e.Request["url"].(string)
		if url != "" {
			// Strip scheme+host to keep just the path
			if i := strings.Index(url, "://"); i >= 0 {
				rest := url[i+3:]
				if j := strings.Index(rest, "/"); j >= 0 {
					url = rest[j:]
				}
			}
			if method != "" {
				return strings.ToUpper(method) + " " + url
			}
			return url
		}
	}
	if hint, ok := e.groupingHint(); ok && hint.Culprit != "" {
		return hint.Culprit
	}
	// Fall back to transaction (often the route)
	if e.Transaction != "" {
		return e.Transaction
	}
	return ""
}

// ComputeFingerprint generates a grouping hash for the event.
func (e *SentryEvent) ComputeFingerprint() string {
	// If SDK provides a custom fingerprint, use it directly
	if len(e.Fingerprint) > 0 && !(len(e.Fingerprint) == 1 && e.Fingerprint[0] == "{{ default }}") {
		hasher := sha256.New()
		for _, part := range e.Fingerprint {
			if part == "{{ default }}" {
				hasher.Write([]byte(e.defaultFingerprint()))
			} else {
				hasher.Write([]byte(part))
			}
		}
		return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
	}

	return e.defaultFingerprint()
}

func (e *SentryEvent) defaultFingerprint() string {
	hasher := sha256.New()

	// For exceptions: hash type + relevant frames
	if e.Exception != nil && len(e.Exception.Values) > 0 {
		for _, exc := range e.Exception.Values {
			hasher.Write([]byte(exc.Type))

			if exc.Stacktrace != nil && len(exc.Stacktrace.Frames) > 0 {
				// Check if throw point (innermost frame) is in_app
				innermost := exc.Stacktrace.Frames[len(exc.Stacktrace.Frames)-1]
				throwIsInApp := innermost.InApp != nil && *innermost.InApp

				if throwIsInApp {
					// Exception thrown in app code: group by in_app frames
					for _, frame := range exc.Stacktrace.Frames {
						if frame.InApp != nil && *frame.InApp {
							parts := []string{frame.Module, frame.Function, frame.Filename}
							hasher.Write([]byte(strings.Join(parts, "|")))
						}
					}
				} else {
					// Exception thrown in vendor/library: different app call paths
					// (middleware, controllers) lead to the same library bug.
					// Group by throw location only, ignoring caller variation.
					parts := []string{innermost.Module, innermost.Function, innermost.Filename}
					hasher.Write([]byte(strings.Join(parts, "|")))
				}
			}
		}
		return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
	}

	if hint, ok := e.groupingHint(); ok {
		hasher.Write([]byte(hint.FingerprintKey))
		return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
	}

	// For message events: hash the template message
	if e.Logentry != nil && e.Logentry.Message != "" {
		hasher.Write([]byte(e.Logentry.Message))
		return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
	}

	// Fallback: hash the message string
	if e.Message != "" {
		hasher.Write([]byte(e.Message))
		return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
	}

	// Last resort: hash event_id (each event is its own issue)
	hasher.Write([]byte(e.EventID))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
}

type groupingHint struct {
	FingerprintKey string
	Title          string
	Culprit        string
}

func (e *SentryEvent) groupingHint() (groupingHint, bool) {
	source := e.groupingMessage()
	if source == "" {
		return groupingHint{}, false
	}
	if hint, ok := parseExcessiveQueriesHint(source); ok {
		return hint, true
	}
	return groupingHint{}, false
}

func (e *SentryEvent) groupingMessage() string {
	if e.Logentry != nil {
		if e.Logentry.Formatted != "" {
			return e.Logentry.Formatted
		}
		if e.Logentry.Message != "" {
			return e.Logentry.Message
		}
	}
	return e.Message
}

func parseExcessiveQueriesHint(source string) (groupingHint, bool) {
	if !strings.Contains(source, "[ExcessiveQueries]") {
		return groupingHint{}, false
	}

	method, path := parseMethodAndPathFromMessage(source)
	if path == "" {
		return groupingHint{}, false
	}

	culprit := path
	key := "message|ExcessiveQueries|" + path
	if method != "" {
		culprit = method + " " + path
		key = "message|ExcessiveQueries|" + method + "|" + path
	}

	return groupingHint{
		FingerprintKey: key,
		Title:          "[ExcessiveQueries] " + culprit,
		Culprit:        culprit,
	}, true
}

func parseMethodAndPathFromMessage(source string) (string, string) {
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "URL:") {
			continue
		}

		rest := strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
		if rest == "" {
			continue
		}

		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}

		if len(fields) == 1 {
			return "", normalizeURLPath(fields[0])
		}

		method := strings.ToUpper(fields[0])
		path := normalizeURLPath(fields[1])
		if path != "" {
			return method, path
		}
	}

	firstLine, _, _ := strings.Cut(source, "\n")
	if idx := strings.LastIndex(firstLine, " — "); idx >= 0 {
		if path := normalizeURLPath(strings.TrimSpace(firstLine[idx+len(" — "):])); path != "" {
			return "", path
		}
	}
	if idx := strings.LastIndex(firstLine, " - "); idx >= 0 {
		if path := normalizeURLPath(strings.TrimSpace(firstLine[idx+len(" - "):])); path != "" {
			return "", path
		}
	}

	return "", ""
}

func normalizeURLPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if u, err := url.Parse(raw); err == nil {
		if u.Path != "" {
			return u.Path
		}
	}

	if idx := strings.IndexAny(raw, "?#"); idx >= 0 {
		raw = raw[:idx]
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}

	return ""
}
