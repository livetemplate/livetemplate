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

1. Skip `lvt-action`, `data`, `action`, and the empty key.
2. Find fields where `len(values) == 1 && values[0] == ""`.
3. If exactly one such field exists, its name is the action.
4. If zero or two-or-more such fields exist, the heuristic returns `""` (ambiguous).

### Where the heuristic breaks down

1. **Ambiguous forms are silently dropped.** Two empty-value fields → no action routed. The user clicks a button, the server defaults to `submit`, and the developer has to reverse-engineer why.
2. **User-supplied empty inputs collide with button names.** A `<input name="search" value="">` (legitimately empty) is indistinguishable on the wire from a button that submitted with empty value. Today the heuristic excludes this collision class only by chance — if the form has *one* empty input and *no* clicked button, the input is misrouted as the action.
3. **`<button name="action">` is reserved by accident.** The current code excludes `name="action"` to avoid a different ambiguity (browsers don't submit `<form action="...">`, so an empty `action=` field is treated as user data). This is a workaround for a heuristic, not a real reservation.
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

For pure no-JS form submits (no client at all), the developer can opt in by adding a hidden input populated by a small inline script:

```html
<form lvt-form:emit-submitter>
  ...
</form>
```

The `lvt-form:emit-submitter` attribute is a directive that wires a `<button>` click handler injecting the hidden field before submit. Apps that don't opt in keep getting heuristic behavior — same as today.

### WebSocket path

The client already serializes a structured action message:

```json
{
  "action": "draft",
  "data": { "title": "My Post" }
}
```

Promote `submitter` to a top-level optional field:

```json
{
  "action": "draft",
  "submitter": "draft",
  "data": { "title": "My Post" }
}
```

When `action` is set explicitly (via `lvt-form:action`), `submitter` is informational. When `action` is empty and `submitter` is set, the server uses `submitter` as the action — which is what the current client already does inline; this just moves the resolution server-side and removes the duplicated logic.

### Server resolution order

`internal/send/message.go::ParseActionFromHTTP` becomes:

```
1. r.FormValue("lvt-action")          // explicit progressive-enhancement override (existing)
2. r.FormValue("lvt-submitter")       // NEW — explicit client-emitted submitter
3. detectSubmitButtonName(...)        // existing heuristic, now a fallback
4. ""                                  // server defaults to "submit"
```

Identical layering on the WS path: explicit `action` field → explicit `submitter` field → no heuristic needed (WS doesn't need it because `action` is already required by the protocol).

## Migration Plan

This is a backwards-compatible protocol extension. No client or server is required to opt in.

### Phase 1 — Server accepts `lvt-submitter`

- Add `lvt-submitter` to the `actionFields` skiplist in `parseURLEncodedForm` and `parseMultipartForm` (so it's not echoed back into `msg.Data`).
- Read `lvt-submitter` after `lvt-action` and before the heuristic.
- Add tests in `internal/send/message_test.go` mirroring the existing button-name-as-action cases plus collision cases (empty input + explicit submitter, two empty inputs + explicit submitter — both must route via `lvt-submitter`).
- Server release: minor bump.

### Phase 2 — Client emits `lvt-submitter`

- In `dom/event-delegation.ts`, on form submit, read `(e as SubmitEvent).submitter?.name` and inject it into the FormData / JSON payload as `lvt-submitter`.
- Continue capturing the existing inline `action = submitter.name` for the WS path; the `submitter` field is just additional belt-and-suspenders that survives a server-side rewrite.
- Add a `lvt-form:emit-submitter` opt-in directive for pure no-JS forms (it injects a tiny click-handler that writes a hidden `<input name="lvt-submitter">`).
- Client release: minor bump. Old client + new server: heuristic still runs (existing behavior). New client + old server: extra `lvt-submitter` field is treated as data — no breakage but the heuristic still wins. Both sides up-to-date: explicit submitter wins.

### Phase 3 — Deprecate the heuristic

After two minor versions of overlap (so the npm distribution has settled), mark the heuristic deprecated. Continue accepting it but log a warning when:

- the heuristic resolves to a name AND `lvt-submitter` was absent from the request.

This makes the silent-ambiguity case loud: developers see "your form submitted via the heuristic; consider upgrading the client or adding `lvt-form:emit-submitter`."

### Phase 4 — Remove the heuristic in v0.9.0

`detectSubmitButtonName` and the call sites disappear. The migration target lines up with the `lvt-no-intercept` shim removal already planned for v0.9.0 (see client `dom/link-interceptor.ts` shim and `client/CHANGELOG.md` "Migration: Phase 1A breaking changes").

## Risks and Open Questions

1. **Reserved-field name conflicts.** `lvt-submitter` becomes a reserved form field name. If an app already uses that name for user data, the upgrade will route it as a submitter. Mitigation: this is the same shape as the existing `lvt-action` reservation; document in CHANGELOG with the rest of the Phase 1A migration.

2. **The `lvt-form:emit-submitter` directive is opt-in for pure no-JS.** Without it, a no-JS form still relies on the heuristic. Should it be opt-out (default-on)? Argument for opt-in: no-JS users who don't have the client at all still get the heuristic and won't notice the difference; opt-in avoids surprising them with a sudden hidden field.

3. **Multipart form bodies need separate handling.** The existing helper handles both URL-encoded and multipart, so adding `lvt-submitter` to the skiplist works for both — but file-upload payloads can be large enough that the heuristic was meaningful as a "the action is whichever button you clicked, no extra fields" pattern. Worth verifying the file-upload examples still feel ergonomic.

4. **Should the server also emit a deprecation log when the heuristic fires AND `lvt-submitter` was absent?** The Phase 3 warning above is opinionated. Some teams will want it suppressible via an option. Suggestion: add `WithFormSubmitterMode("strict"|"compat"|"silent")`, default `"compat"` (current behavior) for v0.8.x → `"strict"` (warn) for v0.9.0 → fully strict (heuristic gone) for v1.0.

5. **Naming.** `lvt-submitter` parallels `lvt-action` and `lvt-form:*`. Alternatives: `lvt-form:submitter`, `lvt-clicked`, `_submitter`. The proposal uses `lvt-submitter` for symmetry with `lvt-action`, which has the same precedence shape (explicit override of resolution order).

## Out of Scope

- **Multi-button race conditions on the *same* form.** The current heuristic and the proposed explicit submitter both assume one click → one submitter. Forms that programmatically dispatch synthetic `SubmitEvent`s without a real click are out of scope; they should set `lvt-form:action` directly.
- **Input-type-image submitters.** `<input type="image">` submits coordinates (`name.x`, `name.y`) instead of an empty value. Current heuristic doesn't handle this either. Adding `lvt-submitter` does — coordinate inputs become orthogonal.

## Verification

When this proposal is implemented:

1. New tests in `internal/send/message_test.go` covering `lvt-submitter` as the routing source, including the collision scenarios that the heuristic gets wrong today.
2. New e2e test (or extend an existing form submission e2e in the lvt repo) verifying the explicit submitter path round-trips.
3. Manual smoke on the patterns examples (login, blog, etc.) confirming no regression in the heuristic-driven path.
4. iPhone-on-Tailscale smoke per CLAUDE.md for any form-submit UI that the lvt examples cover.

## Appendix: References

- Heuristic implementation: `internal/send/message.go`, search anchor `detectSubmitButtonName`.
- Client capture: `dom/event-delegation.ts` (client repo), search anchor `__lvtSubmitter`.
- Action resolution doc: `internal/send/message.go`, search anchor `Action resolution order`.
- Existing `actionFields` skiplist: `internal/send/message.go`, search anchor `actionFields := map[string]bool`.
- Recent helper extraction (#327): commit `86a33c67`, file `internal/send/message.go`.
