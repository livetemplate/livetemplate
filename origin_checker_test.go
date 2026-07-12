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
		forwarded      string
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

		// RFC 7239 Forwarded header (fallback when X-Forwarded-Proto absent)
		{
			name:      "Forwarded header proto https",
			origin:    "https://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "for=192.0.2.60;proto=https;by=203.0.113.43",
			want:      true,
		},
		{
			name:      "Forwarded header proto http",
			origin:    "http://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "for=192.0.2.60;proto=http",
			want:      true,
		},
		{
			name:      "Forwarded header proto mismatch rejected",
			origin:    "http://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "proto=https",
			want:      false,
		},
		{
			name:      "Forwarded header quoted proto value",
			origin:    "https://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: `for=192.0.2.60;proto="https"`,
			want:      true,
		},
		{
			name:      "Forwarded header case-insensitive param name",
			origin:    "https://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "Proto=https",
			want:      true,
		},
		{
			name:      "Forwarded header multi-element uses first hop",
			origin:    "https://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "proto=https, proto=http",
			want:      true,
		},
		{
			name:      "Forwarded header without proto param falls back to r.TLS",
			origin:    "http://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "for=192.0.2.60;by=203.0.113.43",
			want:      true, // no proto → r.TLS nil → http
		},
		{
			name:           "X-Forwarded-Proto wins over Forwarded when both present",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "https",
			forwarded:      "proto=http", // conflicting; X-Forwarded-Proto takes precedence
			want:           true,
		},
		{
			name:           "invalid X-Forwarded-Proto falls through to Forwarded",
			origin:         "https://example.com",
			host:           "example.com",
			tls:            false,
			forwardedProto: "ftp",
			forwarded:      "proto=https",
			want:           true,
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
			if tt.forwarded != "" {
				req.Header.Set("Forwarded", tt.forwarded)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			got := checker(req)
			if got != tt.want {
				t.Errorf("createSecureOriginChecker(%v, %v)(origin=%q, host=%q, tls=%v, proto=%q, forwarded=%q) = %v, want %v",
					tt.allowedOrigins, tt.devMode, tt.origin, tt.host, tt.tls, tt.forwardedProto, tt.forwarded, got, tt.want)
			}
		})
	}
}

// installedCheckOrigin returns the CheckOrigin function New() wired onto the
// default gorilla upgrader, so these tests exercise the real construction path
// (option -> New() finalization -> upgrader) rather than calling
// createSecureOriginChecker directly like TestCreateSecureOriginChecker does.
func installedCheckOrigin(t *testing.T, opts ...Option) func(*http.Request) bool {
	t.Helper()
	tmpl, err := New("app", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	gu, ok := tmpl.config.Upgrader.(*GorillaUpgrader)
	if !ok {
		t.Fatalf("default upgrader is %T, want *GorillaUpgrader", tmpl.config.Upgrader)
	}
	if gu.inner.CheckOrigin == nil {
		t.Fatal("New() left CheckOrigin nil; expected an origin checker to be installed")
	}
	return gu.inner.CheckOrigin
}

// crossOriginRequest is an Origin that never matches the localhost host — it is
// allowed only when the origin check is fully relaxed.
func crossOriginRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest("GET", "/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "https://evil.com")
	return req
}

// TestWithDevModeWiresPermissiveOrigin locks the fact that WithDevMode(true)
// ALONE relaxes WebSocket origin checking to allow all origins — so apps do not
// need to also pass WithPermissiveOriginCheck() in dev. The negative case is the
// gate: without dev mode, the same cross-origin request is rejected, proving the
// installed checker is a real same-origin check and not a no-op that always
// passes (which would make the positive assertion meaningless).
func TestWithDevModeWiresPermissiveOrigin(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want bool
	}{
		{
			name: "dev mode alone allows cross-origin",
			opts: []Option{WithDevMode(true)},
			want: true,
		},
		{
			name: "no options rejects cross-origin",
			opts: nil,
			want: false,
		},
		{
			name: "permissive origin check alone allows cross-origin",
			opts: []Option{WithPermissiveOriginCheck()},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := installedCheckOrigin(t, tt.opts...)
			if got := checker(crossOriginRequest(t)); got != tt.want {
				t.Errorf("cross-origin allowed = %v, want %v", got, tt.want)
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
		forwarded      string
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
			name:      "Forwarded header ignored when untrusted",
			origin:    "https://example.com",
			host:      "example.com",
			tls:       false,
			forwarded: "proto=https",
			want:      false, // Untrusted: forged Forwarded must not upgrade scheme to https
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
			if tt.forwarded != "" {
				req.Header.Set("Forwarded", tt.forwarded)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			got := checker(req)
			if got != tt.want {
				t.Errorf("trust=false (origin=%q, host=%q, tls=%v, proto=%q, forwarded=%q) = %v, want %v",
					tt.origin, tt.host, tt.tls, tt.forwardedProto, tt.forwarded, got, tt.want)
			}
		})
	}
}
