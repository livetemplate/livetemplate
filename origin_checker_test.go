package livetemplate

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestCreateSecureOriginChecker(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		devMode        bool
		origin         string
		host           string
		tls            bool
		forwardedProto string
		want           bool
	}{
		// DevMode tests
		{
			name:    "dev mode allows any origin",
			devMode: true,
			origin:  "https://evil.com",
			host:    "localhost:8080",
			want:    true,
		},

		// No origin header
		{
			name:   "no origin header is allowed (same-origin request)",
			origin: "",
			host:   "example.com",
			want:   true,
		},

		// AllowedOrigins tests
		{
			name:           "allowed origin matches",
			allowedOrigins: []string{"https://example.com", "https://other.com"},
			origin:         "https://example.com",
			host:           "example.com",
			want:           true,
		},
		{
			name:           "origin not in allowed list is rejected",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://evil.com",
			host:           "example.com",
			want:           false,
		},

		// AllowedOrigins takes priority over X-Forwarded-Proto
		{
			name:           "X-Forwarded-Proto ignored when allowedOrigins is set",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "http", // would cause mismatch in same-origin path, but allowedOrigins takes priority
			want:           true,
		},

		// Same-origin: direct TLS connection
		{
			name:   "direct HTTPS connection with matching origin",
			origin: "https://example.com",
			host:   "example.com",
			tls:    true,
			want:   true,
		},
		{
			name:   "direct HTTPS connection with mismatched origin",
			origin: "https://evil.com",
			host:   "example.com",
			tls:    true,
			want:   false,
		},

		// Same-origin: direct non-TLS connection
		{
			name:   "direct HTTP connection with matching origin",
			origin: "http://example.com",
			host:   "example.com",
			tls:    false,
			want:   true,
		},
		{
			name:   "direct HTTP connection with mismatched origin",
			origin: "http://evil.com",
			host:   "example.com",
			tls:    false,
			want:   false,
		},

		// Same-origin: behind reverse proxy with X-Forwarded-Proto
		{
			name:           "reverse proxy with X-Forwarded-Proto https",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false, // TLS terminated at proxy
			forwardedProto: "https",
			want:           true,
		},
		{
			name:           "reverse proxy with X-Forwarded-Proto http",
			origin:         "http://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "http",
			want:           true,
		},
		{
			name:           "reverse proxy with X-Forwarded-Proto mismatch rejected",
			origin:         "http://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "https",
			want:           false,
		},

		// Case insensitivity of X-Forwarded-Proto
		{
			name:           "X-Forwarded-Proto uppercase HTTPS",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "HTTPS",
			want:           true,
		},
		{
			name:           "X-Forwarded-Proto mixed case Https",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "Https",
			want:           true,
		},

		// Multi-valued X-Forwarded-Proto (chained proxies)
		{
			name:           "X-Forwarded-Proto multi-value takes first",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "https, http",
			want:           true,
		},
		{
			name:           "X-Forwarded-Proto multi-value with spaces",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "  https , http",
			want:           true,
		},

		// Invalid X-Forwarded-Proto falls back to r.TLS
		{
			name:           "X-Forwarded-Proto invalid value falls back to r.TLS nil",
			origin:         "http://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "ftp",
			want:           true, // falls back to http (r.TLS == nil)
		},
		{
			name:           "X-Forwarded-Proto invalid value falls back to r.TLS set",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            true,
			forwardedProto: "ftp",
			want:           true, // falls back to https (r.TLS != nil)
		},

		// Empty host
		{
			name:   "empty host is rejected",
			origin: "https://example.com",
			host:   "",
			want:   false,
		},

		// Host with port
		{
			name:   "host with port matches origin with port",
			origin: "https://example.com:8443",
			host:   "example.com:8443",
			tls:    true,
			want:   true,
		},
		{
			name:   "host with port mismatch rejected",
			origin: "https://example.com:8443",
			host:   "example.com:9443",
			tls:    true,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := createSecureOriginChecker(tt.allowedOrigins, tt.devMode, true)

			req, err := http.NewRequest("GET", "/ws", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			got := checker(req)
			if got != tt.want {
				t.Errorf("createSecureOriginChecker(%v, %v)(origin=%q, host=%q, tls=%v, proto=%q) = %v, want %v",
					tt.allowedOrigins, tt.devMode, tt.origin, tt.host, tt.tls, tt.forwardedProto, got, tt.want)
			}
		})
	}
}

func TestCreateSecureOriginChecker_UntrustedForwardedHeaders(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		host           string
		tls            bool
		forwardedProto string
		want           bool
	}{
		{
			name:           "X-Forwarded-Proto ignored when untrusted",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "https",
			want:           false, // Without proxy trust, TLS=nil means HTTP, but origin says HTTPS
		},
		{
			name:           "falls back to r.TLS when untrusted",
			origin:         "http://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "https",
			want:           true, // r.TLS=nil → HTTP scheme, origin says HTTP → match
		},
		{
			name:   "direct HTTPS works without proxy",
			origin: "https://example.com",
			host:   "example.com",
			tls:    true,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := createSecureOriginChecker(nil, false, false)

			req, err := http.NewRequest("GET", "/ws", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			got := checker(req)
			if got != tt.want {
				t.Errorf("trust=false (origin=%q, host=%q, tls=%v, proto=%q) = %v, want %v",
					tt.origin, tt.host, tt.tls, tt.forwardedProto, got, tt.want)
			}
		})
	}
}
