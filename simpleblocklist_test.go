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

	// Write test IPs and networks to blacklist
	content := []byte(`# Test blacklist
192.0.2.1
203.0.113.2

# Network blocks
192.168.1.0/24
2001:db8::/32

# IPv6 addresses
2001:db8::1

# Empty lines and comments should be ignored

10.0.0.1  # With comment
`)
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
			desc:           "IP in blacklisted network range",
			xForwardedFor:  "192.168.1.100",
			blacklisted:    true,
			expectedStatus: 403,
		},
		{
			desc:           "IPv6 in blacklisted network range",
			xForwardedFor:  "2001:db8:0:1::1",
			blacklisted:    true,
			expectedStatus: 403,
		},
		{
			desc:           "Specific blacklisted IPv6",
			xForwardedFor:  "2001:db8::1",
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
			desc:           "Non-blacklisted IPv6",
			xForwardedFor:  "2001:db9::1",
			blacklisted:    false,
			expectedStatus: 200,
		},
		{
			desc:           "Local IP allowed",
			xForwardedFor:  "127.0.0.1",
			blacklisted:    false,
			expectedStatus: 200,
		},
		{
			desc:           "IP from RemoteAddr",
			remoteAddr:     "192.0.2.1:12345",
			blacklisted:    true,
			expectedStatus: 403,
		},
		{
			desc:           "IP with inline comment",
			xForwardedFor:  "10.0.0.1",
			blacklisted:    true,
			expectedStatus: 403,
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

			if test.remoteAddr != "" {
				req.RemoteAddr = test.remoteAddr
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

func TestSimpleBlocklist_InvalidBlacklistEntries(t *testing.T) {
	// Create a temporary blacklist file
	tmpfile, err := os.CreateTemp("", "blacklist")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test entries with some invalid ones
	content := []byte(`# Valid entries
192.0.2.1
192.168.1.0/24

# Invalid entries that should be ignored
invalid.ip.address
256.256.256.256
192.168.1.0/33
not-an-ip

# Valid entry after invalid ones
203.0.113.1
`)
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := simpleblocklist.CreateConfig()
	cfg.BlacklistPath = tmpfile.Name()

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	handler, err := simpleblocklist.New(ctx, next, cfg, "simpleblocklist")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc           string
		ip             string
		expectedStatus int
	}{
		{
			desc:           "Valid blacklisted IP",
			ip:             "192.0.2.1",
			expectedStatus: 403,
		},
		{
			desc:           "IP in valid CIDR range",
			ip:             "192.168.1.100",
			expectedStatus: 403,
		},
		{
			desc:           "Valid IP after invalid entries",
			ip:             "203.0.113.1",
			expectedStatus: 403,
		},
		{
			desc:           "Non-blacklisted IP",
			ip:             "192.0.2.2",
			expectedStatus: 200,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-Forwarded-For", test.ip)

			handler.ServeHTTP(recorder, req)

			if recorder.Code != test.expectedStatus {
				t.Errorf("got status code %d, want %d", recorder.Code, test.expectedStatus)
			}
		})
	}
}
