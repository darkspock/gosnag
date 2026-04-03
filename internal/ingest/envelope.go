package ingest

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type EnvelopeHeader struct {
	EventID string `json:"event_id"`
	DSN     string `json:"dsn"`
	SentAt  string `json:"sent_at"`
}

type ItemHeader struct {
	Type        string `json:"type"`
	Length      int    `json:"length"`
	ContentType string `json:"content_type"`
}

type EnvelopeItem struct {
	Header  ItemHeader
	Payload []byte
}

// ParseEnvelope parses a Sentry envelope from a request body.
// Format: envelope_header\n(item_header\npayload\n)*
// Handles the `length` field in item headers for payloads with embedded newlines.
func ParseEnvelope(r *http.Request) (*EnvelopeHeader, []EnvelopeItem, error) {
	body, err := readBody(r)
	if err != nil {
		return nil, nil, fmt.Errorf("reading body: %w", err)
	}

	// Find first newline for envelope header
	idx := bytes.IndexByte(body, '\n')
	if idx < 0 {
		return nil, nil, fmt.Errorf("empty envelope")
	}

	var header EnvelopeHeader
	if err := json.Unmarshal(body[:idx], &header); err != nil {
		return nil, nil, fmt.Errorf("parsing envelope header: %w", err)
	}

	pos := idx + 1
	var items []EnvelopeItem

	for pos < len(body) {
		// Skip empty lines
		if body[pos] == '\n' {
			pos++
			continue
		}

		// Read item header (until next newline)
		end := bytes.IndexByte(body[pos:], '\n')
		if end < 0 {
			break
		}

		var itemHeader ItemHeader
		if err := json.Unmarshal(body[pos:pos+end], &itemHeader); err != nil {
			pos += end + 1
			continue
		}
		pos += end + 1

		// Read payload
		var payload []byte
		if itemHeader.Length > 0 {
			// Read exactly Length bytes (handles embedded newlines)
			if pos+itemHeader.Length <= len(body) {
				payload = body[pos : pos+itemHeader.Length]
				pos += itemHeader.Length
				// Skip trailing newline if present
				if pos < len(body) && body[pos] == '\n' {
					pos++
				}
			} else {
				payload = body[pos:]
				pos = len(body)
			}
		} else {
			// No length specified: read until next newline
			end = bytes.IndexByte(body[pos:], '\n')
			if end >= 0 {
				payload = body[pos : pos+end]
				pos += end + 1
			} else {
				payload = body[pos:]
				pos = len(body)
			}
		}

		if len(payload) > 0 {
			items = append(items, EnvelopeItem{
				Header:  itemHeader,
				Payload: payload,
			})
		}
	}

	return &header, items, nil
}

// readBody handles decompression of the request body.
func readBody(r *http.Request) ([]byte, error) {
	var reader io.Reader = r.Body
	defer r.Body.Close()

	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	case "deflate":
		zr, err := zlib.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		reader = zr
	}

	return io.ReadAll(io.LimitReader(reader, 1024*1024)) // 1MB limit
}
