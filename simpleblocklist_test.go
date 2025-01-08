package simpleblocklist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/LucaNori/traefik-simpleblocklist"
)

func TestSimpleBlocklist(t *testing.T) {
	// Create a temporary blacklist file
	tmpfile, err := os.CreateTemp("", "blacklist")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test IPs to blacklist
	content := []byte("192.0.2.1\n203.0.113.2\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		blacklisted    bool
		expectedStatus int
	}{
		{
			desc:           "Blacklisted IP in X-Forwarded-For",
			xForwardedFor:  "192.0.2.1",
			blacklisted:    true,
			expectedStatus: 403,
		},
		{
			desc:           "Blacklisted IP in X-Real-IP",
			xRealIP:        "203.0.113.2",
			blacklisted:    true,
			expectedStatus: 403,
		},
		{
			desc:           "Non-blacklisted IP",
			xForwardedFor:  "192.0.2.100",
			blacklisted:    false,
			expectedStatus: 200,
		},
		{
			desc:           "Local IP allowed",
			xForwardedFor:  "127.0.0.1",
			blacklisted:    false,
			expectedStatus: 200,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := simpleblocklist.CreateConfig()
			cfg.BlacklistPath = tmpfile.Name()
			cfg.AllowLocalRequests = true

			ctx := context.Background()
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusOK)
			})

			handler, err := simpleblocklist.New(ctx, next, cfg, "simpleblocklist")
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatal(err)
			}

			if test.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", test.xForwardedFor)
			}
			if test.xRealIP != "" {
				req.Header.Set("X-Real-IP", test.xRealIP)
			}

			handler.ServeHTTP(recorder, req)

			if recorder.Code != test.expectedStatus {
				t.Errorf("got status code %d, want %d", recorder.Code, test.expectedStatus)
			}
		})
	}
}

func TestSimpleBlocklist_NoBlacklistFile(t *testing.T) {
	cfg := simpleblocklist.CreateConfig()
	cfg.BlacklistPath = "nonexistent.txt"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := simpleblocklist.New(ctx, next, cfg, "simpleblocklist")
	if err == nil {
		t.Error("expected error when blacklist file doesn't exist")
	}
}

func TestSimpleBlocklist_CustomStatusCode(t *testing.T) {
	// Create a temporary blacklist file
	tmpfile, err := os.CreateTemp("", "blacklist")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test IP to blacklist
	content := []byte("192.0.2.1\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := simpleblocklist.CreateConfig()
	cfg.BlacklistPath = tmpfile.Name()
	cfg.HTTPStatusCodeDeniedRequest = 429 // Too Many Requests

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := simpleblocklist.New(ctx, next, cfg, "simpleblocklist")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "192.0.2.1")

	handler.ServeHTTP(recorder, req)

	if recorder.Code != 429 {
		t.Errorf("got status code %d, want 429", recorder.Code)
	}
}
