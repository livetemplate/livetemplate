# Recursive `{{template}}` Support

**Status:** Proposed
**Tracking:** Tier C item C8 of the boilerplate-reduction pass (issue #483)
**Audience:** livetemplate maintainers — a design to review before any implementation.

## TL;DR

**Problem.** livetemplate inlines every `{{template "name" .}}` invocation into one flat
template at parse time. A self-referential template (a folder tree, a nav tree, a threaded
comment list) expands forever and **stack-overflows during `Parse`**. Two independent apps
(prereview, tinkerdown) work around it by rendering the recursive part with standalone
`html/template` and injecting the result as opaque `template.HTML` — which **removes the whole
subtree from livetemplate's reactive diffing**: the entire tree is re-rendered and re-sent on
every keystroke, defeating the framework's core value.

**Solution (recommended).** Stop inlining *recursive* invocations. Detect a self-referential
invocation graph at parse time (the guard that's missing today), and evaluate those invocations
at **runtime as a nested `TreeNode`** — the same mechanism `{{range}}` already uses. Recursion
then terminates on the **data** (a finite tree of nodes), not on parse-time text expansion, and
each level is a first-class diffable subtree. Non-recursive invocations keep flattening
unchanged (zero regression risk to existing composition).

**Non-goal.** This is not "arbitrary Go template recursion at any cost." It's "let a template
call itself over a finite data structure and still get minimal updates."

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

There is **no explicit "recursion unsupported" marker** in the docs; the support matrix
(`template-support-matrix.md:174-179`) simply says composition is "automatically flattened,"
which silently assumes a finite, acyclic invocation graph.

### The real cost: opaque HTML defeats reactive diffing

Both apps that need recursion escape-hatch identically. prereview
(`internal/review/filetree.go`) is the clearest:

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

### Evidence (≥2 independent consumers)

- **prereview** — `internal/review/filetree.go` + `filetree.tmpl` (`treeNode` recurses on
  `.Children`; native `<details>`; injected as `template.HTML`).
- **tinkerdown** — nested nav built recursively (`internal/site/manager.go:115`,
  `buildNavPageNode` recurses into children), rendered via the same native-`<details>` opaque-HTML
  approach prereview's comment cross-references.

This is the one Tier C item with two independent hand-rolled consumers *and* a genuine capability
gap (not a discoverability or docs gap) — which is why it earns a real implementation rather than
a doc note.

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
  invocation; needs an eval-time depth/cycle guard for pathological data.

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

### 2. Retain named templates as pre-built sub-trees

Today flattening discards the separate `{{define}}`s (it inlines them). For runtime invocation,
the framework must **keep** each runtime-invoked template's parsed body and build it into a
reusable sub-tree "template" (mirroring how a `{{range}}` item body is compiled once and
instantiated per item). Store these in a name→subtree registry on the compiled `Template`.

### 3. Evaluate: instantiate the sub-tree per invocation → nested `TreeNode`

When evaluation reaches a runtime invocation boundary:

1. Resolve the invoked name against the registry (step 2).
2. Evaluate that sub-tree against the invocation's **pipe value** as the new dot. `$` naturally
   rebinds to that dot per invocation — matching Go's rule that a template calling itself drops
   the caller's `$` (prereview relies on this: it resolves per-node display state at build time so
   the recursive body never needs the root `$`).
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

### 5. Fingerprint: reuse per template, don't re-hash per level

A recursive structure repeats the *same* statics at every level, so the invoked template's static
fingerprint is **identical** at every node. Fingerprint the runtime-invoked sub-tree **once** (by
template name) and reuse it, rather than re-hashing O(depth) times. The existing cycle guard in
`fingerprint.go` already makes a cyclic `TreeNode` graph well-defined; this optimization keeps
deep *acyclic* trees O(nodes) with a tiny constant.

This is a **server-side** hashing optimization only — the client never sees fingerprints
(§6). Its user-visible payoff is statics *suppression*: at **stable depth**, the per-position
`ClientNeedsStatics` finds an unchanged node and sends dynamics-only. This is not free at
*changing* depth: a newly-appeared level is position-new, so its statics are (correctly) sent
once. The self-similarity is a hashing win, not a wire-cost guarantee across depth changes.

### 6. Wire format & diffing — no new client ops

Each level serializes as a nested tree: statics sent on first render, dynamics-only on update.
Because every node carries a stable identity (prereview already emits `data-key`), a change deep
in the tree produces a targeted `["u", key, changes]` update — not a full re-send. **No new wire
operations and no client changes** — verified, not assumed, against the client source:

- **The client has no fingerprint concept at all.** Fingerprints are server-only: the server uses
  them to *decide whether to send statics* (`ClientNeedsStatics`), and the wire never carries them
  (`grep fingerprint` over `github.com/livetemplate/client` returns zero hits, as of client
  v0.16.5). So there is no fingerprint-keyed client cache that identical-per-level structure could
  collide in — the concern that self-similar levels confuse the client does not arise.
- **The client caches statics *positionally* and merges depth-agnostically.** `TreeRenderer`
  (`client/state/tree-renderer.ts`) stores statics inline at each tree path in `treeState`, and
  `deepMergeTreeNodes` recurses on object nesting keyed by a path string with **no depth cap**.
  Unbounded data-driven recursion is structurally identical to arbitrarily-deep nested ranges,
  which the client already applies — so no client change is needed to *apply* the updates.

The one design obligation this puts on the **server** eval (not the client): when recursion depth
*changes* between renders (a node gains or loses children), each newly-materialized level is a
tree position that had no prior node, so the existing per-position `ClientNeedsStatics(nil, new)`
must fire and emit that level's statics. The recursive eval must therefore produce genuinely
position-distinct nested nodes so this per-position logic engages — see Open question 5.

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
- User selects a file 4 levels deep: only that node's dynamic (its `data-key` row) diffs → a
  single `["u", "<path>", {…}]` reaches the client. Today: the entire `{{.FileBrowserHTML}}`
  string re-sends.

---

## Implementation phases

> Outline for review — not yet a commitment. Each phase is one focused change with its own tests.

- [ ] **Phase 1 — Defensive cycle guard (independently shippable).** Add visited-set/depth
      detection to `walkAndFlatten` so a self-referential template returns a clear
      `ParseError` ("recursive template requires runtime invocation; see …") instead of
      stack-overflowing. Ships value on its own (crash → diagnosable error) and lays the parse-time
      detection Phase 2 builds on. Tests: direct + indirect cycle → error, not panic.
- [ ] **Phase 2 — Retain runtime-invoked templates as sub-trees.** Build + register the named
      body as a reusable sub-tree for templates flagged recursive in Phase 1. No rendering yet;
      unit-test the registry + sub-tree build.
- [ ] **Phase 3 — Runtime invocation → nested `TreeNode`.** Wire eval to instantiate the sub-tree
      per invocation against the pipe dot and splice the nested tree. Tests: a recursive fixture
      renders identically to the equivalent stdlib `html/template` output at depths 1..N, plus a
      *between-render depth change* (grow and shrink) asserting a newly-materialized level receives
      its statics and a removed one is dropped — the Open-question-5 invariant.
- [ ] **Phase 4 — Fingerprint reuse + eval-time depth guard.** Per-name fingerprint caching;
      max-depth option with a clear error. Tests: deep tree stays O(nodes); cyclic data errors
      cleanly.
- [ ] **Phase 5 — Minimal-update proof + docs.** Assert a deep single-node change emits one
      `["u", key, …]` and not a full re-send (the payoff). Update `template-support-matrix.md` and
      `current-limitations.md`; add a recipe. **Companion:** migrate prereview's `filetree.go`
      off the standalone-template escape hatch onto a native recursive `{{template}}` (proves the
      boilerplate is actually removed; lands per the lockstep convention once a release ships).

---

## Scope & risk

- **Surfaces touched:** `internal/parse/` (cycle detection, runtime-boundary emission),
  a name→subtree registry on the compiled `Template`, `internal/build`/eval (instantiate sub-tree
  → nested `TreeNode`), `internal/build/fingerprint.go` (per-name reuse), an eval-time depth
  guard. `internal/diff` and the **client are unchanged** (reuses nested-tree wire format).
- **Backward compatibility:** high. The selective split means non-recursive composition keeps the
  existing flatten path byte-for-byte; only cyclic invocation graphs take the new path.
- **Biggest risk:** the eval-time sub-tree instantiation is new plumbing. Mitigated by leaning on
  the range item-template model, which already does structurally the same thing.
- **Performance:** deep trees render O(nodes); positionally-cached statics keep the wire cost
  near dynamics-only after first render — strictly better than today's full-string re-send.

## Open questions

1. **Selective vs. uniform.** Recommended: convert *only* recursive invocations to runtime
   boundaries (keep flattening non-recursive ones). Uniform runtime invocation is conceptually
   cleaner but a much larger blast radius and a perf regression for the common flat case. Confirm
   selective.
2. **Depth guard: max-depth vs. pointer-visited.** Recommended max-depth (simpler, clearer error).
   Default value + option name to settle.
3. **Mutual recursion / indirect cycles** (`a`→`b`→`a`). The design handles it (the cycle set can
   contain multiple names), but confirm test coverage expectations.
4. **`data-key` requirement.** Targeted updates need stable node identity. Should the framework
   *require* a `data-key` inside a runtime-invoked template (error if absent), or fall back to
   content-hash keys as `{{range}}` does today? Recommend: fall back like range, document that
   explicit `data-key` is strongly preferred for large trees.
5. **Depth-change ⇒ position-distinct nested nodes.** The "no client changes" result (§6) rests on
   the eval producing genuinely position-distinct nested nodes at each recursion level, so the
   existing per-position `ClientNeedsStatics(nil, new)` fires for a *newly-materialized* level and
   emits its statics. This is a Phase 3 correctness invariant, not a client concern — the design
   assumes it and Phase 3's depth-1..N parity tests must exercise depth *growth and shrink between
   renders*, not just static depths. Confirm the eval reuses the range item-instantiation path,
   which already assigns position-distinct child nodes.
5. **Dynamic invocation** (`{{template (printf …) .}}`) remains a documented fallback
   (`current-limitations.md:15,19`); recursion support does not change that — out of scope.
