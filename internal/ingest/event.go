package ingest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// SentryEvent represents the JSON structure sent by Sentry SDKs.
type SentryEvent struct {
	EventID     string                 `json:"event_id"`
	Timestamp   any                    `json:"timestamp"` // string or float
	Platform    string                 `json:"platform"`
	Level       string                 `json:"level"`
	Logger      string                 `json:"logger"`
	Transaction string                 `json:"transaction"`
	ServerName  string                 `json:"server_name"`
	Release     string                 `json:"release"`
	Dist        string                 `json:"dist"`
	Environment string                 `json:"environment"`
	Message     string                 `json:"message"`
	Logentry    *LogEntry              `json:"logentry"`
	Exception   *ExceptionData         `json:"exception"`
	Tags        map[string]string      `json:"tags"`
	Extra       map[string]any         `json:"extra"`
	Fingerprint []string               `json:"fingerprint"`
	User        map[string]any         `json:"user"`
	Request     map[string]any         `json:"request"`
	Contexts    map[string]any         `json:"contexts"`
	Breadcrumbs map[string]any         `json:"breadcrumbs"`
	SDK         map[string]any         `json:"sdk"`
	Modules     map[string]string      `json:"modules"`
	Raw         map[string]any         `json:"-"` // full raw event for storage
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

	// For exceptions: hash type + in-app frames
	if e.Exception != nil && len(e.Exception.Values) > 0 {
		for _, exc := range e.Exception.Values {
			hasher.Write([]byte(exc.Type))

			if exc.Stacktrace != nil {
				for _, frame := range exc.Stacktrace.Frames {
					if frame.InApp != nil && !*frame.InApp {
						continue
					}
					parts := []string{frame.Module, frame.Function, frame.Filename}
					hasher.Write([]byte(strings.Join(parts, "|")))
				}
			}
		}
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
