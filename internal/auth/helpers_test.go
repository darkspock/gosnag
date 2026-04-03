package auth

import (
	"net/http"
	"testing"
)

func TestShouldSecureCookie(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	tests := []struct {
		name    string
		baseURL string
		header  string
		want    bool
	}{
		{name: "plain http request", baseURL: "http://example.com", want: false},
		{name: "https base url", baseURL: "https://example.com", want: true},
		{name: "forwarded proto https", baseURL: "http://example.com", header: "https", want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reqCopy := req.Clone(req.Context())
			if tt.header != "" {
				reqCopy.Header.Set("X-Forwarded-Proto", tt.header)
			}

			if got := shouldSecureCookie(reqCopy, tt.baseURL); got != tt.want {
				t.Fatalf("shouldSecureCookie(..., %q) = %t, want %t", tt.baseURL, got, tt.want)
			}
		})
	}
}
