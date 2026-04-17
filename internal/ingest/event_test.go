package ingest

import (
	"testing"

	"github.com/darkspock/gosnag/internal/database/db"
)

func boolPtr(v bool) *bool { return &v }

func TestFingerprintVendorException(t *testing.T) {
	// Same vendor exception reached via different app call paths should produce the same fingerprint
	eventA := &SentryEvent{
		Exception: &ExceptionData{Values: []ExceptionValue{{
			Type: "UnexpectedValueException",
			Stacktrace: &Stacktrace{Frames: []Frame{
				{Filename: "/public/index.php", InApp: boolPtr(true)},
				{Filename: "/vendor/monolog/StreamHandler.php", Function: "write", InApp: boolPtr(false)},
			}},
		}}},
	}

	eventB := &SentryEvent{
		Exception: &ExceptionData{Values: []ExceptionValue{{
			Type: "UnexpectedValueException",
			Stacktrace: &Stacktrace{Frames: []Frame{
				{Filename: "/public/index.php", InApp: boolPtr(true)},
				{Filename: "/app/Middleware/Auth.php", Function: "handle", InApp: boolPtr(true)},
				{Filename: "/app/Controllers/FooController.php", Function: "index", InApp: boolPtr(true)},
				{Filename: "/vendor/monolog/StreamHandler.php", Function: "write", InApp: boolPtr(false)},
			}},
		}}},
	}

	fpA := eventA.ComputeFingerprint()
	fpB := eventB.ComputeFingerprint()

	if fpA != fpB {
		t.Fatalf("vendor exception with different callers should have same fingerprint, got %s vs %s", fpA, fpB)
	}
}

func TestFingerprintAppException(t *testing.T) {
	// Same exception thrown from different in_app locations should produce different fingerprints
	eventA := &SentryEvent{
		Exception: &ExceptionData{Values: []ExceptionValue{{
			Type: "RuntimeException",
			Stacktrace: &Stacktrace{Frames: []Frame{
				{Filename: "/app/ServiceA.php", Function: "doWork", InApp: boolPtr(true)},
			}},
		}}},
	}

	eventB := &SentryEvent{
		Exception: &ExceptionData{Values: []ExceptionValue{{
			Type: "RuntimeException",
			Stacktrace: &Stacktrace{Frames: []Frame{
				{Filename: "/app/ServiceB.php", Function: "doOtherWork", InApp: boolPtr(true)},
			}},
		}}},
	}

	fpA := eventA.ComputeFingerprint()
	fpB := eventB.ComputeFingerprint()

	if fpA == fpB {
		t.Fatalf("different in_app throw locations should have different fingerprints")
	}
}

func TestFingerprintCustom(t *testing.T) {
	event := &SentryEvent{
		Fingerprint: []string{"my-custom-group"},
		Message:     "some message",
	}

	fp := event.ComputeFingerprint()
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}

	// Same custom fingerprint should produce same hash
	event2 := &SentryEvent{
		Fingerprint: []string{"my-custom-group"},
		Message:     "different message",
	}
	if event.ComputeFingerprint() != event2.ComputeFingerprint() {
		t.Fatal("same custom fingerprint should produce same hash")
	}
}

func TestFingerprintMessageFallback(t *testing.T) {
	eventA := &SentryEvent{Message: "connection timeout"}
	eventB := &SentryEvent{Message: "connection timeout"}
	eventC := &SentryEvent{Message: "disk full"}

	if eventA.ComputeFingerprint() != eventB.ComputeFingerprint() {
		t.Fatal("same message should produce same fingerprint")
	}
	if eventA.ComputeFingerprint() == eventC.ComputeFingerprint() {
		t.Fatal("different messages should produce different fingerprints")
	}
}

func TestURLGroupingHintUsesRequestURL(t *testing.T) {
	eventA := &SentryEvent{
		Request: map[string]any{
			"method": "post",
			"url":    "https://www.covermanager.com/api2/GoogleMaps/v3/CreateBooking?foo=bar",
		},
	}

	hint, ok := eventA.URLGroupingHint()
	if !ok {
		t.Fatal("expected URL grouping hint")
	}

	if got, want := hint.Culprit, "POST /api2/GoogleMaps/v3/CreateBooking"; got != want {
		t.Fatalf("unexpected culprit: got %q want %q", got, want)
	}

	if got, want := hint.Title, "POST /api2/GoogleMaps/v3/CreateBooking"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
}

