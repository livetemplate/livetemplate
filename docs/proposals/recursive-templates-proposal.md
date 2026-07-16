# Recursive `{{template}}` Support

**Status:** Approved (2026-07-15) — the three headline decisions are settled (see § Decisions);
Phase 1 (crash → `ParseError` guard) is ready to implement.
**Tracking:** Tier C item C8 of the boilerplate-reduction pass (issue #483)
**Audience:** livetemplate maintainers — the approved design of record for implementation.

## TL;DR

**Problem.** livetemplate inlines every `{{template "name" .}}` invocation into one flat
template at parse time. A self-referential template (a folder tree, a nav tree, a threaded
comment list) expands forever and **stack-overflows during `Parse`**. **prereview** hits this
directly: it wants a reactive folder tree, cannot use a recursive livetemplate template, and
works around it by rendering the tree with standalone `html/template` and injecting the result as
opaque `template.HTML` — which **removes the whole subtree from livetemplate's reactive diffing**:
the entire tree is re-rendered and re-sent on every keystroke, defeating the framework's core
value. **tinkerdown** corroborates that recursive tree UI is a recurring need — its sidebar nav is
built by a recursive Go function emitting the same native-`<details>` markup — though its pages are
currently served as static HTML (not livetemplate-reactive), so it is a latent case, not a second
victim of the flatten bug. The core justification rests on the objective crash + prereview's
documented reactive-diffing loss, not on consumer count.

**Solution (recommended).** Stop inlining *recursive* invocations. Detect a self-referential
invocation graph at parse time (the guard that's missing today), and evaluate those invocations
at **runtime as a nested `TreeNode`** — the same mechanism `{{range}}` already uses. Recursion
then terminates on the **data** (a finite tree of nodes), not on parse-time text expansion, and
each level is a first-class diffable subtree. Non-recursive invocations keep flattening
unchanged (zero regression risk to existing composition).

**Non-goal.** This is not "arbitrary Go template recursion at any cost." It's "let a template
call itself over a finite data structure and stay inside the reactive tree."

**What ships now vs. later (measured, not assumed).** What lands with C8: recursive templates
*render* correctly and *update reactively* in place (DOM/focus/scroll preserved) — the capability
prereview and tinkerdown lack today. What does **not** yet land: genuinely *minimal* per-edit
payloads. Recursive range items are keyed by a deep content hash, so a deep edit re-sends its whole
enclosing top-level branch; for a deep, narrow tree that is ~the size of re-sending the entire
opaque string it replaces (benchmarked: ~24 KB vs ~23.5 KB, at ~25× the CPU). Scoping deep edits to
a single leaf `["u", key, {…}]` needs the data-key-through-the-wrapper follow-up (§ Remaining design
notes). So C8 is shipped for the *capability*, with update-size optimization tracked separately.

---

## The Problem

### What breaks today

`{{template "name" pipe}}` is never a runtime construct. At parse time
(`template.go:1195-1207`, `parseInternal`), `parse.FlattenTemplate` (`internal/parse/flatten.go`)
**textually inlines** each invocation's body into the caller and replaces the template source
with the flattened string. The `TemplateInvokeConstruct` named in CLAUDE.md is conceptual — it
exists in no Go source; the real work is stdlib `*parse.TemplateNode` handling in
`walkAndFlatten` (`flatten.go:286-325`).

That routine looks the invoked template up by name (`flatten.go:288`) and **re-enters its body**
(`flatten.go:315/322`) with **no visited-set, no depth counter, and no cycle check**. So:

```
{{define "treeNode"}}
  <li>{{.Name}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul></li>
{{end}}
```

makes `walkAndFlatten` re-enter `treeNode`'s body endlessly, writing an infinitely long template
string → **stack overflow at `Parse`**, before any `TreeNode`, evaluation, or fingerprint stage
is reached. (The tree walker `walker.go:42-47` even returns a `ParseError` "template invocation
found — should be flattened" if an invocation ever survives to tree-build time, confirming
invocations are *only* ever inlined, never evaluated.)

The support matrix *does* flag this — `template-support-matrix.md` has a "Circular template
references | ❌ | Not supported, would cause infinite loop" row — but it **mischaracterizes the
failure mode**: it isn't a runtime infinite loop, it's a **stack overflow during `Parse`** (the
recursive `walkAndFlatten` above), before any tree or render. And there is **no implementation
guard** — the crash is unbounded on template source. Phase 1 both corrects that doc row and turns
the crash into a clear `ParseError`, independent of whether the rest of the design lands.

### The real cost: opaque HTML defeats reactive diffing

The one app that hits this inside a livetemplate reactive tree, prereview
(`internal/review/filetree.go`), shows the cost sharply:

```go
// fileBrowserTmpl is a standalone html/template (NOT processed by livetemplate)
// … because the folder tree is self-recursive ({{template "treeNode"}} calls
// itself) and livetemplate flattens {{template}} calls at parse time, which
// overflows on recursion. … the rendered markup is injected back into
// prereview.tmpl as template.HTML — the same native-<details> approach
// tinkerdown uses for its nested nav.
func (s PrereviewState) FileBrowserHTML() template.HTML { … }
```

The template `emits {{.FileBrowserHTML}}` — a single dynamic slot holding the **entire** rendered
tree as one HTML string. To livetemplate that slot is one opaque value: when *any* node changes
(a file gets selected, a comment count ticks), the whole subtree string differs, so the whole
subtree is re-serialized and re-sent. The recursive part gets **none** of livetemplate's minimal
`["u", key, …]` updates — even though prereview already annotates every node with `data-key`,
precisely so it *could* be diffed if the framework rendered it.

That is the boilerplate C8 removes: a hand-maintained standalone template, a zero-arg
`template.HTML` state method, and the silent loss of incremental updates for exactly the kind of
large, frequently-touched structure (a repo's whole file tree) where incremental updates matter
most.

### Evidence

- **prereview (the direct consumer)** — `internal/review/filetree.go` + `filetree.tmpl`
  (`treeNode` recurses on `.Children`; native `<details>`; `data-key` on every node; injected as
  `template.HTML`). It runs inside a livetemplate reactive tree, so the opaque slot is a real,
  measured loss of incremental diffing. The `filetree.go` comment states the cause verbatim
  ("livetemplate flattens `{{template}}` calls at parse time, which overflows on recursion").
- **tinkerdown (corroborating pattern, not a bug victim)** — the sidebar nav is rendered by a
  recursive **Go** function (`writeNavNode`, `internal/server/server.go:3660`, recursing over
  `node.Children` and emitting the same native-`<details>` markup); the tree it walks is built by
  `buildNavPageNode` (`internal/site/manager.go:121`). But tinkerdown serves pages as **static
  HTML** (`servePage`: *"For now, just serve the static HTML"*) — the nav is never a livetemplate
  reactive tree, and stdlib `html/template` would handle its recursion fine — so tinkerdown did
  **not** hit the flatten bug. It confirms recursive-tree UI is a recurring need and that
  native-`<details>` is the idiom, and it would become a real consumer *if* it made the nav
  reactive — a latent case, kept separate from the direct evidence per this project's
  "don't bundle unverified downstream-impact claims" discipline.

This is the one Tier C item with a documented direct consumer (prereview) *and* a genuine
capability gap (not a discoverability or docs gap) — the objective parse-time crash plus
prereview's reactive-diffing loss are what earn it a real implementation rather than a doc note.

---

## Approaches considered

### Approach A — Runtime invocation as a nested `TreeNode` (recommended)

Treat a recursive `{{template "name" .}}` as a **node boundary** instead of inlined text. At
render time, the framework looks up the named template, evaluates its pre-built sub-tree against
the invocation's dot, and splices the result in as a **nested `TreeNode`** (a child subtree with
its own statics, dynamics, and fingerprint). Recursion bottoms out when the data does — a finite
tree of nodes yields a finite tree of `TreeNode`s.

This is deliberately the **same shape `{{range}}` already produces**: a range renders its item
template once per item as a nested tree; a recursive template renders its named body once per
node as a nested tree. The build/diff/wire machinery for nested trees already exists and is
exercised — see `tree_test.go` nested cases and the range operations (`["u"/"i"/"r"/"o"]`). We
are routing recursion through the path that works, not inventing a new one.

- **Pro:** restores full reactive diffing for recursive structures (the whole point). Reuses the
  nested-tree wire format — no new client-side ops. Backward compatible if applied *only* to
  recursive invocations (see "Selective" below).
- **Con:** the deepest change — parse must detect recursion and emit a runtime boundary; the
  framework must retain named templates as pre-built sub-trees and instantiate them per
  invocation; needs an eval-time depth guard for pathological *data* (a self-referential
  `.Children` pointer cycle, distinct from a merely-deep-but-finite tree — see Phase 4 for how
  max-depth covers both shapes).

### Approach C — A sanctioned opaque-HTML subtree primitive (lighter fallback)

Bless the escape hatch: a first-class helper (e.g. `livetemplate.RawSubtree(html)` or a
documented `template.HTML` convention) that the framework treats as a diff-opaque leaf, with
framework-managed re-render. This is essentially what the apps do today, made official.

- **Pro:** small, low-risk; removes the "is this safe?" ambiguity around injecting `template.HTML`.
- **Con:** does **not** restore incremental updates — the subtree still re-sends wholesale on any
  change. It sanctions the boilerplate rather than removing its cost. Worth shipping only if A is
  deemed too large for now; A supersedes it.

### Approach B — Depth-bounded inlining (rejected)

Add a visited-set / max-depth guard to `walkAndFlatten` so recursion fails gracefully instead of
overflowing. This prevents the crash but **cannot render data-driven depth** — the depth is
unknown at parse time, so bounded inlining can only ever produce a fixed number of levels. It
turns a crash into a wrong render. Rejected as a solution (though the *guard itself* is worth
adding defensively — see Phase 1).

---

## Design (Approach A)

### 1. Parse: detect recursion, split the invocation graph

Add the missing cycle detection to flattening (`internal/parse/flatten.go`). While walking the
invocation graph, track an in-progress set of template names. Two outcomes:

- **Acyclic invocation** (today's case): inline as now. **Zero behavior change** — all existing
  composition tests pass untouched.
- **Cyclic invocation** (a name reachable from itself): do **not** inline. Mark every template on
  the cycle as *runtime-invoked* and emit a **runtime invocation boundary** in the built tree at
  each `{{template "name" .}}` site on the cycle, instead of splicing the body.

This selective split is the key to low risk: non-recursive composition — the overwhelming common
case — is completely unaffected. Only self-referential templates take the new path.

### 2. Retain named templates as re-walkable ASTs (name→AST registry)

Today flattening inlines the separate `{{define}}`s and discards them. For runtime invocation,
the framework must **keep** each runtime-invoked template's parsed body — as its **AST, re-walked
per invocation**, not a compiled/cached sub-tree. (There is no "compile-once" model to mirror:
`{{range}}`, `{{if}}`, and `{{with}}` all re-walk their raw `*parse.ListNode` on every evaluation;
the registry is the same idea keyed by name.) As implemented, `FlattenTemplate` leaves recursive
calls verbatim and appends each flattened body as a `{{define}}` block, so the un-wrapped flattened
string carries them; `parse.Parse` then collects the associated templates into a
`registry map[string]*parse.Tree` on the compiled `Template` — no separate threading needed.

### 3. Evaluate: instantiate the sub-tree per invocation → nested `TreeNode`

When evaluation reaches a runtime invocation boundary:

1. Resolve the invoked name against the registry (step 2).
2. Evaluate that sub-tree against the invocation's **pipe value** as the new dot. `$` naturally
   rebinds to that dot per invocation — matching Go's rule that a template calling itself drops
   the caller's `$`. This is a non-issue for prereview: its recursive `treeNode` body never
   references root `$` (its only `$` — `{{$.FileFilter}}` — is in the non-recursive `fileBrowser`
   wrapper), so the rebind doesn't change its behavior.
3. Emit the result as a **nested `TreeNode`** under the current node (the same slot a nested tree
   already occupies).

Recursion terminates because step 2's dot advances down the finite data tree (`.Children` empties
out at the leaves).

### 4. Eval-time depth/cycle guard (safety)

Runtime recursion is bounded by data, so a **cyclic data structure** (a node whose `.Children`
transitively contains an ancestor) would loop forever at eval. Add a guard: either a
configurable **max-depth** (clear error on exceed) or a pointer-visited set mirroring the
fingerprint code's existing `visitPath` pattern (`fingerprint.go:46,57-64`). Max-depth is simpler
and gives a comprehensible error; recommend a generous default (e.g. 128) overridable via option.

### 5. Fingerprint: lean on the existing per-instance cache — do **not** cache per template name

**Naive intuition (wrong):** "a recursive structure repeats the same statics at every level, so
fingerprint the invoked template once by name and reuse it." This is unsafe, and the worked
example below is exactly the counter-case: `{{if .IsDir}}<ul>…</ul>{{end}}` means a **leaf** node
(`IsDir=false`) and a **directory** node (`IsDir=true`) take different conditional branches and so
build **structurally different** trees — different structure fingerprints. A per-name cache would
conflate them. Most real recursive templates have a leaf/branch conditional, so this is the common
case, not an edge case.

**Correct approach — reuse existing machinery, add nothing.** The structure fingerprint is
**content-independent**: two nodes with the same structure hash to the same value regardless of
their dynamic content (per `CalculateStructureFingerprint`). A recursive template therefore emits
a **small, bounded set of distinct branch-shapes** (here: leaf-shape and dir-shape), not one shape
and not O(depth) shapes — and all nodes of a given shape already share a fingerprint *value* by
equality, with no per-name cache. Recomputation is already avoided by the **existing per-instance
lazy cache** `TreeNode.GetStructureFingerprint()` (`internal/build/types.go`): each node computes
its fingerprint once on first access. So the design needs **no new fingerprint caching at all** —
it must only *not* introduce a per-name cache that would collapse distinct branch-shapes. The
existing `fingerprint.go` cycle guard (`visitPath`) keeps a cyclic `TreeNode` graph well-defined.

The user-visible payoff is statics *suppression*, and it composes with the per-shape fingerprints:
at **stable depth**, the per-position `ClientNeedsStatics` compares a node's fingerprint to its
prior render's and sends dynamics-only when the shape is unchanged. A node that *flips branch*
(a leaf becoming a directory) changes shape → its statics are correctly re-sent. A
newly-materialized level at *growing* depth is position-new → its statics are (correctly) sent
once. Suppression is per-node-per-shape, not a blanket "identical across all levels" guarantee.

### 6. Wire format & diffing — no new client ops

Each level serializes as a nested tree: statics sent on first render, dynamics-only on update.
Direct-child list edits (add / insert / reorder / remove) produce granular range ops
(`["a"]`/`["i"]`/`["o"]`/`["r"]`) carrying only the changed items — **no new wire operations and no
client changes** — verified, not assumed, against the client source below.

> **Keying caveat (implemented behavior, corrected from the original claim).** Each `{{range}}`
> item is a `{{template}}` invocation, so its top-level tree is the invocation wrapper (empty
> statics). `buildRangeTreeWithStatics` looks for an explicit `data-key` in those top-level statics
> and, not finding it (the wrapper hides the item's real `<li data-key>`), falls back to a **deep
> content hash** over the item's dynamics. Identity is still exact — the `data-key` value is one of
> the hashed dynamics, so distinct items get distinct keys. But because the hash is *deep* (a
> directory item's dynamics include its whole nested subtree), editing a node **deep** in the tree
> changes every ancestor's key, so the enclosing branch is re-sent whole (its unaffected siblings
> are not). A per-leaf `["u", key, {…}]` for deep edits requires honoring the explicit `data-key`
> *through* the invocation wrapper; a first attempt (unwrapping the item) regressed the deep-edit
> case to a full range re-send, so it is deferred to a focused follow-up. Renders are always
> correct; this is update *size*, not correctness. Pinned by
> `TestRecursiveTemplate_DescendantBranchRebuild`.

- **The client has no fingerprint concept at all.** Fingerprints are server-only: the server uses
  them to *decide whether to send statics* (`ClientNeedsStatics`), and the wire never carries them
  (`grep fingerprint` over `github.com/livetemplate/client` returns zero hits, as of client
  v0.16.5). So there is no fingerprint-keyed client cache that repeated per-level structure could
  collide in — the concern that recurring levels confuse the client does not arise.
- **The client caches statics *positionally* and merges depth-agnostically.** `TreeRenderer`
  (`client/state/tree-renderer.ts`) stores statics inline at each tree path in `treeState`, and
  `deepMergeTreeNodes` recurses on object nesting keyed by a path string with **no depth cap**.
  Unbounded data-driven recursion is structurally identical to arbitrarily-deep nested ranges,
  which the client already applies — so no client change is needed to *apply* the updates.

The one design obligation this puts on the **server** eval (not the client): when recursion depth
*changes* between renders (a node gains or loses children), each newly-materialized level is a
tree position that had no prior node, so that level's statics must be (re)sent. The mechanism is
**not** a literal `ClientNeedsStatics(nil, new)` call at that position — it is the ordinary
fingerprint-based per-position resend: the node whose structure changed (e.g. the `{{if .IsDir}}`
slot going from empty to a nested `<ul>`+range) gets a different `GetStructureFingerprint()`, so
`ClientNeedsStatics` returns true for it and `PrepareTreeForClient` keeps its statics; the new
range items carry their own statics on insert. The recursive eval must therefore produce genuinely
position-distinct nested nodes so this per-position logic engages. **Verified** by
`TestRecursiveTemplate_MinimalUpdate_DepthGrows` (a leaf→directory flip resends the new level's
`"s"`) — see Open question 5.

### Worked example

```
{{define "treeNode"}}
  <li data-key="{{.Path}}">
    <span>{{.Name}}</span>
    {{if .IsDir}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul>{{end}}
  </li>
{{end}}
```

- Parse detects `treeNode` → `treeNode` (via the `range`'s invocation) is a cycle → `treeNode`
  becomes runtime-invoked.
- First render: full nested `TreeNode` tree; `treeNode`'s statics transmitted once per level the
  data materializes, then cached client-side positionally and merged dynamics-only thereafter.
- User renames a file 4 levels deep: the change reaches the client, and the diff is scoped to the
  branch that contains the edit — its sibling branches are **not** re-sent. (Per the keying caveat
  above, today the enclosing branch is re-sent whole rather than a single per-leaf
  `["u", "<path>", {…}]`; scoping the deep edit down to the leaf is a tracked follow-up.)
  **How much this beats the status quo depends on the tree's shape, and the honest answer is "less
  than you'd hope for deep, narrow trees."** The enclosing branch is `1/branch` of the tree, so a
  wide tree (many top-level entries) re-sends a small slice, but a deep, narrow one re-sends a large
  fraction. Measured (`recursive_template_bench_test.go`, depth-5 branch-3): a deep-leaf edit's
  update is **~24 KB vs ~23.5 KB** for re-sending the entire opaque `{{.FileBrowserHTML}}` string —
  essentially equal in bytes, and ~25× the CPU to compute. So the unconditional win here is **not**
  payload size; it is (a) rendering correctly at all (the status quo overflows the flattener) and
  (b) applying the change as an in-place tree update that preserves DOM/focus/scroll state, where the
  opaque string can only replace `innerHTML` wholesale. The payload-size win arrives with the
  data-key-through-the-wrapper follow-up.

---

## Implementation phases

> Outline for review — not yet a commitment. Each phase is one focused change. **Every phase must
> ship its own tests across the three categories below; a phase is not done until its slice of the
> test matrix is green.**

### Test strategy (applies to every phase)

Three categories, each a hard gate — no phase merges with a category left as "later":

- **Correctness unit tests** (`internal/…`, in-repo): parse/build/eval invariants against
  in-memory fixtures and `fstest`. Every phase adds the unit tests for the behavior it introduces.
- **E2E integration tests** (chromedp, black-box in the `lvt` repo's `e2e/livetemplate_core_test.go`
  suite per CLAUDE.md): a real browser drives a recursive-tree page; the harness captures **browser
  console logs, server stderr, WebSocket frames, and rendered HTML** so a failure is diagnosable.
  Asserts the payoff end-to-end — a deep single-node change delivers one `["u", key, …]` frame, not
  a full re-send — and that expand/collapse/reorder at depth behave.
- **Performance benchmarks** (`go test -bench`, checked in): `BenchmarkRecursiveRender` (first
  render, O(nodes)) and `BenchmarkRecursiveUpdate` (one deep node changes) at depths/breadths
  {10, 100, 1000 nodes}, plus a wire-size assertion (updated-bytes ≪ full-render bytes). Guards the
  central claim that recursion is O(nodes) render + near-dynamics-only wire, and catches regressions
  vs. today's opaque full-string re-send as a baseline row.

### Core library

- [x] **Phase 1 — Defensive cycle guard (independently shippable). DONE.** Added an active-path
      cycle check (`checkFlattenCycle`) to `walkAndFlatten` (`internal/parse/flatten.go`): a
      `{{template}}` whose name is already being inlined on the current path returns a
      `ParseError` naming the cycle (`treeNode -> treeNode`, `a -> b -> a`) instead of
      stack-overflowing during `Parse`. The stack is seeded with the entry point's name so a
      self-referential entry point is caught too. Preserves the cycle-name info Phase 2 needs to
      flag runtime-invoked templates. Also corrected the `template-support-matrix.md`
      "Recursive / circular template references" row (failure mode: parse-time `ParseError`, not a
      runtime infinite loop). **Unit (`internal/parse/flatten_cycle_test.go`):** the discriminating
      *diamond* test (same template invoked on non-nested paths still flattens — proves active-path,
      not global-visited), direct + mutual (`a`→`b`→`a`) + self-referential-entry cycles → `*ParseError`
      not panic, non-recursive composition unaffected. **Black-box (`recursive_template_test.go`):**
      the real `New(...).Parse(...)` path returns an error, not a crash. **Fuzz
      (`FuzzFlattenTemplate`):** seeded with self-referential sources; 370K execs, no overflow.
      **E2E:** n/a (parse-time). **Bench:** n/a.
- [x] **Phase 2 — Detection + registry (via appended `{{define}}` blocks). DONE.** Corrected the
      original "retain a reusable/compile-once sub-tree" framing: there is **no compiled sub-tree** —
      the registry holds each recursive body's **AST, re-walked per invocation** (exactly as `{{range}}`
      re-walks its item body per item; see §5). `detectRecursiveTemplates` (a DFS reachable-from-self
      pre-pass in `internal/parse/flatten.go`) flags the cycle members; `walkAndFlatten` emits their
      `{{template}}` calls **verbatim** (un-inlined) instead of recursing, and `FlattenTemplate` appends
      each flattened body as a `{{define "name"}}…{{end}}` block. On re-parse those become associated
      templates that `parse.Parse` collects into a `registry map[string]*parse.Tree` on `parse.Template`
      — **no cross-package threading**; the flattened string carries everything. `checkFlattenCycle`
      stays as a backstop: if detection ever under-identifies a cycle, the un-emitted call re-enters
      and errors cleanly instead of overflowing. **Unit (`flatten_cycle_test.go`):** recursion emitted
      verbatim + as a define (direct, mutual, self-entry), re-parses cleanly; the diamond still inlines
      3×; the backstop fires on an empty recursive set.
- [x] **Phase 3 — Runtime invocation → nested `TreeNode`. DONE.** `evaluator` gained a
      `templates map[string]*parse.Tree`; `walkAST`'s `TemplateNode` case now calls `invokeTemplate`
      (`internal/parse/invoke.go`), which **rebinds dot** to match Go exactly: `{{template "x" pipe}}`
      binds dot to the pipe value (evaluated against the caller's dot), while a no-argument
      `{{template "x"}}` binds dot to **nil** (not the caller's dot) — a nil `varCtx` gives the clean
      fresh scope, and `walkList` lazily re-inits vars if the body declares any. It re-walks the
      registered body and wraps the result via `createConditionalWrapper` (one nested dynamic slot,
      mirroring `{{if}}`). **Unit (`recursive_template_test.go` / `recursive_template_diff_test.go`):**
      nested file-tree, single-level base case, and **mutual** recursion render to exact HTML;
      direct-child edits emit granular range ops (`["a"]` append, `["i", after-id]` mid-list insert
      anchored on the preceding sibling's key, `["o"]` reorder, `["r"]` remove) carrying only the
      changed item — proving keying descends through the invocation wrapper; a between-render
      **depth-grows** change resends the new level's statics; a **deep**-descendant edit re-sends the
      enclosing branch whole (`_DescendantBranchRebuild`, per the §6 keying caveat) while leaving
      sibling branches untouched; `Execute` (initial HTTP HTML) renders recursion natively.
      **E2E/Bench:** deferred to Phase 5.
- [x] **Phase 4 — Eval-time depth guard + option. DONE.** `build.Context` gained
      `InvocationDepth`/`MaxInvocationDepth`; `invokeTemplate` increments a per-invocation `*ctx` copy
      **before** the guard check, so a Go-value **pointer cycle** in `.Children` (a short loop) still
      trips the ceiling rather than infinite-looping — the deliberate max-depth-over-visited-set
      tradeoff (Decision 2), hence configurable via `WithMaxTemplateDepth(n)` /
      `LVT_MAX_TEMPLATE_DEPTH` (default 128). Note the guard's reach differs by path: the tree-only
      path (`compat.ParseTemplateToTree`) has no other protection and our guard is sole (proven on
      infinite data); the full `buildTree` path calls html/template `Execute` first, whose own depth
      limit trips first on infinite data, and on first render an over-limit finite tree degrades to the
      HTML-structure fallback while the **update** path propagates the guard error. **Unit:** tree-path
      guard on infinite data; no-crash on the full path; `WithMaxTemplateDepth(3)` rejects a deep tree
      on update that the default renders.
- [~] **Phase 5 — Benchmarks + browser e2e + docs (partly done).** The minimal-update **proof** landed
      in Phase 3 (`_MinimalUpdate_AddChild`/`_InsertMiddle`/`_Reorder`/`_Remove`/`_DepthGrows`,
      `_DescendantBranchRebuild`), plus the depth-guard first-render leg (`_FirstRenderOverLimit`).
      - [x] **Browser e2e written + validated.** `lvt/e2e/recursive_tree_e2e_test.go`
        (`TestRecursiveTemplate_E2E`, `//go:build browser`): a full-HTML recursive file tree renders
        its complete nested structure on first load AND reactively applies a deep-branch insert through
        the **published** client — no reload, no console errors — capturing all four sources (console +
        server logs + WS frames + HTML). **PASS (1.96s)** against the C8 branch via a temporary go.work
        repoint to the worktree. This gate caught a real bug the fragment-only unit tests missed: the
        e2e first failed with a `walkAndFlatten` **stack overflow** because it was silently building
        against the *published* (pre-C8) livetemplate — proving the run genuinely exercises the
        recursion path. **Release-gated:** committed on branch `lvt/tierc-c8-recursive-e2e` (not `main`
        — lvt CI builds against published livetemplate, which overflows until C8 releases); open the PR
        in lockstep once a livetemplate release ships C8.
      - [x] `BenchmarkRecursiveRender`/`BenchmarkRecursiveUpdate` with the opaque-`template.HTML`
        baseline row (`recursive_template_bench_test.go`). **Finding (corrects the "minimal updates"
        premise):** a deep-leaf edit's update (~24 KB, the enclosing top-level branch) is
        ~equal in bytes to re-sending the whole opaque string (~23.5 KB) and ~25× the CPU — so the
        payload win is not delivered for deep, narrow trees pre-follow-up. Render is ~20-30× an opaque
        Execute. The benchmark comments and TL;DR/§6 now state this honestly; the win C8 ships is
        capability + reactive DOM-state preservation, not payload size.
      - [ ] **Release-gated docs:** flip the `template-support-matrix.md` "Recursive / circular template
        references" row from ❌ to ✅ **with the keying footnote** (deep edits re-render the enclosing
        branch; explicit `data-key` not yet honored through the invocation wrapper), update
        `current-limitations.md`, add a recipe. Matrix reflects *released* behavior, so this lands with
        the release, not before.
- [ ] **Follow-up — honor explicit `data-key` through the invocation wrapper.** So a deep-descendant
      edit scopes down to a single per-leaf `["u", key, {…}]` instead of re-sending the enclosing
      branch. The naive unwrap (exposing the item's `<li>` as its top-level tree) regressed the
      deep-edit case to a full range re-send via an un-diagnosed diff-engine path — so this needs a
      diagnosed diff-engine fix, tracked as its own issue. Not a C8 blocker (renders are correct;
      this is update size).

### Companion migrations (dependent repos — land per the lockstep convention once a release ships)

- [ ] **Phase 6 — prereview: remove the escape hatch (true workaround removal).** Replace
      `FileBrowserHTML`/`fileBrowserTmpl` (`internal/review/filetree.go`) with a native recursive
      `{{template "treeNode"}}` inside the reactive `prereview.tmpl`, deleting the standalone
      `html/template` and the zero-arg `template.HTML` method. **Acceptance:** the file tree now
      receives incremental `["u", key, …]` updates (verify via WS-frame capture in prereview's e2e
      harness that selecting a deep file no longer re-sends the whole tree); existing prereview e2e
      stays green; a benchmark/measurement of update bytes before/after documents the win.
- [ ] **Phase 7 — tinkerdown: make the sidebar nav reactive (latent case → real consumer).**
      Migrate `writeNavNode` (recursive-Go `<details>` string building, `internal/server/server.go`)
      to a native recursive livetemplate `{{template}}`. **Note the honest scope:** tinkerdown
      currently serves pages as static HTML (`servePage`'s *"Add WebSocket support for interactivity"*
      TODO), so this phase also entails routing the nav through a livetemplate reactive tree — a
      larger change than prereview's, and optional to the core feature. It earns its place by
      proving recursion on a *second, independently-authored* app and retiring the hand-rolled
      recursive-Go renderer. **Acceptance:** nav renders identically to the `writeNavNode` output
      (visual-regression screenshot parity), and a nav change (e.g. active-page move) produces a
      targeted update; tinkerdown e2e green. If reactive nav proves out of scope for tinkerdown's
      roadmap, this phase degrades to "tinkerdown *could* adopt it" and is dropped without blocking
      C8 — the core feature stands on Phase 6.

---

## Scope & risk

- **Surfaces touched:** `internal/parse/` (cycle detection, runtime-boundary emission),
  a name→subtree registry on the compiled `Template`, `internal/build`/eval (instantiate sub-tree
  → nested `TreeNode`), an eval-time depth guard. `internal/build/fingerprint.go` is **reused
  as-is** — the existing per-instance lazy cache already covers deep recursion (§5); no per-name
  cache is added. `internal/diff` and the **client are unchanged** (reuses nested-tree wire format).
  Tests span the core repo (unit + benchmarks), the `lvt` repo (chromedp e2e per CLAUDE.md), and —
  for the companion migrations — the **prereview** and **tinkerdown** repos (Phases 6–7).
- **Cross-repo work:** prereview (Phase 6, escape-hatch removal — a genuine simplification) and
  tinkerdown (Phase 7, nav→reactive — a larger, optional migration). Both land per the lockstep
  convention *after* a livetemplate release ships the feature; neither blocks the core phases.
- **Backward compatibility:** high. The selective split means non-recursive composition keeps the
  existing flatten path byte-for-byte; only cyclic invocation graphs take the new path.
- **Biggest risk:** the eval-time sub-tree instantiation is new plumbing. Mitigated by leaning on
  the range item-template model, which already does structurally the same thing.
- **Performance:** deep trees render O(nodes); positionally-cached statics keep the wire cost
  near dynamics-only after first render — strictly better than today's full-string re-send.

## Decisions

The three headline questions this proposal solicited are **decided** (maintainer sign-off,
2026-07-15). *(Items keep their original open-question numbers; decided ones are pulled up here,
so the numbering below is 1/2/4 and § Remaining design notes keeps 3/5/6.)*

1. ✅ **Selective, not uniform.** Convert *only* recursive invocations to runtime boundaries; keep
   flattening non-recursive ones. Uniform runtime invocation is conceptually cleaner but a much
   larger blast radius and a perf regression for the common flat case — not worth it.
2. ✅ **Max-depth guard (`WithMaxTemplateDepth`, default 128).** A configurable **max-depth**
   (simpler and a clearer error than a pointer-visited set), incremented before the check so a
   short pointer cycle still trips it. **Resolved with a known path divergence** (see Phase 4): the
   guard fires in `walkAST`, but exceeding it on the **update** path propagates a clean depth error,
   while on **first render** it degrades to the HTML-structure fallback (the framework's general
   AST-build-failure behavior) rather than erroring. Infinite data on the full `buildTree` path
   trips html/template `Execute`'s own depth limit first; the tree-only path
   (`compat.ParseTemplateToTree`) relies solely on this guard. The uniform-hard-error goal was
   dropped as it would require special-casing depth errors out of the shared fallback — acceptable
   for alpha; revisit if the divergence proves confusing.
4. ✅ **`data-key` falls back like `{{range}}`.** Do *not* require an explicit `data-key` inside a
   runtime-invoked template; fall back to content-hash keys exactly as `{{range}}` does today.
   **Implemented reality (see §6 keying caveat):** a recursive range item is a `{{template}}`
   invocation whose wrapper hides its `<li data-key>`, so keying *always* uses the content hash today
   — the explicit `data-key` is not yet honored through the wrapper. Identity is still exact (the
   `data-key` value is a hashed dynamic), and direct-child add/insert/reorder/remove stay granular;
   the only cost is that a *deep* edit re-sends the enclosing branch. Making an explicit `data-key`
   actually take effect (and scope deep edits to the leaf) is the tracked follow-up.

## Remaining design notes

These are implementation invariants / scope boundaries, not open decisions:

3. **Mutual recursion / indirect cycles** (`a`→`b`→`a`). The design handles it (the cycle set can
   contain multiple names); confirm test coverage during Phase 1/3.
5. **Depth-change ⇒ position-distinct nested nodes.** The "no client changes" result (§6) rests on
   the eval producing genuinely position-distinct nested nodes at each recursion level, so the
   existing per-position `ClientNeedsStatics(nil, new)` fires for a *newly-materialized* level and
   emits its statics. This is a Phase 3 correctness invariant, not a client concern — the design
   assumes it and Phase 3's depth-1..N parity tests must exercise depth *growth and shrink between
   renders*, not just static depths. Confirm the eval reuses the range item-instantiation path,
   which already assigns position-distinct child nodes.
6. **Dynamic invocation** (`{{template (printf …) .}}`) remains a documented fallback
   (`current-limitations.md:15,19`); recursion support does not change that — out of scope.
