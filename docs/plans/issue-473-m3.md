# Plan: M3 — replace DOM-watching workarounds, then decide on per-element hooks (#473)

Phase 6 of [issue-473.md](issue-473.md) adds per-element hooks only if an app needs them. This plan tests that need. Each app script that watches the DOM gets moved to the tool that fits it, or deleted when it is dead. Phase 6 is designed only if a livetemplate-rendered case can't be expressed without per-element state.

## State

Done: nothing. Next: Phase A audit, once this plan has signoff.

## Progress tracker

| Phase | What | Repos | Status |
|---|---|---|---|
| A | Mermaid diagrams as a registry handler | prereview | ☐ |
| B | Toast auto-dismiss: delete a dead script | lvt | ☐ |
| C | Form values after a patch (`data-expected-value`) | client, lvt | ☐ |
| D | tinkerdown toolbar: a direct call | tinkerdown | ☐ |
| E | The gate: design Phase 6 or close it | livetemplate | ☐ |

## Decisions

- The one registry migration runs first (A). The toast deletion (B) comes next because it is small, then the client question (C), then tinkerdown (D).
  alternative: form values first
  because: A is the one case that tests the registry, and C may end with only a deletion
- Each case gets the tool that matches where its element comes from: a core client fix, a registry handler, or a direct call. A registry handler only sees elements inside a livetemplate wrapper.
  alternative: move all four to registry handlers
  because: three of the four aren't app behaviour. The select sync works around the client, the toast script targets markup nothing renders, and tinkerdown's sidebar is never rendered by livetemplate
- **A:** prereview's diagrams get a declarative handler on a new `lvt-x:mermaid` attribute. Its value is the existing `mmd-<hash>` key, so it is never empty. `onElementAdded` renders the diagram, which replaces the `data-mmd-claimed` bookkeeping and the `lvt:updated` sweep. A failed render keeps today's fallback, which leaves the source visible, and logs one `console.warn` naming the diagram's key. The same holds when `loadMermaid()` rejects, for example on a network error. That diagram stays as source until its definition changes or the page reloads; the prereview PR lists this as a known limitation. The handler keeps today's lazy `loadMermaid()` promise and renders when it resolves, so a diagram armed before the library loads waits instead of failing. It isn't retried: `onElementAdded` fires once per arming, so after a bad definition the next attempt comes when the definition changes. The handler lives in prereview's `handlers/src/`, bundled into `/prereview-handlers.js`, which `page.tmpl` already loads, and `mermaid-init.js` goes.
  alternative: keep the `lvt:updated` sweep, a public client event
  because: the sweep re-queries the whole document after every patch, and the handler sees each new diagram once. Only the listener goes: the client still fires `lvt:updated`, and `mermaid-init.js` is prereview's only listener (`grep -rn 'lvt:updated'` on 2026-11-24)
- **A:** a changed diagram needs no `updated` callback. `MermaidHTML` hashes the definition into `data-key`, so a changed diagram arrives as a new element.
- **B:** delete the `[data-toast][data-auto-dismiss]` script from the kit layouts and resource templates, with no replacement. Nothing renders that markup: since lvt #290 (2026-04-02) the client builds toasts from `data-toast-trigger` and removes each after its `dismissMS` (`client/dom/directives.ts`).
  alternative: migrate it to a registry handler, as this plan first proposed
  because: a handler for markup nobody renders would only move dead code
- **C:** first, a chromedp test checks whether the select or the search input still loses the server's value after a patch. If it does, fix it in the client, beside the checkbox and radio fix from client v0.25.10 (`lastRenderedChecked` in `livetemplate-client.ts`). Then delete `data-expected-value` and `syncFormValues` from every lvt template. If the test passes without the workaround, only delete it.
  alternative: a registry handler on `data-expected-value`
  because: every app with a server-driven `<select>` has the same gap, so the fix belongs in the client
- **D:** tinkerdown's `createSidebar()` (`client/src/core/navigation.ts`) moves the toolbar itself, and the `MutationObserver` in `server.go` goes.
  alternative: a registry handler on the sidebar
  because: the registry only sees livetemplate-rendered elements, and the code that creates the sidebar already knows when it exists
- **E:** Phase 6 gets designed only if Phase A or C finds a livetemplate-rendered element whose behaviour needs state that survives a patch of that element. Phase D says nothing about hooks either way.

## Files to modify

- client: `livetemplate-client.ts` (Phase C, only if the test fails), `CHANGELOG.md`
- lvt, the 12 files `grep -rlE 'syncFormValues|data-expected-value|data-auto-dismiss' internal` lists on 2026-11-24:
  - `internal/kits/system/{single,multi,simple}/components/layout.tmpl` (both scripts)
  - `internal/kits/system/{single,multi}/components/{sort,toolbar}.tmpl`, `internal/generator/templates/components/{sort,toolbar}.tmpl` (`data-expected-value`)
  - `internal/kits/system/{single,multi}/templates/resource/template.tmpl.tmpl`, `internal/generator/templates/resource/template.tmpl.tmpl` (both)
