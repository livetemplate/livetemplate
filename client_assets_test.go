package livetemplate_test

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate"
)

// TestClientVersionIsSemver guards against a malformed pin (e.g. a stray "v"
// prefix or "latest"), which would produce a broken CDN URL.
func TestClientVersionIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(livetemplate.ClientVersion) {
		t.Fatalf("ClientVersion %q is not a bare semver (want MAJOR.MINOR.PATCH, no leading v)", livetemplate.ClientVersion)
	}
}

// TestClientURLsArePinned is the core regression guard for B2: the client URLs
// must be HTTPS, carry the exact pinned ClientVersion, and never fall back to
// the unpinned "@latest" tag (the wire-incompat footgun this constant replaces).
func TestClientURLsArePinned(t *testing.T) {
	for name, url := range map[string]string{
		"ClientScriptURL": livetemplate.ClientScriptURL,
		"ClientStyleURL":  livetemplate.ClientStyleURL,
	} {
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("%s = %q: must be HTTPS", name, url)
		}
		if strings.Contains(url, "@latest") {
			t.Errorf("%s = %q: must not use the unpinned @latest tag", name, url)
		}
		if !strings.Contains(url, "@"+livetemplate.ClientVersion+"/") {
			t.Errorf("%s = %q: must pin @%s", name, url, livetemplate.ClientVersion)
		}
	}
}

// TestClientURLsPointAtBundleFiles locks the specific artifact paths so a typo
// (e.g. dropping the dist/ prefix, or pointing CSS at dist/) is caught here
// rather than as a 404 in a browser.
func TestClientURLsPointAtBundleFiles(t *testing.T) {
	if !strings.HasSuffix(livetemplate.ClientScriptURL, "/dist/livetemplate-client.browser.js") {
		t.Errorf("ClientScriptURL = %q: want suffix /dist/livetemplate-client.browser.js", livetemplate.ClientScriptURL)
	}
	// The stylesheet lives at the package root, not under dist/.
	if !strings.HasSuffix(livetemplate.ClientStyleURL, "/livetemplate.css") {
		t.Errorf("ClientStyleURL = %q: want suffix /livetemplate.css", livetemplate.ClientStyleURL)
	}
	if strings.Contains(livetemplate.ClientStyleURL, "/dist/") {
		t.Errorf("ClientStyleURL = %q: stylesheet is at the package root, not under dist/", livetemplate.ClientStyleURL)
	}
}

// TestClientURLTemplateFuncsRender is the end-to-end proof of the consumption
// path: a full-HTML document can reference lvtClientScriptURL / lvtClientStyleURL
// with no per-app State field or Funcs registration, and they render the pinned
// URLs. It renders through a Clone because production renders through per-session
// clones — a master-only render would pass even if Clone dropped the seeded funcs.
func TestClientURLTemplateFuncsRender(t *testing.T) {
	const doc = `<!DOCTYPE html><html><head>` +
		`<link rel="stylesheet" href="{{lvtClientStyleURL}}">` +
		`</head><body><script src="{{lvtClientScriptURL}}" defer></script></body></html>`

	master, err := livetemplate.New("client-funcs-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Parse must succeed: html/template rejects a template referencing an
	// undefined func, so this fails loudly if the funcs were not seeded.
	if _, err := master.Parse(doc); err != nil {
		t.Fatalf("parse referencing framework funcs: %v", err)
	}
	clone, err := master.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	var buf bytes.Buffer
	if err := clone.Execute(&buf, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, livetemplate.ClientScriptURL) {
		t.Errorf("rendered HTML missing ClientScriptURL %q:\n%s", livetemplate.ClientScriptURL, html)
	}
	if !strings.Contains(html, livetemplate.ClientStyleURL) {
		t.Errorf("rendered HTML missing ClientStyleURL %q:\n%s", livetemplate.ClientStyleURL, html)
	}
}

// TestClientURLTemplateFuncsAreOverridable locks the behavior documented on
// (*Template).Funcs and in client_assets.go: because Funcs merges into the same
// FuncMap by name, a user-registered func with the same name overrides the
// framework-seeded one. A self-hoster who wants same-origin URLs uses this to
// point lvtClientScriptURL at their own path without writing a new tag.
func TestClientURLTemplateFuncsAreOverridable(t *testing.T) {
	const doc = `<script src="{{lvtClientScriptURL}}"></script>`
	const selfHosted = "/assets/livetemplate-client.js"

	master, err := livetemplate.New("client-funcs-override-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Register the override before parsing, so the parse-time FuncMap already
	// carries the user value rather than the framework default.
	master.Funcs(template.FuncMap{
		"lvtClientScriptURL": func() string { return selfHosted },
	})
	if _, err := master.Parse(doc); err != nil {
		t.Fatalf("parse with overridden func: %v", err)
	}
	clone, err := master.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	var buf bytes.Buffer
	if err := clone.Execute(&buf, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, selfHosted) {
		t.Errorf("override did not win: want %q in rendered HTML:\n%s", selfHosted, html)
	}
	if strings.Contains(html, livetemplate.ClientScriptURL) {
		t.Errorf("framework default leaked through despite override: %q in:\n%s", livetemplate.ClientScriptURL, html)
	}
}