func TestURLGroupingHintFallsBackToMessageURL(t *testing.T) {
	event := &SentryEvent{
		Message: "Error: [SlowRequest]\nURL: POST http://www.covermanager.com/api2/GoogleMaps/v3/CreateBooking\nBody: {\"merchant_id\":\"5453\"}",
	}

	hint, ok := event.URLGroupingHint()
	if !ok {
		t.Fatal("expected URL grouping hint from message")
	}

	if got, want := hint.FingerprintKey, "info:url|POST|/api2/GoogleMaps/v3/CreateBooking"; got != want {
		t.Fatalf("unexpected fingerprint key: got %q want %q", got, want)
	}
}

func TestURLGroupingHintNormalizesFrameworkParams(t *testing.T) {
	event := &SentryEvent{
		Request: map[string]any{
			"method": "get",
			"url":    "http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026",
		},
	}

	hint, ok := event.URLGroupingHint()
	if !ok {
		t.Fatal("expected URL grouping hint")
	}

	if got, want := hint.Culprit, "GET /coverApp/Reserv/getCalendar/:int/:year"; got != want {
		t.Fatalf("unexpected culprit: got %q want %q", got, want)
	}
}

func TestResolveIssueGroupingByURLForErrorWithoutException(t *testing.T) {
	event := &SentryEvent{
		Level:   "error",
		Message: "Error: [ExcessiveQueries]\nURL: GET http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026",
		Request: map[string]any{
			"method": "get",
			"url":    "http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026",
		},
	}

	fingerprint, title, culprit := resolveIssueGrouping(db.Project{}.ID, event, issueSettings{ErrorGroupingMode: "by_url"}, nil)

	if fingerprint != hashFingerprintKey("info:url|GET|/coverApp/Reserv/getCalendar/:int/:year") {
		t.Fatalf("unexpected fingerprint: %q", fingerprint)
	}
	if title != "Error: [ExcessiveQueries]\nURL: GET http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026" {
		t.Fatalf("unexpected title: %q", title)
	}
	if culprit != "GET /coverApp/Reserv/getCalendar/:int/:year" {
		t.Fatalf("unexpected culprit: %q", culprit)
	}
}

func TestResolveIssueGroupingByURLForWarningUsesEffectiveErrorMode(t *testing.T) {
	event := &SentryEvent{
		Level:   "warning",
		Message: "Warning: [SlowRequest]\nURL: GET http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026",
		Request: map[string]any{
			"method": "get",
			"url":    "http://www.covermanager.com/coverApp/Reserv/getCalendar/4/2026",
		},
	}

	fingerprint, _, culprit := resolveIssueGrouping(db.Project{}.ID, event, issueSettings{
		WarningAsError:    true,
		ErrorGroupingMode: "by_url",
	}, nil)

	if fingerprint != hashFingerprintKey("info:url|GET|/coverApp/Reserv/getCalendar/:int/:year") {
		t.Fatalf("unexpected fingerprint: %q", fingerprint)
	}
	if culprit != "GET /coverApp/Reserv/getCalendar/:int/:year" {
		t.Fatalf("unexpected culprit: %q", culprit)
	}
}

func TestFileGroupingHintUsesInAppFrame(t *testing.T) {
	event := &SentryEvent{
		Exception: &ExceptionData{Values: []ExceptionValue{{
			Type: "RuntimeException",
			Stacktrace: &Stacktrace{Frames: []Frame{
				{Filename: "/vendor/framework/Core.php", Function: "run", InApp: boolPtr(false)},
				{Filename: "/application/controllers/Booking.php", Function: "create", InApp: boolPtr(true)},
			}},
		}}},
	}

	hint, ok := event.FileGroupingHint()
	if !ok {
		t.Fatal("expected file grouping hint")
	}

	if got, want := hint.Culprit, "/application/controllers/Booking.php"; got != want {
		t.Fatalf("unexpected culprit: got %q want %q", got, want)
	}

	if got, want := hint.Title, "RuntimeException: Booking.php"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
}
