package main

import "testing"

func TestBuildAllowedOrigins(t *testing.T) {
	t.Parallel()

	origins := buildAllowedOrigins("http://localhost:8080", []string{"https://app.example.com", "invalid"})

	for _, origin := range []string{
		"http://localhost:8080",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"https://app.example.com",
	} {
		if _, ok := origins[origin]; !ok {
			t.Fatalf("expected allowed origin %q", origin)
		}
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	t.Parallel()

	allowed := buildAllowedOrigins("https://gosnag.example.com", nil)

	if !isAllowedOrigin("https://gosnag.example.com", "", allowed) {
		t.Fatal("expected base origin to be allowed")
	}

	if isAllowedOrigin("https://evil.example.com", "", allowed) {
		t.Fatal("did not expect arbitrary origin to be allowed")
	}
}
