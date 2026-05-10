# Explicit submitter on the wire

**Status:** Accepted (Phase 1 shipped in [#396](https://github.com/livetemplate/livetemplate/pull/396); Phase 2 client work is the active follow-up)
**Tracking issue:** [livetemplate/livetemplate#237](https://github.com/livetemplate/livetemplate/issues/237) (P1)
**Related:** [#326](https://github.com/livetemplate/livetemplate/pull/326), [#327](https://github.com/livetemplate/livetemplate/pull/327), [#396](https://github.com/livetemplate/livetemplate/pull/396)

## TL;DR

**Problem:** The server identifies which submit button triggered a form by *heuristic* — scanning form data for a single field whose value is the empty string and treating its name as the action. This works for the common pattern (`<button name="save">Save</button>` → browser submits `save=`) but is fragile under edge cases: ambiguity (multiple empty-value fields), collisions with deliberately-empty user input, and silent action loss if the heuristic misses.

**Solution:** The client already captures the native `SubmitEvent.submitter` (see `dom/event-delegation.ts`, search anchor `__lvtSubmitter`). Promote that data to a first-class wire field — a reserved `lvt-submitter` form field on the HTTP path and a `submitter` key on WS action payloads. The server prefers the explicit field; the heuristic stays as the **permanent no-JS fallback** for apps that cannot run JS at form-submit time (no script tags loaded, strict CSP without `'unsafe-inline'`).

**Status quo (after #326 / #327 / #396):**
- Client: captures `(e as SubmitEvent).submitter` and stores it on `__lvtSubmitter` (already shipping). Does **not** yet promote it to a first-class wire field — that is Phase 2 below.
- Server: accepts both `submitter` (JSON) and `lvt-submitter` (form) as first-class fields and prefers them over the heuristic. Helper `resolveSubmitterFallback` (search anchor in `internal/send/message.go`) is called from all four parsers. Heuristic `detectSubmitButtonName` (search anchor) remains as the no-JS fallback.

The data exists on the client. The server already accepts the explicit field. They just need the client to actually emit it (Phase 2).

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

`detectSubmitButtonName(values, actionFields)` (server, search anchor in `internal/send/message.go`) scans every key:

1. Skip the empty key, the literal name `action`, and any name in `actionFields`.
2. Find fields where `len(values) == 1 && values[0] == ""`.
3. If exactly one such field exists, its name is the action.
4. If zero or two-or-more such fields exist, the heuristic returns `""` (ambiguous).

The two callers pass *different* `actionFields` skiplists:

- `parseURLEncodedForm`: `{lvt-action: true, lvt-submitter: true}` — explicit overrides.
- `parseMultipartForm`: `{lvt-action: true, lvt-submitter: true, data: true}` — also excludes the JSON envelope field used by the client library.

This means a `<input name="data" value="">` would be treated as a button-name candidate on the URL-encoded path but excluded on the multipart path. The asymmetry is intentional and load-bearing only for the multipart JSON-envelope case; the URL-encoded path doesn't carry a JSON envelope, so excluding `data` there would silently break legitimate user inputs named `data`.

### Where the heuristic breaks down

1. **Ambiguous forms are silently dropped.** Two empty-value fields → no action routed. The user clicks a button, the server defaults to `submit`, and the developer has to reverse-engineer why.
2. **User-supplied empty inputs collide with button names.** A `<input name="search" value="">` (legitimately empty) is indistinguishable on the wire from a button that submitted with empty value. Today the heuristic excludes this collision class only by chance — if the form has *one* empty input and *no* clicked button, the input is misrouted as the action.
3. **`<button name="action">` can't route through the heuristic.** `detectSubmitButtonName` deliberately excludes the literal key `"action"` from the empty-value scan to dodge an ambiguity with the HTML `<form action="...">` attribute. Browsers don't submit `<form action>`, but if a button's `name` happens to also be `action`, an empty `action=` in form data could be either case. Apps that have JS available avoid this edge entirely once they emit `lvt-submitter`/`submitter`.
4. **No way to express "no button name, just submit".** A form whose buttons have no `name` attribute at all submits with no empty-value field, which is fine — but a form whose buttons happen to have names is locked into the heuristic.

These are all real-world friction points, not theoretical. Issue #237 is marked P1 because progressive-enhancement (server-only POST) flows are exactly where they bite hardest. Phase 2 eliminates them for the JS-enabled audience; the no-JS path keeps the heuristic and accepts these limitations as documented (see "No-JS forms" below).

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

For Tier 1 (progressive enhancement, native HTTP POST), the browser owns the wire format. In pure no-JS mode the client can't change the wire format at all, so the empty-value heuristic on the server is the only signal — and that's where the failure modes above live permanently. With the `lvt-form:emit-submitter` directive (Phase 2), JS-enabled apps that submit forms natively (rather than via fetch) get the explicit-submitter contract via a `submit`-event listener that mutates the form before serialization.

## Proposed Wire Format

### HTTP path

When the client submits a form via fetch — or when JS-enabled apps with native HTML form submissions opt into the `lvt-form:emit-submitter` directive — include a reserved hidden field:

```
title=My+Post&draft=&lvt-submitter=draft
```

The `lvt-submitter` key carries the explicit submitter name. The empty-value `draft=` is left as-is so apps that have not opted into the directive (or run in strict-CSP no-JS mode) fall back to the heuristic.

**JSON HTTP path:** `application/json` HTTP requests are JSON-decoded directly into `ActionMessage` and so use the `submitter` key (same as WebSocket), populated via `ActionMessage.Submitter` — the `lvt-submitter` form field name applies only to `application/x-www-form-urlencoded` and `multipart/form-data` submissions. The same `resolveSubmitterFallback` helper is called after `Decode`.

For "lite-JS" submits (browser has JS but the form submits natively rather than via fetch), the developer can opt in to a directive that injects the field before submit:

```html
<form lvt-form:emit-submitter>
  ...
</form>
```

The `lvt-form:emit-submitter` attribute wires a **`submit` event listener** (not a click handler) that reads `(e as SubmitEvent).submitter?.name` and writes it into a hidden `<input name="lvt-submitter">` before the browser serializes the form. The `submit` event is the right hook because it also fires for keyboard-triggered submits (Enter in a text field selects the form's default submit button as `submitter`, populating the field correctly), whereas a click handler would miss those entirely. **This still requires JS to run** — it doesn't help truly no-JS users.

For apps that don't load the LiveTemplate client bundle (no `<script>` tags fetching it), the only way to emit `lvt-submitter` is per-button inline `onclick` — which is HTML-attribute scripting rather than `<script>`-tag scripting. **CSP caveat:** apps that ship a strict `Content-Security-Policy` header without `'unsafe-inline'` cause the browser to silently ignore `onclick` handlers, so this snippet has zero effect under a strict CSP — those apps fall back to the heuristic permanently.

```html
<form action="/post" method="POST">
  <input type="hidden" name="lvt-submitter" value="">
  <button name="save"  onclick="this.form['lvt-submitter'].value='save'">Save</button>
  <button name="draft" onclick="this.form['lvt-submitter'].value='draft'">Save as Draft</button>
</form>
```

Apps that don't opt in keep getting heuristic behavior — same as today.

### WebSocket path

The client already serializes a structured action message:

```json
{
  "action": "draft",
  "data": { "title": "My Post" }
}
```

`submitter` is a top-level optional field on `ActionMessage` (shipped in #396):

```json
{
  "action": "draft",
  "submitter": "draft",
  "data": { "title": "My Post" }
}
```

The `ActionMessage` struct in `internal/send/message.go` is:

```go
type ActionMessage struct {
    Action    string                 `json:"action"`
    Submitter string                 `json:"submitter,omitempty"`
    Data      map[string]interface{} `json:"data"`
}
```

A shared `resolveSubmitterFallback(msg *ActionMessage)` helper applies the fallback for **all** content types: `if msg.Action == "" && msg.Submitter != "" { msg.Action = msg.Submitter }`. `ParseActionFromWebSocket`, `ParseActionFromHTTP`'s JSON branch, `parseURLEncodedForm`, and `parseMultipartForm` all call it. When `action` is set explicitly (via `lvt-form:action`), `submitter` is informational. When `action` is empty and `submitter` is set, the server uses `submitter` as the action — which is what the current client already does inline; this just moves the resolution server-side and removes the duplicated logic.

**Note on the WS fallback firing:** in normal operation the client populates `action` from `submitter.name` *before* sending, so the server-side `if msg.Action == "" { ... }` fallback rarely runs in practice — it's belt-and-suspenders that lets the server stay correct if a future client refactor stops doing the inline action computation, or if a third-party WS client implementation only knows to send `submitter`. The two views of `submitter` here ("server uses it as action" + "client pre-populates so it rarely fires") are intentionally redundant: the visible signal stays in `msg.Action` for current operation, while `msg.Submitter` is diagnostic context that protects future implementations from silently regressing.

**Important:** the WS field is `submitter` (top-level, structured), and the HTTP field is `lvt-submitter` (form key, prefixed for skiplist symmetry with `lvt-action`). These are intentionally different because they live in different wire formats — JSON vs URL-encoded — and follow each format's existing naming conventions. The Phase 2 client implementation must use the right name on each path.

**Form `name` attribute preservation.** The current client also supports a fallback where `<form name="search">` (with no clicked button name) routes to a `Search` action. That fallback is implemented client-side in `dom/event-delegation.ts` (search anchor `Action resolution order`) — the client computes the action *before* sending, so the WS `action` field is already populated. This proposal does not change that behavior; the server still receives a fully-resolved `action` for the `form.name` case. The `submitter` field is only consulted when the client deliberately leaves `action` empty.

### Server resolution order

`ParseActionFromHTTP` is a content-type dispatcher; the resolution lives in its callees. `parseURLEncodedForm` and `parseMultipartForm` (both in `internal/send/message.go`) implement a 4-step order:

```
1. r.FormValue("lvt-action")          // explicit progressive-enhancement override
2. r.FormValue("lvt-submitter")       // explicit client-emitted submitter
3. detectSubmitButtonName(...)        // empty-value heuristic, permanent no-JS fallback
4. ""                                  // server defaults to "submit"
```

In `parseMultipartForm`, both `lvt-action` and `lvt-submitter` are read **before** the `jsonDataParsed` branch (the same control-flow position the existing `lvt-action` read uses) so they take precedence over the heuristic regardless of whether a JSON `data` envelope is present.

The WS path layers the same way but inside `ParseActionFromWebSocket`: explicit `action` field → explicit `submitter` field → empty. `ParseActionFromWebSocket` calls `resolveSubmitterFallback(&msg)` after `Unmarshal`, keeping the resolution logic co-located with parsing rather than scattering it across `mount.go`'s `applyDefaultAction` call sites. After `ParseActionFromWebSocket` returns, `mount.go` continues to call `applyDefaultAction` (search anchor `applyDefaultAction` in `action.go` and `mount.go`) which ultimately defaults an empty action to `"submit"` — that step is unchanged. The WS path doesn't need a heuristic at all because the `action` and `submitter` fields are first-class on the wire — the client either populates them or the server returns `Submit`.

## Implementation Plan

This is a backwards-compatible protocol extension. No client or server is required to opt in.

**Why two phases instead of four?** LiveTemplate is alpha with no external users at the time of writing, so the original four-phase migration's deprecation/compat scaffolding (Phase 3 rate-limited compat warning, Phase 4 ADR-gated heuristic removal) had no audience to protect. The phases collapse cleanly: server accepts the explicit field, client emits it, heuristic stays permanently for the no-JS audience. If LiveTemplate gains downstream consumers before Phase 2 ships, this section should be revisited.

### Phase 1 — Server accepts the explicit submitter (Complete, [#396](https://github.com/livetemplate/livetemplate/pull/396))

Shipped in PR #396. The server now accepts both `submitter` (JSON envelope) and `lvt-submitter` (form key) as first-class fields, with a shared `resolveSubmitterFallback` helper called from all four parsers (`ParseActionFromHTTP` JSON branch, `ParseActionFromWebSocket`, `parseURLEncodedForm`, `parseMultipartForm`). `lvt-submitter` is in the `actionFields` skiplist on both URL-encoded and multipart paths so it is not echoed back into `msg.Data`. `parseMultipartForm` reads `lvt-submitter` before the `jsonDataParsed` branch, mirroring the existing `lvt-action` control-flow position. The `Action resolution order` doc comment is now mirrored in both URL-encoded and multipart parsers. Tests in `internal/send/message_test.go` cover all three content types plus the collision cases (legitimate empty input + explicit submitter, two empty fields + explicit submitter, `<button name="action">` resolution, `lvt-action` always wins over `lvt-submitter`).

#### Implementation reference

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

Search anchor `resolveSubmitterFallback` in `internal/send/message.go`.

### Phase 2 — Client emits `lvt-submitter` (HTTP) and `submitter` (WS)

Active follow-up. Cross-repo in `livetemplate/client`.

- **WS path**: in `dom/event-delegation.ts`, after the existing `__lvtSubmitter` capture and the `data` map assembly, set `message.submitter = submitter.name` when a named submitter is present. Continue populating `action` inline from `submitter.name` as today; the new `submitter` field is additive belt-and-suspenders that survives future client refactors.
- **HTTP fetch path (Tier 1 multipart)**: in `dom/event-delegation.ts`, immediately after the existing `tier1FormData.set("lvt-action", action)`, set `tier1FormData.set("lvt-submitter", submitter.name)` only when the submitter has a name. Skip the set on empty (don't write empty-string noise — the server's `resolveSubmitterFallback` only fires when `Submitter != ""`).
- **HTTP fetch path (URL-encoded)**: nothing to do — the client never produces URL-encoded request bodies. `sendHTTP()` (in `livetemplate-client.ts`, search anchor) sends `application/json`, and `sendHTTPMultipart()` sends `multipart/form-data`. The server's URL-encoded parser is exercised only by native browser form submissions, which the `lvt-form:emit-submitter` directive (next bullet) covers by injecting the hidden input into the form before the browser serializes.
- **`lvt-form:emit-submitter` directive (lite-JS opt-in)**: in the existing event-delegated `submit` listener at the wrapper, when the target form has the attribute, look up or create a hidden `<input name="lvt-submitter">` child and write `(e as SubmitEvent).submitter?.name` into its value before the browser serializes. Use the `submit` event (not `click`) so keyboard-triggered submits work. **Use event delegation at the wrapper, not per-form `addEventListener`** — LiveTemplate swaps DOM nodes on update, and per-form listeners would not survive a re-render of the form's container. The listener implementation must **create the hidden input on first run if it doesn't already exist** (and update its `value` on each subsequent submit), so apps don't have to remember to render the input alongside the directive.
- **Tests** in `tests/event-delegation.test.ts` covering: WS message includes `submitter` when named; WS message omits `submitter` when no name; HTTP multipart sets `lvt-submitter`; `lvt-form:emit-submitter` directive populates the hidden input on click submit; same directive populates on keyboard submit (dispatch a SubmitEvent with `submitter` set to the form's default submit button).
- **Release**: minor bump per the client repo's release script. Old client + new server: heuristic still runs (current behavior). New client + old server: extra `lvt-submitter` / `submitter` fields are ignored by the old server's parsers — no breakage, the heuristic still wins. Both up-to-date: explicit submitter wins.

### No-JS forms

The empty-value heuristic stays as the permanent fallback for apps that cannot run JS at form-submit time:

- Pure no-JS forms (no `<script>` tags loaded at all).
- Strict-CSP apps without `'unsafe-inline'`, where the browser silently ignores `onclick` handlers, so the inline-`onclick` workaround above has no effect.
- Apps that render forms server-side and submit them natively before the lvt client's `submit` listener has been installed (rare but possible).

These apps inherit all four failure modes from "Where the heuristic breaks down" (ambiguous empty fields, user-supplied empty input collisions, `<button name="action">` shadowing, no way to express "no button name"). The cost of keeping the heuristic is one extra branch in the URL-encoded and multipart parsers; the benefit is keeping genuinely-no-JS apps as a first-class audience. Apps that have any JS at all can opt into `lvt-form:emit-submitter` to escape these limitations on natively-submitted forms.

## Risks and Open Questions

1. **Reserved-field name conflicts.** `lvt-submitter` is a reserved form field name. If an app uses that name for user data, the field will be routed as a submitter and stripped from `msg.Data`. Mitigation: this is the same shape as the existing `lvt-action` reservation; document in CHANGELOG when Phase 2 lands.

2. **`lvt-form:emit-submitter` opt-in vs. opt-out.** The directive is opt-in. Argument for opt-in: injecting a hidden field by default into every form submission is a wire-format change that affects all existing apps on upgrade; opt-in keeps the upgrade silent. **Decision: opt-in.**

3. **No-JS support.** The empty-value heuristic stays as the permanent no-JS fallback. Pure no-JS forms (no script tags loaded, strict CSP without `'unsafe-inline'`) cannot inject `lvt-submitter` from JS at all and rely on the heuristic. The cost is one extra branch in the URL-encoded and multipart parsers; the benefit is keeping genuinely-no-JS apps as a first-class audience. The four failure modes from "Where the heuristic breaks down" are accepted as documented limitations of the no-JS path.

4. **Naming.** `lvt-submitter` parallels `lvt-action` and `lvt-form:*`. Alternatives: `lvt-form:submitter`, `lvt-clicked`, `_submitter`. The proposal uses `lvt-submitter` for symmetry with `lvt-action`, which has the same precedence shape (explicit override of resolution order). On the WS path the field is just `submitter` because JSON keys don't need a `lvt-` namespace prefix — they're already inside a LiveTemplate-owned message envelope.

5. **Multipart form bodies need separate handling.** The existing helper handles both URL-encoded and multipart, so `lvt-submitter` in the skiplist works for both. Read overhead for the explicit-submitter approach on multipart is zero: `r.FormValue("lvt-submitter")` reads from the already-parsed `r.MultipartForm.Value` map, which `r.ParseMultipartForm` populated earlier in the request handler.

## Out of Scope

- **Multi-button race conditions on the *same* form.** The current heuristic and the explicit submitter both assume one click → one submitter. Forms that programmatically dispatch synthetic `SubmitEvent`s without a real click are out of scope; they should set `lvt-form:action` directly.
- **Input-type-image submitters.** `<input type="image">` submits coordinates (`name.x`, `name.y`) instead of an empty value. The heuristic doesn't handle this. Adding `lvt-submitter` does — coordinate inputs become orthogonal.
- **`<form name="search">` HTTP routing.** A form with a `name` attribute and no named buttons / no `lvt-submitter` field falls through to `applyDefaultAction`, which defaults the action to `"submit"`. The `form.name` → action mapping is a *client-side* convenience implemented in `dom/event-delegation.ts` and the server simply receives a fully-resolved `action` for the WS path. The HTTP no-JS path has never resolved actions from `form.name` and this proposal doesn't change that.
- **Re-entrant double-submit guards in `lvt-form:emit-submitter`.** If two rapid submits race (e.g., double-click before the first fetch returns), the listener fires twice. The second fire updates the hidden input's value, but the first request has already serialized the body with the value the listener wrote on the first fire — harmless, because each request carries whatever `submitter` its own click resolved (the same name for a same-button double-click, different names for a cross-button race). No mutual-exclusion guarantee is needed or provided. Implementations should *not* add mutex-style locks for this.
- **`submitter` collision inside the JSON `data` blob.** A client that accidentally puts a key named `submitter` inside the JSON `data` envelope (rather than at the message top level) does not get routing behavior — it surfaces as `msg.Data["submitter"]` and stays as user data. This mirrors how `action` inside `data` works today and is intentional: the routing-layer fields live at the envelope level, the data dictionary is a leaf. Test matrix should cover the `{lvt-submitter set on form} + {submitter set inside data blob}` case to confirm the form-level value wins for routing.

## Verification

When Phase 2 is implemented:

1. Tests in `internal/send/message_test.go` covering `lvt-submitter` as the routing source, including the collision scenarios that the heuristic gets wrong (multiple empty fields, user-supplied empty inputs, `<button name="action">`). **These already shipped in #396**; no new server tests required for Phase 2.
2. WS-path tests in the same file covering `ActionMessage.Submitter` resolution: `action="" + submitter="X"` → action becomes `X`; `action="Y" + submitter="X"` → action stays `Y`. **Already shipped in #396.**
3. New e2e test (or extend an existing form submission e2e in the lvt repo at `e2e/livetemplate_core_test.go`) verifying the explicit submitter path round-trips through both the HTTP and WS paths. Adding this is the responsibility of the Phase 2 implementation plan, not the proposal-revision PR. Tracked under [#237](https://github.com/livetemplate/livetemplate/issues/237) until the Phase 2 PR opens; that PR will cite this verification item explicitly so the cross-repo handoff doesn't drop the requirement.
4. Manual smoke on the patterns examples (login, blog, etc.) confirming no regression in the heuristic-driven path.
5. Cross-repo verification: bump client to a version that emits `lvt-submitter`, then confirm the lvt examples still work end-to-end.

## Appendix: References

- Heuristic implementation: `internal/send/message.go`, search anchor `detectSubmitButtonName`.
- Fallback helper: `internal/send/message.go`, search anchor `resolveSubmitterFallback`.
- Client capture: `dom/event-delegation.ts` (client repo), search anchor `__lvtSubmitter`.
- Action resolution doc: `internal/send/message.go`, search anchor `Action resolution order`.
- `actionFields` skiplist: `internal/send/message.go`, search anchor `actionFields := map[string]bool`.
- Phase 1 server-side merge: [PR #396](https://github.com/livetemplate/livetemplate/pull/396).