- lvt, also: `testdata/golden/resource_template.tmpl.golden`; `e2e/form_value_sync_test.go` (new, Phase C)
- prereview: `gitdiff/mermaid.go` (adds `lvt-x:mermaid`), `handlers/src/mermaid.ts` (new) and `handlers/src/index.ts`, `internal/assets/mermaid-init.js` and its embed in `internal/assets/assets.go` (removed), the `/mermaid-init.js` route in `server.go`, `templates/page.tmpl`, `e2e/e2e_mermaid_test.go` (gains the edit and invalid-definition cases)
- tinkerdown: `client/src/core/navigation.ts`, `internal/server/server.go`, the bundled `internal/assets/client/tinkerdown-client.browser.js` (rebuilt from `client/src`, never edited by hand), `tutorial_toolbar_e2e_test.go` (new)
- livetemplate: `docs/plans/issue-473.md` (tracker row 6 and its State point here)

## Assumptions to verify before building

- [x] The registry sees a new `.md-mermaid` element (Phase A). **2026-11-24, from the client source:** `lvt-ignore` is checked only in `onBeforeElUpdated`, so it stops morphdom updating an existing element and nothing else. `attribute-registry.ts` never mentions it. `getNodeKey` matches on `data-key`, so a changed diagram (new `mmd-<hash>`) is removed and re-inserted, never updated in place. Phase A's e2e test confirms it in a browser
- [x] `page.tmpl` keeps `mermaid-init.js` standalone to keep it "out of livetemplate's template tree". A handler in `handlers/` meets that too: `page.tmpl` already loads `/prereview-handlers.js` as a separate `<script>`
- [x] The lvt kits load a client recent enough for `window.lvtAttributes` and `ctx.signal` (client ≥ 0.25.6). They serve the client embedded in core, and lvt pins core v0.25.15, whose `ClientVersion` is 0.25.12
- [x] Every lvt copy of the two scripts is listed under § Files, from the grep there

## Acceptance criteria

Each phase opens with an Audit that re-runs the § Files grep and re-checks the version pins in § Assumptions, and its first acceptance criterion is `/simplify`. Every chromedp test captures console logs, server logs, WebSocket frames and the rendered HTML.

- [ ] **A:** a prereview e2e test opens a page with a diagram in its first render and sees it drawn. It then renders a diagram, edits its source, and sees the new diagram rendered with no console error. An invalid definition leaves its source visible and logs one warning
- [ ] **B:** `grep -rE 'data-auto-dismiss' lvt --exclude-dir=.worktrees` returns nothing, and `TestToastAutoDismiss` in `e2e/embedded_browser_test.go` still passes
- [ ] **C:** an lvt e2e test changes the sort to a value other than the default and clears the search, then checks that each control shows the server's value. The test lands in its own commit before any template change, and runs in three steps; the PR records the results:
  1. With the workaround in place, it must pass. That proves the test sees what the workaround hides.
  2. With the workaround removed and no client change, it asserts each control's value: a pass means the phase only deletes the workaround; a failure means the client has the gap.
  3. After a client fix, it must pass.
  A patch that arrives while the user has the select focused keeps the user's choice, and so does one that arrives while the user is typing in the search input (both cases in `form_value_sync_test.go`)
- [ ] **C:** `grep -rE 'syncFormValues|data-expected-value' lvt --exclude-dir=.worktrees` returns nothing, goldens and e2e included
- [ ] **D:** tinkerdown's tutorial-mode e2e test finds the toolbar inside the sidebar, and `server.go` has no `MutationObserver`
- [ ] **E:** tracker row 6 in `issue-473.md` reads either "designing" with the case that needs hooks, or "closed: no demand in lvt, prereview or tinkerdown" with this plan linked. A later app that needs hooks reopens Phase 6 with a new issue that names its case. Only Phases A and C count toward the gate. Phase A ending with a working registry handler is itself evidence of no demand, and "no demand" covers only the apps this plan audited

## Constraints

- Each repo ships its own PR and release through its `release.sh`. lvt's goldens change in the same PR as its templates.
- The client fix in Phase C ships as a client patch, then a core patch adopts it, as in Phase 4 of `issue-473.md`.

## Risks you'd act on

- Phase C's client fix touches the same patch hook as the checkbox and radio fix. A form control the user is editing must keep the user's value. Reuse the "server rendered a different value than last time" rule rather than a new one.
