# Explicit submitter on the wire

**Status:** Proposed
**Tracking issue:** [livetemplate/livetemplate#237](https://github.com/livetemplate/livetemplate/issues/237) (P1)
**Related:** [#326](https://github.com/livetemplate/livetemplate/pull/326), [#327](https://github.com/livetemplate/livetemplate/pull/327)

## TL;DR

**Problem:** The server identifies which submit button triggered a form by *heuristic* — scanning form data for a single field whose value is the empty string and treating its name as the action. This works for the common pattern (`<button name="save">Save</button>` → browser submits `save=`) but is fragile under edge cases: ambiguity (multiple empty-value fields), collisions with deliberately-empty user input, and silent action loss if the heuristic misses.

**Solution:** The client already captures the native `SubmitEvent.submitter` (see `dom/event-delegation.ts`, search anchor `__lvtSubmitter`). Promote that data to a first-class wire field — a reserved `lvt-submitter` form field on the HTTP path and a `submitter` key on WS action payloads. The server prefers the explicit field; the heuristic stays as a one-version compatibility fallback.

**Status quo (after #326 / #327):**
- Client: captures `(e as SubmitEvent).submitter` and stores it on `__lvtSubmitter` (already shipping).
- Server: `internal/send/message.go` calls a single `detectSubmitButtonName(values, actionFields)` helper from both `parseMultipartForm` and `parseURLEncodedForm` (the helper landed in #327; it scans for a unique single-value-empty-string field).

The data exists on the client. The heuristic exists on the server. They just don't talk yet.

## The Problem

### What "button name as action" means today

The standard HTML pattern is:

```html
<form>
  <input name="title">
  <button name="save">Save</button>
  <button name="draft">Save as Draft</button>
</form>
```

When the user clicks **Save as Draft**, browsers submit:

```
title=My+Post&draft=
```

— i.e., the *clicked* button's name shows up with an empty value, and the *other* button is omitted entirely. LiveTemplate uses this to route the action: the server reads `draft=` and treats `draft` as the action name.

### How detection works today

`detectSubmitButtonName(values, actionFields)` (server, internal/send/message.go) scans every key:

1. Skip the empty key, the literal name `action`, and any name in `actionFields`.
2. Find fields where `len(values) == 1 && values[0] == ""`.
3. If exactly one such field exists, its name is the action.
4. If zero or two-or-more such fields exist, the heuristic returns `""` (ambiguous).

The two callers pass *different* `actionFields` skiplists:

- `parseURLEncodedForm`: `{lvt-action: true}` — only the explicit override is excluded.
- `parseMultipartForm`: `{lvt-action: true, data: true}` — also excludes the JSON envelope field used by the client library.

This means a `<input name="data" value="">` would be treated as a button-name candidate on the URL-encoded path but excluded on the multipart path. Phase 1 of this proposal makes the skiplist symmetric by adding `lvt-submitter` to both, and Phase 4 removes the heuristic entirely so the asymmetry stops mattering.

### Where the heuristic breaks down

1. **Ambiguous forms are silently dropped.** Two empty-value fields → no action routed. The user clicks a button, the server defaults to `submit`, and the developer has to reverse-engineer why.
2. **User-supplied empty inputs collide with button names.** A `<input name="search" value="">` (legitimately empty) is indistinguishable on the wire from a button that submitted with empty value. Today the heuristic excludes this collision class only by chance — if the form has *one* empty input and *no* clicked button, the input is misrouted as the action.
3. **`<button name="action">` can't route through the heuristic.** `detectSubmitButtonName` deliberately excludes the literal key `"action"` from the empty-value scan (the exclusion lives inside the helper itself at `key == "action"`, not via the `actionFields` skiplist) to dodge an ambiguity with the HTML `<form action="...">` attribute. Browsers don't submit `<form action>`, but if a button's `name` happens to also be `action`, an empty `action=` in form data could be either case. This is a documented workaround that disappears automatically when the heuristic is removed in Phase 4 — the entire helper goes away, so no separate cleanup is needed.
4. **No way to express "no button name, just submit".** A form whose buttons have no `name` attribute at all submits with no empty-value field, which is fine — but a form whose buttons happen to have names is locked into the heuristic.

These are all real-world friction points, not theoretical. Issue #237 is marked P1 because progressive-enhancement (server-only POST) flows are exactly where they bite hardest — there's no client-side capture to fall back on, and the server has nothing better than the heuristic.

## Current Client Capability (Already Shipping)

The client *already* knows which button was clicked. The `SubmitEvent` API gives it directly:

```ts
// dom/event-delegation.ts (search: __lvtSubmitter)
const submitter = (e as SubmitEvent).submitter;
if (submitter instanceof HTMLButtonElement && submitter.name) {
  action = submitter.name;
}
// ... payload assembly stashes submitter on the action message
```

For Tier 2 (WS-driven) form submits, the client extracts the action from `submitter.name` directly and serializes it as the action of the WS message. The empty-value heuristic doesn't run for this path because the client controls the wire format.

For Tier 1 (progressive enhancement, native HTTP POST), the browser owns the wire format. The client can't intercept, so the empty-value heuristic on the server is the only signal — and that's where the failure modes above live.

## Proposed Wire Format

### HTTP path

When the client submits a form via fetch (and via the server's HTTP fallback for no-JS users, with a small client shim), include a reserved hidden field:

```
title=My+Post&draft=&lvt-submitter=draft
```

The `lvt-submitter` key carries the explicit submitter name. The empty-value `draft=` is left as-is so progressive-enhancement falls back to the heuristic when an old client (no shim) submits to a new server.

For "lite-JS" submits (browser has JS but the full client hasn't loaded yet, or a small subset of the client is in use), the developer can opt in to a directive that injects the field before submit:

```html
<form lvt-form:emit-submitter>
  ...
</form>
```

The `lvt-form:emit-submitter` attribute wires a **`submit` event listener** (not a click handler) that reads `(e as SubmitEvent).submitter?.name` and writes it into a hidden `<input name="lvt-submitter">` before the browser serializes the form. The `submit` event is the right hook because it also fires for keyboard-triggered submits (Enter in a text field selects the form's default submit button as `submitter`, populating the field correctly), whereas a click handler would miss those entirely. **This still requires JS to run** — it doesn't help truly no-JS users.

For genuinely no-JS forms (form-only HTML, no script tags loaded), the only way to send `lvt-submitter` is to render the value server-side per button using a tiny inline `onclick` (which is HTML-attribute scripting, not script-tag scripting, so it works even when JS is selectively disabled but the page allows event-attribute handlers):

> **CSP caveat:** any app that ships a `Content-Security-Policy` header without `'unsafe-inline'` (the recommended posture) silently drops `onclick` handlers — the snippet below has zero effect under a strict CSP. Apps that need a strict CSP *and* the no-JS path *and* the explicit-submitter contract have no good option short of keeping the heuristic. This combination is the strongest argument for the no-JS support decision in Q3 below; option (a) or (c) — keeping the heuristic available — is the only path that serves it.

```html
<form action="/post" method="POST">
  <input type="hidden" name="lvt-submitter" value="">
  <button name="save"  onclick="this.form['lvt-submitter'].value='save'">Save</button>
  <button name="draft" onclick="this.form['lvt-submitter'].value='draft'">Save as Draft</button>
</form>
```

Or, more practically: the heuristic stays as the fallback for no-JS forms (it covers the common case correctly today; the failure modes above mostly bite when JS is in the loop). Phase 4's removal of the heuristic is conditional on whether no-JS support is a goal — flagged as an open question below.

Apps that don't opt in keep getting heuristic behavior — same as today.

### WebSocket path

The client already serializes a structured action message:

```json
{
  "action": "draft",
  "data": { "title": "My Post" }
}
```

Promote `submitter` to a top-level optional field on `ActionMessage`:

```json
{
  "action": "draft",
  "submitter": "draft",
  "data": { "title": "My Post" }
}
```

This requires extending the `ActionMessage` struct in `internal/send/message.go`:

```go
type ActionMessage struct {
    Action    string                 `json:"action"`
    Submitter string                 `json:"submitter,omitempty"` // NEW
    Data      map[string]interface{} `json:"data"`
}
```

A shared `resolveSubmitterFallback(msg *ActionMessage)` helper (proposed in Phase 1, see Migration Plan) applies the fallback for **all** content types: `if msg.Action == "" && msg.Submitter != "" { msg.Action = msg.Submitter }`. `ParseActionFromWebSocket`, `ParseActionFromHTTP`'s JSON branch, `parseURLEncodedForm`, and `parseMultipartForm` all call it. When `action` is set explicitly (via `lvt-form:action`), `submitter` is informational. When `action` is empty and `submitter` is set, the server uses `submitter` as the action — which is what the current client already does inline; this just moves the resolution server-side and removes the duplicated logic.

**Note on the WS fallback firing:** in normal operation, the client populates `action` from `submitter.name` *before* sending, so the server-side `if msg.Action == "" { ... }` fallback rarely runs in practice — it's belt-and-suspenders that lets the server stay correct if a future client refactor stops doing the inline action computation, or if a third-party WS client implementation only knows to send `submitter`. The two views of `submitter` in this section ("server uses it as action" + "client pre-populates so it rarely fires") are intentionally redundant: the visible signal stays in `msg.Action` for current operation, while `msg.Submitter` is diagnostic context that protects future implementations from silently regressing.

**Important:** the WS field is `submitter` (top-level, structured), and the HTTP field is `lvt-submitter` (form key, prefixed for skiplist symmetry with `lvt-action`). These are intentionally different because they live in different wire formats — JSON vs URL-encoded — and follow each format's existing naming conventions. The Phase 2 client implementation must be careful to use the right name on each path.

**Form `name` attribute preservation.** The current client also supports a fallback where `<form name="search">` (with no clicked button name) routes to a `Search` action. That fallback is implemented client-side in `dom/event-delegation.ts` (search anchor `Action resolution order`) — the client computes the action *before* sending, so the WS `action` field is already populated. This proposal does not change that behavior; the server still receives a fully-resolved `action` for the `form.name` case. The `submitter` field is only consulted when the client deliberately leaves `action` empty.

### Server resolution order

`ParseActionFromHTTP` is a content-type dispatcher; the resolution lives in its callees. `parseURLEncodedForm` and `parseMultipartForm` (both in `internal/send/message.go`) gain a new step between the existing `lvt-action` read and the heuristic call:

```
1. r.FormValue("lvt-action")          // explicit progressive-enhancement override (existing)
2. r.FormValue("lvt-submitter")       // NEW — explicit client-emitted submitter
3. detectSubmitButtonName(...)        // existing heuristic, now a fallback
4. ""                                  // server defaults to "submit"
```

In `parseMultipartForm` specifically, both `lvt-action` and `lvt-submitter` must be read **before** the `jsonDataParsed` branch (the same control-flow position the existing `lvt-action` read uses). This way they take precedence over the heuristic regardless of whether a JSON `data` envelope is present.

The WS path layers the same way but inside `ParseActionFromWebSocket`: explicit `action` field → explicit `submitter` field → empty. `ParseActionFromWebSocket` calls `resolveSubmitterFallback(&msg)` after `Unmarshal` (see Phase 1), keeping the resolution logic co-located with parsing rather than scattering it across `mount.go`'s `applyDefaultAction` call sites. After `ParseActionFromWebSocket` returns, `mount.go` continues to call `applyDefaultAction` (search anchor `applyDefaultAction` in `action.go` and `mount.go`) which ultimately defaults an empty action to `"submit"` — that step is unchanged. The WS path doesn't need a heuristic at all because the `action` and `submitter` fields are first-class on the wire — the client either populates them or the server returns `Submit`.

## Migration Plan

This is a backwards-compatible protocol extension. No client or server is required to opt in.

### Phase 1 — Server accepts `lvt-submitter`

- Add `Submitter string \`json:"submitter,omitempty"\`` to `ActionMessage` in `internal/send/message.go`. The struct decoder automatically populates this field for any JSON-marshalled message — but populating the field isn't the same as *resolving* it; see the helper below.
- Extract a shared helper for the `Submitter → Action` fallback so all three content-type paths use the same resolution:
   ```go
   // resolveSubmitterFallback fills msg.Action from msg.Submitter when Action
   // is empty, so an explicit `submitter` field on the wire is treated as the
   // action of last resort. Idempotent; safe to call from any parser.
   func resolveSubmitterFallback(msg *ActionMessage) {
       if msg.Action == "" && msg.Submitter != "" {
           msg.Action = msg.Submitter
       }
   }
   ```
   Call it from **all four** parser call sites:
   1. `ParseActionFromHTTP`'s default JSON branch (after `Decode` returns), so JSON-content-type form submissions get the fallback too.
   2. `ParseActionFromWebSocket` (after `Unmarshal`).
   3. `parseURLEncodedForm`, after reading `lvt-submitter` into `msg.Submitter` (see next bullet).
   4. `parseMultipartForm`, after reading `lvt-submitter` into `msg.Submitter` (see next bullet).
- Add `lvt-submitter` to the `actionFields` skiplist in `parseURLEncodedForm` and `parseMultipartForm` (so it's not echoed back into `msg.Data`).
- Read `lvt-submitter` into `msg.Submitter` after `lvt-action` and before the heuristic, then call `resolveSubmitterFallback(&msg)`. The helper's internal `if msg.Action == ""` guard ensures an explicit `lvt-action` is never overwritten.
- In `parseMultipartForm`, place the `lvt-submitter` read at the same control-flow position as `lvt-action` (before the `jsonDataParsed` branch) so it takes precedence over the heuristic regardless of whether a JSON `data` envelope is present.
- Add an "Action resolution order" doc comment to `parseMultipartForm` mirroring the one already in `parseURLEncodedForm`. Both functions will share the same 4-step order after Phase 1; documenting it in only one place is asymmetric.
- Add tests in `internal/send/message_test.go` mirroring the existing button-name-as-action cases plus collision cases (empty input + explicit submitter, two empty inputs + explicit submitter, `lvt-action` set + `lvt-submitter` set — all must respect the documented precedence). Cover all three content types.
- Server release: minor bump.

### Phase 2 — Client emits `lvt-submitter` (HTTP) and `submitter` (WS)

- In `dom/event-delegation.ts`, on form submit, read `(e as SubmitEvent).submitter?.name` and:
  - **HTTP path** (form submitted via `fetch`): inject as form key `lvt-submitter`.
  - **WS path** (lvt-driven submit): inject as top-level JSON key `submitter` on the action message.
- Continue capturing the existing inline `action = submitter.name` for the WS path; the structured `submitter` field is additional belt-and-suspenders that survives a server-side rewrite.
- Add a `lvt-form:emit-submitter` opt-in directive for lite-JS forms. It installs a `submit` event listener — *not* a click handler, see "HTTP path" above for the keyboard-submit reasoning — that writes the hidden `<input name="lvt-submitter">` from `(e as SubmitEvent).submitter?.name` before the browser serializes the form. The listener implementation must **create the hidden input on first run if it doesn't already exist** (and update its `value` on each subsequent submit), so apps don't have to remember to render the input alongside the directive. Pure no-JS forms cannot use this and stay on the heuristic.
- Client release: minor bump. Old client + new server: heuristic still runs (existing behavior). New client + old server: extra `lvt-submitter` / `submitter` fields are ignored by the old server's parsers — no breakage, the heuristic still wins. Both sides up-to-date: explicit submitter wins.

### Phase 3 — Deprecate the heuristic

After two minor versions of overlap (so the npm distribution has settled), mark the heuristic deprecated. Continue accepting it but log a warning when:

- the heuristic resolves to a name AND `lvt-submitter` was absent from the request.

This makes the silent-ambiguity case loud: developers see "your form submitted via the heuristic; consider upgrading the client or adding `lvt-form:emit-submitter`."

**Rate limiting.** Apps running mixed client versions during the migration window will hit this on every form submission, which can be noisy in production logs. The warning should be:

- Emitted at most once per process start per (URL path, action name) tuple — long enough to surface the issue, short enough to avoid spam.
- Structured (zap/slog fields, not free-form `printf`) so log aggregators can group / silence it without losing other warnings on the same logger.
- Suppressible via `LVT_FORM_SUBMITTER_COMPAT=silent` (see Q4 below).

### Phase 4 — Remove the heuristic (target: v0.9.0)

`detectSubmitButtonName` and the call sites disappear. The migration target is proposed to coincide with the `lvt-no-intercept` shim removal scheduled for v0.9.0 in the client (see `client/dom/link-interceptor.ts` shim and `client/CHANGELOG.md` "Migration: Phase 1A breaking changes" — the shim's removal there is committed; this proposal asks to land the heuristic removal in the *same* server release for breaking-change consolidation).

This phase is **conditional on the no-JS support decision** (Q3 below). If pure no-JS forms remain a supported audience, the heuristic must stay in some form — possibly behind a feature flag rather than removed outright. The Phase 4 milestone should not land until that decision is made.

## Risks and Open Questions

1. **Reserved-field name conflicts.** `lvt-submitter` becomes a reserved form field name. If an app already uses that name for user data, the upgrade will route it as a submitter. Mitigation: this is the same shape as the existing `lvt-action` reservation; document in CHANGELOG with the rest of the Phase 1A migration.

2. **`lvt-form:emit-submitter` opt-in vs. opt-out.** The directive is proposed as opt-in. Argument for keeping opt-in: injecting a hidden field by default into every form submission is a wire-format change that affects all existing apps on upgrade; opt-in keeps the upgrade silent. **Recommendation: opt-in.**

3. **No-JS support.** This is the load-bearing decision for Phase 4. If progressive-enhancement (server-only POST, no JS at all) is a first-class audience, the heuristic must stay in some form — `lvt-form:emit-submitter` and `lvt-submitter` injection both require JS to populate the field. Three sub-options:
    - **(a)** Keep the heuristic permanently as the no-JS fallback; never remove it. Phase 4 just adds a soft warning and stops shipping no-JS-incompatible features.
    - **(b)** Drop no-JS support entirely; remove the heuristic in v0.9.0; document in release notes that LiveTemplate now requires JS for form submission.
    - **(c)** Keep the heuristic behind an opt-in option (`WithProgressiveEnhancement(true)`); default to off in v0.9.0; apps that need no-JS opt back in.
    - **Decision needed before Phase 4 work begins.** Recommendation: (c) — splits the difference; the heuristic stays in the codebase but isn't on by default, so most apps get the cleaner contract while no-JS apps can still opt back in. Apps with a strict Content-Security-Policy (no `'unsafe-inline'`) can't use the inline-`onclick` no-JS workaround at all (see CSP caveat in "HTTP path" above), making (a)/(c) the only viable choices for that audience.
    - **Phase 4 gating:** the decision must be recorded **before any Phase 4 PR opens** by adding a short ADR at `docs/proposals/adr-no-js-support.md` and linking it from this proposal's `Status` block. Single source of truth — no scattered records. "Decide in the implementation PR" is the failure mode to avoid.

4. **Deprecation-log suppression mechanism.** The Phase 3 warning fires when the heuristic resolves *and* `lvt-submitter` was absent. Two options for letting users suppress it:
    - **(a)** New `WithFormSubmitterMode("strict"|"compat"|"silent")` option threaded through the parsing layer. Permanent API surface.
    - **(b)** `LVT_FORM_SUBMITTER_COMPAT=silent` environment variable — same observability, no Go API surface to maintain post-removal.
    - **Recommendation: (b)** — the warning is a transitional aid; an env var avoids permanent API debt for transitional behavior. The rate-limiting cache (one warning per `(URL path, action name)` tuple per process) should use a `sync.Map` rather than a plain `map` + mutex to avoid contention on the request hot path. Use `LoadOrStore` (atomic store-if-absent) rather than separate `Load` + `Store` calls — the two-step pattern has a TOCTOU gap where two concurrent requests both miss the read and both emit the warning. Pattern: `if _, loaded := seen.LoadOrStore(key, struct{}{}); !loaded { /* emit warning */ }`.

5. **Naming.** `lvt-submitter` parallels `lvt-action` and `lvt-form:*`. Alternatives: `lvt-form:submitter`, `lvt-clicked`, `_submitter`. The proposal uses `lvt-submitter` for symmetry with `lvt-action`, which has the same precedence shape (explicit override of resolution order). On the WS path the field is just `submitter` because JSON keys don't need a `lvt-` namespace prefix — they're already inside a LiveTemplate-owned message envelope.

6. **Multipart form bodies need separate handling.** The existing helper handles both URL-encoded and multipart, so adding `lvt-submitter` to the skiplist works for both — but file-upload payloads can be large enough that the heuristic was meaningful as a "the action is whichever button you clicked, no extra fields" pattern. Worth verifying the file-upload examples still feel ergonomic with the explicit submitter approach. Note that read overhead for the explicit-submitter approach on multipart is zero: `r.FormValue("lvt-submitter")` reads from the already-parsed `r.MultipartForm.Value` map, which `r.ParseMultipartForm` populated earlier in the request handler.

## Out of Scope

- **Multi-button race conditions on the *same* form.** The current heuristic and the proposed explicit submitter both assume one click → one submitter. Forms that programmatically dispatch synthetic `SubmitEvent`s without a real click are out of scope; they should set `lvt-form:action` directly.
- **Input-type-image submitters.** `<input type="image">` submits coordinates (`name.x`, `name.y`) instead of an empty value. Current heuristic doesn't handle this either. Adding `lvt-submitter` does — coordinate inputs become orthogonal.

## Verification

When this proposal is implemented:

1. New tests in `internal/send/message_test.go` covering `lvt-submitter` as the routing source, including the collision scenarios that the heuristic gets wrong today (multiple empty fields, user-supplied empty inputs, `<button name="action">`).
2. New WS-path tests in the same file covering `ActionMessage.Submitter` resolution: `action="" + submitter="X"` → action becomes `X`; `action="Y" + submitter="X"` → action stays `Y`.
3. **Phase 3 deprecation-warning tests:** unit tests around the `sync.Map` / `LoadOrStore` rate-limiting cache. Concurrently invoke the heuristic-resolved code path from N goroutines under `-race` and assert the warning fires *exactly once* per `(path, action)` tuple regardless of concurrency. Add a test that an explicit `lvt-submitter` suppresses the warning entirely. Add a test that `LVT_FORM_SUBMITTER_COMPAT=silent` suppresses the warning even in heuristic-only requests.
4. New e2e test (or extend an existing form submission e2e in the lvt repo at `e2e/livetemplate_core_test.go`) verifying the explicit submitter path round-trips through both the HTTP and WS paths.
5. Manual smoke on the patterns examples (login, blog, etc.) confirming no regression in the heuristic-driven path.
6. Cross-repo verification: bump client to a version that emits `lvt-submitter`, then confirm the lvt examples still work end-to-end.

## Appendix: References

- Heuristic implementation: `internal/send/message.go`, search anchor `detectSubmitButtonName`.
- Client capture: `dom/event-delegation.ts` (client repo), search anchor `__lvtSubmitter`.
- Action resolution doc: `internal/send/message.go`, search anchor `Action resolution order`.
- Existing `actionFields` skiplist: `internal/send/message.go`, search anchor `actionFields := map[string]bool`.
- Recent helper extraction (#327): commit `86a33c67`, file `internal/send/message.go`.
