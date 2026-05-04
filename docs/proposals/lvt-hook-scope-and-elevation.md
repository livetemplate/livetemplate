# `lvt-hook` scope and elevation assessment

**Status:** Decision Needed
**Tracking issues:** [livetemplate/livetemplate#294](https://github.com/livetemplate/livetemplate/issues/294), [livetemplate/client#43](https://github.com/livetemplate/client/issues/43)
**Related:** [docs/proposals/lifecycle-hooks-proposal.md](./lifecycle-hooks-proposal.md), [ROADMAP.md](../../ROADMAP.md) "Next: Feature Development"

## TL;DR

Two lifecycle-hook mechanisms exist in LiveTemplate today. They serve overlapping purposes but were not designed together:

1. **Already shipping (inline attributes):** `lvt-mounted`, `lvt-updated`, `lvt-destroyed` on individual elements. The attribute value is a JS expression evaluated via `new Function()` with `this` bound to the element. Implemented in `livetemplate-client.ts` (search anchor `executeLifecycleHook`).
2. **Proposed (named registry):** `lvt-hook="chart"` on individual elements, with hook implementations registered at `LiveTemplateClient` construction time. Status `Proposed` in `lifecycle-hooks-proposal.md`. Currently in ROADMAP under "Next: Feature Development".

The existing proposal does not mention the inline attributes — it was either written before they shipped or written without reconciling with them. **Before `lvt-hook` is elevated from "Next" to "Now", we need to decide what relationship the two mechanisms have.** This doc frames that decision; it does not decide it.

## What ships today (inline attributes)

In `livetemplate-client.ts`:

```ts
// search anchor: executeLifecycleHook
this.executeLifecycleHook(node as Element, "lvt-mounted");
this.executeLifecycleHook(fromEl, "lvt-updated");
this.executeLifecycleHook(node as Element, "lvt-destroyed");
```

The handler reads the attribute's string value and runs it via `new Function()` with the element as `this`:

```html
<div lvt-mounted="this.scrollIntoView({ behavior: 'smooth' })">…</div>
<canvas lvt-mounted="this.chart = new Chart(this, { /* … */ })"
        lvt-destroyed="this.chart.destroy()"></canvas>
```

**Strengths:**
- Zero registration boilerplate. A template author drops the attribute and it works.
- Co-located with the element — easy to read and grep for.
- No client-side wiring at construction time; the page is self-describing.

**Limitations:**
- **No reusable named behaviors.** Every `<canvas>` chart in a template repeats the same `new Chart(...)` boilerplate. There's no `chart:` named hook that lets multiple elements share one implementation.
- **No `data-*` parsing helper.** Each handler has to read `this.dataset.foo` manually. Hooks proposed in `lifecycle-hooks-proposal.md` provide `this.data` (auto-parsed dataset).
- **No `pushEvent` API.** The inline expression has access to `this` (element) only — no built-in way to send a custom event back to the server.
- **No `connected`/`disconnected` callbacks.** Inline attributes only cover DOM lifecycle (mount/update/destroy), not WebSocket connection state.
- **Security/CSP friction.** The handler value is a string evaluated via `new Function()`, which is essentially `eval`. CSP `unsafe-eval` must be allowed for the directive to work — apps with strict CSP can't use this attribute at all (a real audience, esp. embedded contexts).

## What's proposed (`lifecycle-hooks-proposal.md`)

Named hook registry, registered at client construction:

```ts
new LiveTemplate('/dashboard', {
  hooks: {
    chart: {
      mounted() { /* uses this.el, this.data, this.pushEvent */ },
      updated() { … },
      destroyed() { … },
      connected() { … },
      disconnected() { … },
    }
  }
});
```

Templates reference hooks by name:

```html
<canvas lvt-hook="chart" data-chart-type="line" data-chart-data="{{.ChartJSON}}"></canvas>
```

**Strengths:**
- Reusable: one `chart` hook, many `<canvas lvt-hook="chart">` elements.
- Provides `this.data` (auto-parsed dataset), `this.pushEvent()`, and connection-state callbacks.
- No `eval` / `new Function()` — the registry is plain JavaScript; CSP-friendly.

**Costs:**
- Construction-time registration boilerplate: every template using a hook needs the registry populated at client-init.
- Requires a build step or inline `<script>` to register hooks before `LiveTemplate(...)` is called.
- Two-place declaration: hook definition lives in JS, hook reference lives in HTML.

## The unanswered question

**How do these two mechanisms coexist?** Three coherent options:

### Option A — `lvt-hook` replaces the inline attributes

Deprecate `lvt-mounted` / `lvt-updated` / `lvt-destroyed` when `lvt-hook` ships. Migrate users to the registry approach. Eventually remove the inline attributes (target: the same v0.9.0 cut as other Phase 1A removals).

- **Pro:** one mechanism, one mental model.
- **Pro:** CSP-strict apps eventually get a clean story.
- **Con:** breaking change — every existing inline-hook user has to migrate to a registry, even for one-off cases (`lvt-mounted="this.focus()"`).
- **Con:** the inline form is much terser for one-shot use cases — replacing it with a registry registration for those cases is overkill.

### Option B — Both ship, serving different audiences

Inline attributes for one-shot, element-local behavior (focus, scroll-into-view, simple side effects). `lvt-hook` for reusable named behaviors (charts, editors, maps).

- **Pro:** users pick the right tool for the job. The proposal-as-written serves a specific audience without changing the shipping behavior of inline hooks.
- **Pro:** no breaking change.
- **Con:** two mechanisms to learn, document, and maintain. Newcomers will reasonably ask "which one should I use for X?"
- **Con:** doubled surface for tests, doubled docs, ~doubled bug surface.

### Option C — Inline attributes become syntactic sugar for one-element registry hooks

Keep the inline attribute syntax but reimplement it on top of the registry mechanism. Internally, `<div lvt-mounted="…">` registers a one-shot anonymous hook with a `mounted` callback that runs the inline expression.

- **Pro:** one underlying mechanism, two surface syntaxes.
- **Pro:** users get tersity *and* `this.data` / `pushEvent` / connection-state callbacks if they switch.
- **Con:** the inline form still uses `new Function()` for the expression, so CSP friction remains for that surface — only the registry path is CSP-clean.
- **Con:** more implementation complexity than B; the registry must support unnamed one-shot hooks.

## Pre-elevation questions

Before `lvt-hook` lands in "Now", these need answers:

1. **Which option (A / B / C / other) is the desired end state?** The proposal as written assumes the registry exists in isolation; it doesn't say which option it implies. Inline attributes shipped after the proposal was written, which is why the existing proposal doesn't mention them.

2. **What's the migration story for existing inline-hook users?** Examples that ship today (`lvt-mounted="this.scrollIntoView(...)"`) will continue to work under B and C; under A they break. Users who have written inline hooks need migration guidance regardless of which option.

3. **Does `lvt-hook` need to land before or alongside the v0.9.0 breaking-change cut?** v0.9.0 already plans to remove the `lvt-no-intercept` shim and (per the explicit-submitter proposal) the form-submit heuristic. Adding the inline-hooks deprecation under option A is a third breaking change in the same cut.

4. **What's the relationship to `lvt-fx:`?** Both inline attributes and `lvt-hook` overlap with the existing `lvt-fx:` directive surface (`lvt-fx:scroll`, `lvt-fx:highlight`, `lvt-fx:animate`). The fx directives are CSS-driven, not JS-callback-driven, but the boundaries between "fx" and "hook" should be drawn explicitly so users know which surface to reach for.

5. **CSP target.** Option A is the only path that gets CSP-strict apps to a clean state. Is CSP-strict (`unsafe-eval` not allowed) a supported audience? If yes, A becomes much more attractive.

## Recommendation

**Defer elevation; pick an option first.** Concretely:

- Add a "Lifecycle hooks: which mechanism?" entry to a future ADR (similar to the explicit-submitter proposal's no-JS support ADR pattern).
- Until that decision is made, don't implement `lvt-hook`. The existing inline attributes cover most one-off use cases; the registry adds value primarily for reusable named behaviors, which is a smaller user need than the proposal might suggest.
- If the answer is **B** (both ship): the existing proposal can be implemented mostly as written, but it should add a "Relationship to inline lifecycle attributes" section documenting when to use which.
- If the answer is **A** (replace inline): add a deprecation timeline section to the existing proposal coupled to v0.9.0 breaking-change removal, alongside `lvt-no-intercept` and the form-submit heuristic.
- If the answer is **C** (sugar): the existing proposal needs to be amended to describe how the inline syntax compiles to a registry registration, and the implementation has to support unnamed/one-shot hooks.

**Why not just elevate now?** Without a stance on options A/B/C, an implementation PR will encode a decision implicitly. Better to make the decision explicitly than discover it from the diff during code review.

## Verification

When `lvt-hook` lands (whichever option):

1. The chosen relationship is documented in the implementation PR's CHANGELOG entry.
2. Existing examples using inline `lvt-mounted` etc. either continue to work (B, C) or have a migration entry (A).
3. The lvt-fx:/lvt-hook boundary is documented in `docs/references/client-attributes.md`.
4. CSP-strict consumers have a clear "which surface to use" answer in the docs.

## Appendix: References

- Inline lifecycle hook implementation: `livetemplate-client.ts` (client repo), search anchor `executeLifecycleHook`.
- Existing `lvt-hook` proposal: `docs/proposals/lifecycle-hooks-proposal.md`.
- ROADMAP entry: `ROADMAP.md`, search anchor `Lifecycle Hooks`.
- v0.9.0 breaking-change cut tracking: see explicit-submitter proposal Phase 4 and client `dom/link-interceptor.ts` shim removal.
