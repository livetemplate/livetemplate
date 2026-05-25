# TriggerAction Updates Across the WebSocket Reconnect Gap

**Status:** Accepted — Option C+ adopted in this PR (reference doc + idempotency contract). See progress tracker for follow-up state.
**Date:** 2026-05-25
**Issue:** [livetemplate#342](https://github.com/livetemplate/livetemplate/issues/342)
**Related landed work:** PR #336 (Session wiring), PR #337-followup (depth limiting), PR #339-followup (`ErrSessionDisconnected` sentinel)
**Adoption record:** The "Disconnect & Reconnect Contract" section in [docs/references/server-actions.md](../references/server-actions.md#disconnect--reconnect-contract) is the canonical caller-facing version of this contract. Options A and B remain deferred per the triggers in §Recommendation.

## TL;DR

`Session.TriggerAction` calls that fire during a WebSocket disconnect gap are dropped: the goroutine sees `ErrSessionDisconnected`, exits cleanly, and the payload is lost — even though the client reconnects moments later with the same `groupID`. The patterns examples paper over this by having `OnConnect` re-spawn the work on reconnect, which only works when the dispatch is idempotent and cheap.

This proposal makes the implicit contract explicit and defers protocol-level buffering until a real non-idempotent caller surfaces.

**Recommendation: Option C+ (document the contract, add a re-spawn recipe) now; defer Options A/B until a use case demands them.** The implicit "make your handler idempotent and re-spawn on reconnect" contract is already what the codebase relies on (see `docs/proposals/patterns.md` "Reconnect-during-loading double-fire") — formalize it before adding API surface.

**Adopted in this PR:** the canonical version of the contract now lives in [docs/references/server-actions.md § Disconnect & Reconnect Contract](../references/server-actions.md#disconnect--reconnect-contract). Options A and B remain deferred.

---

## Background

The relevant pieces (current as of this proposal):

- `Session.TriggerAction` (`session_impl.go`) fans an action out to every connection in the session group via `Connection.EnqueueDispatch`. If `registry.GetByGroup(groupID)` returns empty and no `PubSubBroadcaster` is configured, it returns `ErrSessionDisconnected`. With pubsub configured, it returns `nil` even with zero local connections (the action may land on another instance).
- The cookie-bound `groupID` is stable across disconnects. Persisted state (any field tagged `lvt:"persist"`) is restored from the `SessionStore` on reconnect, and `ctx.IsReconnect()` returns true in `OnConnect` when restoration happened.
- The WS protocol carries `UpdateResponse{Tree, Meta}` server→client and `{Action, Data}` client→server. There is no per-message sequence number, no client-side ack, and no client→server "last seen" handshake.
- `Connection.DispatchChan` is per-connection (default buffer 16); it does not survive disconnect. There is no per-group queue.

## The Problem

A background goroutine spawned in `OnConnect` calls `TriggerAction` to push state mutations to the client. The canonical cancellation idiom is:

```go
go func() {
    for {
        time.Sleep(tickRate)
        if err := session.TriggerAction("tick", payload); err != nil {
            return // session disconnected — exit cleanly
        }
    }
}()
```

When the WebSocket disconnects briefly (network blip, tab throttle, cellular handoff) the sequence is:

1. Goroutine wakes inside the gap → `GetByGroup` returns empty → `TriggerAction` returns `ErrSessionDisconnected` → goroutine exits.
2. Client reconnects with the same cookie/`groupID`. `Mount` and `OnConnect` re-run. State is restored from `SessionStore` (any `lvt:"persist"` fields) but the in-flight push is gone.
3. **The payload the goroutine would have delivered is lost.** If the UI relied on that push to transition state (e.g. "data loaded", "job 47% complete"), it is stuck until something else re-triggers the work.

Workaround in `docs/proposals/patterns.md` (LazyLoadController): `OnConnect` checks `state.Loading` and re-spawns the goroutine if a pre-completion state is restored. This works for the cheap, idempotent patterns demoed in Session 3 but does not generalize:

- Re-spawning a long-running DB query or paid API call is wasteful and possibly incorrect.
- Non-idempotent dispatches (counter increments, list appends, side effects) corrupt state under the documented "reconnect-during-loading double-fire" race.
- Every pattern author has to remember to write the re-spawn guard in `OnConnect`. Forgetting is silent.

### Scope axes the issue conflates

| Axis | Single-instance | Multi-instance (PubSub) | Server restart |
|------|------------------|--------------------------|----------------|
| Drop on disconnect gap | Yes — error returned | No error (publishes to peers) | N/A |
| Recovery via persisted state | Yes (`lvt:"persist"`) | Yes | Yes |
| Recovery via in-flight dispatch | **No** — this proposal | **No** — durable cross-instance buffering is Phase 2+ | **No** — would need Redis Streams or equivalent |

Any "buffer the in-flight dispatch" design lives on the **single-instance, in-process** axis unless we also adopt a durable cross-instance backplane. That is a much bigger lift (see "Out of scope").

## Design space

### Option A — Per-group ring buffer with sequence numbers + reconnect handshake replay

Server side:
- `TriggerAction` always enqueues to a per-`groupID` bounded ring buffer (size N, e.g. 32; TTL T, e.g. 30 s) before/instead of failing.
- Each outgoing `UpdateResponse` carries a monotonic `Seq` (per group). The buffer key is the seq.
- A new `groupID` reaper removes empty buffers (or the existing `ephemeralSweepTTL` mechanism is repurposed).

Client side:
- Client tracks the highest seq it has applied.
- On reconnect, client sends that seq in the WS upgrade URL (e.g. `?last_seq=42`) or first WS message.
- Server replays any buffered dispatches with `seq > last_seq` before the live stream resumes; the initial-tree send is gated until replay is acknowledged.

Costs:
- Wire format change (new `Seq` field in `UpdateResponse`; new `last_seq` handshake parameter).
- TypeScript client repo (`github.com/livetemplate/client`) coordinated release.
- Protocol version bump.
- Per-group buffer adds a new piece of in-memory state with its own lock, sweeper, and metrics surface.
- "Server restart" still loses the buffer unless backed by Redis Streams — partial fix.

Pros:
- Solves the problem for *all* `TriggerAction` callers without opt-in.
- Buffer-by-default matches user intuition ("I called TriggerAction, the user should see it").

Cons:
- Largest surface change of the three options.
- The seq protocol must agree with the live WS stream's ordering (every Send goes through the same path, or dispatches replay out-of-order with subsequent live updates).
- A bounded buffer still has a "I disconnected for too long" hole — patterns still have to handle the overflow case. The implicit contract is *not* removed; it is just pushed to a longer window.

### Option B — Opt-in `Session.TriggerActionDurable`

New API method with the same signature as `TriggerAction` but with buffering semantics. Default `TriggerAction` keeps current behavior.

Costs:
- New surface to document, test, and explain when to choose.
- Implementation still needs the per-group buffer and reconnect replay (so most of A's machinery).
- Easier to ship incrementally only if we *do not* also need wire-format seq — which means callers may see duplicate dispatches on reconnect (cannot dedup without per-message ids).

Pros:
- Existing callers unaffected.
- Explicit intent at the call site.

Cons:
- "Opt-in correctness" — same class as remembering to re-spawn in `OnConnect`. The footgun is renamed, not removed.
- Without seq-based dedup, durable mode still requires idempotent handlers, so the API name overpromises.

### Option C — Document the current contract; add a small recipe

Treat what the framework currently does as the contract:

1. `TriggerAction` is best-effort, not durable. Goroutines must exit on `ErrSessionDisconnected`.
2. Persistent state goes through `lvt:"persist"` fields and the `SessionStore`.
3. Reconnect recovery is the controller's job, executed in `OnConnect` (gated by `ctx.IsReconnect()`).
4. All push handlers must be idempotent so the documented double-fire race is benign.

Ship as part of this option:
- A short reference doc (`docs/references/triggeraction.md` or a section in `docs/references/server-api.md`) covering the gap behavior, the `ErrSessionDisconnected` sentinel, the idempotency requirement, and the `OnConnect` re-spawn recipe.
- A canonical helper recipe — *not* a framework helper. Document the shape inline. The sketch below uses stand-in names (`State`, `InProgress()`, `runWork`, `JobID`); substitute your concrete state type and predicate:

  ```go
  func (c *Ctrl) OnConnect(state State, ctx *Context) (State, error) {
      // Respawn whenever state shows in-flight work, regardless of whether this
      // is a new-connect or reconnect. On a fresh new-connect, InProgress() is
      // the zero value (false) so this is a no-op. On reconnect, restored
      // persisted state reflects whatever the prior connection committed.
      if !state.InProgress() {
          return state, nil
      }
      session := ctx.Session()
      if session == nil {
          return state, nil
      }
      go runWork(session, state.JobID) // must be idempotent by construction
      return state, nil
  }
  ```

- Update `docs/proposals/patterns.md` "Reconnect-during-loading double-fire" prose to reference the new doc rather than #342.

Costs:
- Docs only. No code.

Pros:
- Honest framing of what the framework actually guarantees.
- Doesn't add API surface that we may regret when a real durable use case arrives.
- Leaves the door open for A or B if/when a non-idempotent caller surfaces.

Cons:
- Does not solve the underlying "lost push" — patterns must continue to be idempotent.
- "Documented footgun" carries a non-zero educational cost.

### Hybrid considered and rejected

A hybrid that ships C now and reserves the API name `TriggerActionDurable` for B was considered but adds nothing — reserving a name without an implementation is noise. If B becomes the right answer later, the API can be added then.

## Recommendation

**Adopt Option C now. Defer A and B until a concrete non-idempotent use case demands buffering.**

Rationale:

- The codebase already commits to Option C semantics (see `docs/proposals/patterns.md` "Reconnect-during-loading double-fire" — the framework does *not* invalidate old sessions, idempotency is the explicit contract). Making this explicit eliminates a hidden contract without changing behavior.
- LiveTemplate is alpha with no external users; we can revisit the API later without compat concerns.
- Option A's wire-format change is a real chunk of work (server + TS client + protocol bump). Spending it before a forcing use case appears is speculative.
- Option B's "opt-in correctness" trades one footgun for another; not worth the surface.
- If/when a real caller surfaces that genuinely cannot be idempotent (job-queue progress with strict-once semantics, audit-logged notifications, paid-API result streams), revisit. The proposal sketch in this doc remains the starting point.

### Triggers that would justify revisiting

Promote A or B from "deferred" to "do now" when any of:

- A patterns example, livetemplate-built app, or external user files an issue that **cannot** be solved by idempotency. The issue should describe the dispatch's exact non-idempotency (not "would be nicer if").
- The patterns example proposes a feature in category "Async Operations / Server Push" that an idempotent design cannot express cleanly.
- A multi-instance deployment ships and the proposal is updated to cover durability across instance restarts (then Redis Streams becomes the substrate and the wire-format change is part of a larger PubSub-Streams migration anyway).

## Implementation sketch (if A is later adopted)

Captured here so a follow-up does not re-derive it:

1. **Server-side ring** — `internal/session/dispatchbuffer.go`:
   - `type DispatchBuffer struct { mu sync.Mutex; ring []bufferedDispatch; head, tail int; lastSeq uint64 }`
   - Keyed in a new `map[groupID]*DispatchBuffer` on `liveHandler` (or hung off the registry).
   - `Push(payload) seq` and `ReplayAfter(seq) []bufferedDispatch`. Bounded by size *and* age.
   - Reaper goroutine evicts empty buffers after `ephemeralSweepTTL`.
2. **Wire format** — bump `protocol_version`:
   - Server: `UpdateResponse` gains `Seq uint64`. Emitted for both action responses and dispatched actions, monotonic per group.
   - Client (separate repo coordination): tracks `lastSeq`; sends as `?lvt_seq=` in the upgrade URL.
3. **WS handshake** — `handleWebSocket`:
   - Parse `lvt_seq` from `r.URL.Query()`.
   - After `Mount`/`OnConnect`/`persistState`, before the initial-tree send, call `dispatchBuffer.ReplayAfter(lastSeq)` and emit each replayed dispatch using the existing `writeUpdateWebSocket` path (with its original seq).
   - Then emit the initial tree (also seq-tagged).
4. **Metrics** — new counters in `internal/observe`:
   - `livetemplate_dispatch_buffer_size{group_id="..."}` gauge
   - `livetemplate_dispatch_buffer_overflow_total` counter
   - `livetemplate_dispatch_buffer_replays_total` counter
5. **Tests:**
   - Single-connection disconnect → buffered TriggerAction → reconnect → replay arrives in order.
   - Buffer overflow → oldest dropped → metric incremented → reconnect missed dispatches → caller's idempotent handler reconciles.
   - Multi-tab (multiple connections in same group) — confirm whether buffer is shared (yes — keyed by groupID) and that replay does not double-fire to a tab that was connected during the dispatch.
   - Concurrent TriggerAction + Unregister — confirm the buffer push happens regardless of registry state.

Most subtle invariant: a `TriggerAction` fired while a tab is still connected must be buffered *and* delivered live to that tab; reconnect replay must not duplicate-deliver to a tab whose `lastSeq` already covers the dispatch.

## Out of scope

- Multi-instance buffering durability. Requires a durable cross-instance store (Redis Streams or equivalent), not the in-process ring. Separate proposal if/when needed.
- Server-restart durability. Same.
- Strict-once / exactly-once delivery semantics. Buffered replay is at-least-once unless paired with client-side dedup keys on the dispatch.
- Per-message ack handshake. Would solve a different problem (delivery confirmation) and is not necessary for the gap-replay use case.
- Changes to `Mount`, `OnConnect`, `OnDisconnect` signatures.

## Test plan (for the documentation-only deliverable)

This proposal is doc-only. The deliverables to verify (in the follow-up implementation PRs for Option C) are:

- The new reference doc renders cleanly on the docs site.
- `docs/proposals/patterns.md` "Reconnect-during-loading double-fire" cross-references the new reference doc.

When/if A is later adopted, the test plan in "Implementation sketch" applies.

## Progress tracker

| Phase | Status | Notes |
|-------|--------|-------|
| 0. Draft proposal (this doc) | Done | This PR |
| 1. (C) Document the contract in caller-facing reference | Done | "Disconnect & Reconnect Contract" section in `docs/references/server-actions.md` |
| 2. (C) Cross-link from `patterns.md` | Done | "Reconnect-during-loading double-fire" prose updated |
| 3. PR review + merge | Pending | Tracked by the PR itself, not this doc |
| 4. (A/B) Implementation | Deferred | Open new issue with a concrete non-idempotent use case before starting (see §Triggers) |

## References

- Issue [livetemplate#342](https://github.com/livetemplate/livetemplate/issues/342) — the original report.
- `session_impl.go` — current `TriggerAction` (search for `ErrSessionDisconnected`).
- `mount.go` — WS connect path (search for `handleWebSocket`) and `ConnectKindReconnect` (search for `connectKind`).
- `context.go` — `IsReconnect`, `IsNewConnect`, `IsInitialMount` (search for `ConnectKind`).
- `internal/session/registry.go` — `Connection`, `DispatchChan`, `GetByGroup` (search for `EnqueueDispatch`).
- `docs/proposals/patterns.md` — "Reconnect-during-loading double-fire" section (search for `double-fire`).
- `docs/references/pubsub.md` — multi-instance fan-out and the `Seq` field already used for cross-instance ordering (relevant prior art for any seq design).
