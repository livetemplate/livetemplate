# Plan: M3 — replace DOM-watching workarounds, then decide on per-element hooks (#473)

Phase 6 of [issue-473.md](issue-473.md) adds per-element hooks only if an app needs them. This plan tests that need. Each app script that watches the DOM gets moved to the tool that fits it, and Phase 6 is designed only if a livetemplate-rendered case can't be expressed without per-element state.

### State

Done: nothing. Next: Phase A audit.

### Progress tracker

| Phase | What | Repos | Status |
|---|---|---|---|
| A | Form values after a patch (`data-expected-value`) | client, lvt | ☐ |
| B | Toast auto-dismiss as a registry handler | lvt | ☐ |
| C | Mermaid diagrams as a registry handler | prereview | ☐ |
| D | tinkerdown toolbar: a direct call | tinkerdown | ☐ |
| E | The gate: design Phase 6 or close it | livetemplate | ☐ |

## Decisions

- Each case gets the tool that matches where its element comes from: a core client fix, a registry handler, or a direct call. A registry handler only sees elements inside a livetemplate wrapper.
  alternative: move all four to registry handlers
  because: two of the four aren't app behaviour. The select sync works around the client, and tinkerdown's sidebar is never rendered by livetemplate
- **A:** first, a chromedp test checks whether the select or the search input still loses the server's value after a patch. If it does, fix it in the client, beside the checkbox and radio fix from client v0.25.10 (`lastRenderedChecked` in `livetemplate-client.ts`). Then delete `data-expected-value` and `syncFormValues` from every lvt template. If the test passes without the workaround, only delete it.
  alternative: a registry handler on `data-expected-value`
  because: every app with a server-driven `<select>` has the same gap, so the fix belongs in the client
- **B:** the handler keeps the attribute name `data-auto-dismiss`. It is declarative: `onElementAdded` arms the timer, and `ctx.signal` clears it.
  alternative: rename to `lvt-x:auto-dismiss`, so the census warns when no handler is loaded
  because: the name is rendered by the published `lvt/components/toast`, so a rename would need a components release and new goldens in all three kits. The registry accepts any valid attribute name
- **B:** the handler stays where the inline script is now, in each kit's layout and resource template.
  alternative: ship it with `lvt/components/toast`
  because: none considered further until the Phase B audit shows whether components ship any script
- **C:** prereview's diagrams get a declarative handler on a new `lvt-x:mermaid` attribute. Its value is the existing `mmd-<hash>` key, so it is never empty. `onElementAdded` renders the diagram, which replaces the `data-mmd-claimed` bookkeeping and the `lvt:updated` sweep.
  alternative: keep the `lvt:updated` sweep, a public client event
  because: the sweep re-queries the whole document after every patch, and the handler sees each new diagram once
- **C:** a changed diagram needs no `updated` callback. `MermaidHTML` hashes the definition into `data-key`, so a changed diagram arrives as a new element.
- **D:** tinkerdown's `createSidebar()` (`client/src/core/navigation.ts`) moves the toolbar itself, and the `MutationObserver` in `server.go` goes.
  alternative: none considered
- **E:** Phase 6 gets designed only if Phase A, B or C finds a livetemplate-rendered element whose behaviour needs state that survives a patch of that element. Phase D says nothing about hooks either way.

## Files to modify

- client: `livetemplate-client.ts` (Phase A, only if the test fails), `CHANGELOG.md`
- lvt: `internal/kits/system/{single,multi,simple}/components/layout.tmpl`, `internal/kits/system/{single,multi}/templates/resource/template.tmpl.tmpl`, `internal/generator/templates/components/layout.tmpl`, the `toolbar.tmpl`, `sort.tmpl` and resource templates that carry `data-expected-value`, `testdata/golden/resource_template.tmpl.golden`, `e2e/` (new? a test per phase)
- prereview: `gitdiff/mermaid.go`, `internal/assets/mermaid-init.js` (rewritten as a handler, or moved into `handlers/`), `templates/page.tmpl`, `e2e/` (new? a diagram test)
- tinkerdown: `client/src/core/navigation.ts`, `internal/server/server.go`, the bundled `internal/assets/client/tinkerdown-client.browser.js`
- livetemplate: `docs/plans/issue-473.md` (tracker row 6 and its State point here)

## Assumptions to verify before building

- [ ] The registry scans an `lvt-ignore` subtree, so it sees a new `.md-mermaid` element (Phase C). Phase 6 lists hooks inside `lvt-ignore` as an open question, so record the answer either way
- [ ] `page.tmpl` keeps `mermaid-init.js` standalone for a reason ("keeps it out of livetemplate's template tree") that a handler file in `handlers/` also meets
- [ ] The lvt kits load a client recent enough for `window.lvtAttributes` and `ctx.signal` (client ≥ 0.25.6)
- [ ] Every lvt copy of the two scripts is listed: `grep -rlE 'syncFormValues|data-expected-value|data-auto-dismiss' lvt/internal` (12 files on 2026-11-24)

## Acceptance criteria

Each phase opens with an Audit, and its first acceptance criterion is `/simplify`. Every chromedp test captures console logs, server logs, WebSocket frames and the rendered HTML.

- [ ] **A:** an lvt e2e test changes the sort and clears the search with the workaround removed. Each control then shows the server's value. It fails before any client fix and passes after
- [ ] **A:** `grep -rE 'syncFormValues|data-expected-value' lvt/internal` returns nothing
- [ ] **B:** an lvt e2e test shows a toast with `data-auto-dismiss` disappear after its delay, and a toast removed early leaves no timer that clicks a missing button
- [ ] **B:** `grep -rE 'MutationObserver' lvt/internal/kits` returns nothing
- [ ] **C:** a prereview e2e test renders a diagram, edits its source, and sees the new diagram rendered with no console error
- [ ] **D:** tinkerdown's tutorial-mode e2e test finds the toolbar inside the sidebar, and `server.go` has no `MutationObserver`
- [ ] **E:** tracker row 6 in `issue-473.md` reads either "designing" with the case that needs hooks, or "closed: no demand" with this plan linked

## Constraints

- Each repo ships its own PR and release through its `release.sh`. lvt's goldens change in the same PR as its templates.
- The client fix in Phase A ships as a client patch, then a core patch adopts it, as in Phase 4 of `issue-473.md`.

## Risks you'd act on

- Phase A's client fix touches the same patch hook as the checkbox and radio fix. A form control the user is editing must keep the user's value. Reuse the "server rendered a different value than last time" rule rather than a new one.
