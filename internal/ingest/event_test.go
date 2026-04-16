package ingest

import "testing"

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

func TestFingerprintExcessiveQueriesGroupsByRoute(t *testing.T) {
	eventA := &SentryEvent{
		Message: "Error: [ExcessiveQueries] 128 queries (126.5ms total) — /api2/GoogleMaps/v3/CreateBooking\n" +
			"Server: CoverPHP83-API\n" +
			"URL: POST http://www.covermanager.com/api2/GoogleMaps/v3/CreateBooking\n" +
			`Body: {"slot":{"resources":{"party_size":4}},"user_information":{"email":"rosasafont@hotmail.com"},"idempotency_token":"14859062036507846112"}`,
	}

	eventB := &SentryEvent{
		Message: "Error: [ExcessiveQueries] 185 queries (661.71ms total) — /api2/GoogleMaps/v3/CreateBooking\n" +
			"Server: CoverPHP83-API\n" +
			"URL: POST http://www.covermanager.com/api2/GoogleMaps/v3/CreateBooking\n" +
			`Body: {"slot":{"resources":{"party_size":2}},"user_information":{"email":"jose.taracena@hotmail.com"},"idempotency_token":"12452082239147641745"}`,
	}

	if eventA.ComputeFingerprint() != eventB.ComputeFingerprint() {
		t.Fatal("same ExcessiveQueries endpoint should produce the same fingerprint")
	}

	if got, want := eventA.IssueTitle(), "[ExcessiveQueries] POST /api2/GoogleMaps/v3/CreateBooking"; got != want {
		t.Fatalf("unexpected issue title: got %q want %q", got, want)
	}

	if got, want := eventA.Culprit(), "POST /api2/GoogleMaps/v3/CreateBooking"; got != want {
		t.Fatalf("unexpected culprit: got %q want %q", got, want)
	}
}

func TestFingerprintExcessiveQueriesSeparatesRoutes(t *testing.T) {
	eventA := &SentryEvent{
		Message: "Error: [ExcessiveQueries] 128 queries (126.5ms total) — /api2/GoogleMaps/v3/CreateBooking\n" +
			"URL: POST http://www.covermanager.com/api2/GoogleMaps/v3/CreateBooking",
	}

	eventB := &SentryEvent{
		Message: "Error: [ExcessiveQueries] 130 queries (140ms total) — /api2/GoogleMaps/v3/CancelBooking\n" +
			"URL: POST http://www.covermanager.com/api2/GoogleMaps/v3/CancelBooking",
	}

	if eventA.ComputeFingerprint() == eventB.ComputeFingerprint() {
		t.Fatal("different ExcessiveQueries endpoints should produce different fingerprints")
	}
}
