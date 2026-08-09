package livetemplate

import "html/template"

// ClientVersion is the version of the @livetemplate/client browser bundle that
// this release of livetemplate is wire-compatible with.
//
// It moves in lockstep with each livetemplate release, so applications never
// hand-maintain a client version: upgrading the livetemplate dependency
// (go get -u github.com/livetemplate/livetemplate) automatically selects the
// matching client. Pinning to this version — rather than the CDN's "@latest"
// tag — prevents a client-only release from silently shipping a wire-protocol
// change to browsers that are still talking to an older server. There is no
// runtime version handshake between server and client, so an unpinned client is
// unsafe: see the @livetemplate/client CHANGELOG (e.g. the __navigate__ action,
// which is a no-op on older servers).
//
// Maintainers: bump this in the same release that adopts a new client version.
const ClientVersion = "0.20.0"

// clientCDNBase is the jsdelivr base URL for the pinned client bundle. jsdelivr
// serves the published npm package @livetemplate/client.
const clientCDNBase = "https://cdn.jsdelivr.net/npm/@livetemplate/client@" + ClientVersion

// ClientScriptURL is the pinned CDN URL for the browser client bundle, for use
// as the src of a <script> tag:
//
//	<script src="{{ .ClientScriptURL }}" defer></script>
//
// To self-host (offline, air-gapped, or CSP-strict deployments), vendor
// @livetemplate/client@<ClientVersion> and serve it from your own origin
// instead of using this URL.
const ClientScriptURL = clientCDNBase + "/dist/livetemplate-client.browser.js"

// ClientStyleURL is the pinned CDN URL for the client stylesheet, for use as the
// href of a <link rel="stylesheet"> tag. See ClientScriptURL for the
// self-hosting note.
const ClientStyleURL = clientCDNBase + "/livetemplate.css"

// frameworkTemplateFuncs are the template functions livetemplate provides to
// every template automatically (seeded into the FuncMap in New, before parsing,
// so a full-HTML document can reference the pinned client bundle with no per-app
// wiring — no State field, no manual Funcs registration):
//
//	<script src="{{ lvtClientScriptURL }}" defer></script>
//	<link rel="stylesheet" href="{{ lvtClientStyleURL }}">
//
// They return the ClientScriptURL / ClientStyleURL constants, so the pinned
// version stays single-sourced in Go. Self-hosters simply write their own tag
// instead of calling these.
func frameworkTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"lvtClientScriptURL": func() string { return ClientScriptURL },
		"lvtClientStyleURL":  func() string { return ClientStyleURL },
	}
}
