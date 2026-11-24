# Plan: M3 — replace DOM-watching workarounds, then decide on per-element hooks (#473)

Phase 6 of [issue-473.md](issue-473.md) adds per-element hooks only if an app needs them. This plan tests that need. Each app script that watches the DOM gets moved to the tool that fits it, and Phase 6 is designed only if a livetemplate-rendered case can't be expressed without per-element state.

### State

Done: nothing. Next: Phase A audit, once this plan has signoff.

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
- **B:** the handler keeps the attribute name `data-auto-dismiss`. It is declarative: `onElementAdded` arms the timer, and `ctx.signal` clears it. The signal aborts when the element is detached or loses the attribute, not only at client teardown (`ElementContext.signal` in `attribute-registry.ts`). A patch that keeps the attribute leaves the timer running, because `onElementAdded` fires once per arming. That matches today's script, which keys its timers by `data-toast` id. A replaced node is a new arming and starts a new timer, where today's script would keep the old one. That's acceptable, because a toast keeps its `data-key` across renders.
  alternative: rename to `lvt-x:auto-dismiss`, so the census warns when no handler is loaded
  because: the name is rendered by the published `lvt/components/toast`, so a rename would need a components release and new goldens in all three kits. The registry accepts any valid attribute name
- **B:** the handler stays where the inline script is now, in each kit's layout and resource template.
  alternative: ship it with `lvt/components/toast`
  because: no lvt component ships a script (`grep -rl '<script' lvt/components` is empty on 2026-11-24), so this would add the first component asset pipeline for one handler
- **C:** prereview's diagrams get a declarative handler on a new `lvt-x:mermaid` attribute. Its value is the existing `mmd-<hash>` key, so it is never empty. `onElementAdded` renders the diagram, which replaces the `data-mmd-claimed` bookkeeping and the `lvt:updated` sweep. A failed render keeps today's fallback, which leaves the source visible, and logs one `console.warn` naming the diagram's key. The handler lives in prereview's `handlers/src/`, bundled into `/prereview-handlers.js`, which `page.tmpl` already loads, and `mermaid-init.js` goes.
  alternative: keep the `lvt:updated` sweep, a public client event
  because: the sweep re-queries the whole document after every patch, and the handler sees each new diagram once
- **C:** a changed diagram needs no `updated` callback. `MermaidHTML` hashes the definition into `data-key`, so a changed diagram arrives as a new element.
- **D:** tinkerdown's `createSidebar()` (`client/src/core/navigation.ts`) moves the toolbar itself, and the `MutationObserver` in `server.go` goes.
  alternative: none considered
  because: the code that creates the sidebar knows when it exists, so nothing needs to watch for it
- **E:** Phase 6 gets designed only if Phase A, B or C finds a livetemplate-rendered element whose behaviour needs state that survives a patch of that element. Phase D says nothing about hooks either way.

## Files to modify

- client: `livetemplate-client.ts` (Phase A, only if the test fails), `CHANGELOG.md`
- lvt, the 12 files `grep -rlE 'syncFormValues|data-expected-value|data-auto-dismiss' internal` lists on 2026-11-24:
  - `internal/kits/system/{single,multi,simple}/components/layout.tmpl` (both scripts)
  - `internal/kits/system/{single,multi}/components/{sort,toolbar}.tmpl`, `internal/generator/templates/components/{sort,toolbar}.tmpl` (`data-expected-value`)
  - `internal/kits/system/{single,multi}/templates/resource/template.tmpl.tmpl`, `internal/generator/templates/resource/template.tmpl.tmpl` (both)
- lvt, also: `testdata/golden/resource_template.tmpl.golden`; `e2e/form_value_sync_test.go` (new, Phase A); `e2e/embedded_browser_test.go` (`TestToastAutoDismiss` gains the early-removal and patched-toast cases)
- prereview: `gitdiff/mermaid.go` (adds `lvt-x:mermaid`), `handlers/src/mermaid.ts` (new) and `handlers/src/index.ts`, `internal/assets/mermaid-init.js` and its embed in `internal/assets/assets.go` (removed), the `/mermaid-init.js` route in `server.go`, `templates/page.tmpl`, `e2e/e2e_mermaid_test.go` (gains the edit and invalid-definition cases)
- tinkerdown: `client/src/core/navigation.ts`, `internal/server/server.go`, the bundled `internal/assets/client/tinkerdown-client.browser.js`, `tutorial_toolbar_e2e_test.go` (new)
- livetemplate: `docs/plans/issue-473.md` (tracker row 6 and its State point here)

## Assumptions to verify before building

- [x] The registry sees a new `.md-mermaid` element (Phase C). **2026-11-24, from the client source:** `lvt-ignore` is checked only in `onBeforeElUpdated`, so it stops morphdom updating an existing element and nothing else. `attribute-registry.ts` never mentions it. `getNodeKey` matches on `data-key`, so a changed diagram (new `mmd-<hash>`) is removed and re-inserted, never updated in place. Phase C's e2e test confirms it in a browser
- [x] `page.tmpl` keeps `mermaid-init.js` standalone to keep it "out of livetemplate's template tree". A handler in `handlers/` meets that too: `page.tmpl` already loads `/prereview-handlers.js` as a separate `<script>`
- [x] The lvt kits load a client recent enough for `window.lvtAttributes` and `ctx.signal` (client ≥ 0.25.6). They serve the client embedded in core, and lvt pins core v0.25.15, whose `ClientVersion` is 0.25.12
- [x] Every lvt copy of the two scripts is listed under § Files, from the grep there

## Acceptance criteria

Each phase opens with an Audit that re-runs the § Files grep and re-checks the version pins in § Assumptions, and its first acceptance criterion is `/simplify`. Every chromedp test captures console logs, server logs, WebSocket frames and the rendered HTML.

- [ ] **A:** an lvt e2e test changes the sort to a value other than the default and clears the search, and each control shows the server's value. It runs first with the workaround in place, where it must pass, which proves the test sees the bug the workaround hides. Then it runs with the workaround removed and no client change: a failure there means the client has the gap. If it fails at first, it passes after the client fix; if it passes at first, the phase only deletes the workaround. The PR records which happened. A patch that arrives while the user has the select focused keeps the user's choice
- [ ] **A:** `grep -rE 'syncFormValues|data-expected-value' lvt --exclude-dir=.worktrees` returns nothing, goldens and e2e included
- [ ] **B:** an lvt e2e test shows a toast with `data-auto-dismiss` disappear after its delay. A toast removed early leaves no timer that clicks a missing button. A toast patched in place while its timer runs gets exactly one dismiss click, near its original deadline. A toast whose node is replaced gets exactly one dismiss click, on the new timer
- [ ] **B:** `grep -rE 'data-auto-dismiss' lvt/internal/kits --exclude-dir=.worktrees` finds the markup and the handler, and no `MutationObserver` remains beside them
- [ ] **C:** a prereview e2e test renders a diagram, edits its source, and sees the new diagram rendered with no console error. An invalid definition leaves its source visible and logs one warning
- [ ] **D:** tinkerdown's tutorial-mode e2e test finds the toolbar inside the sidebar, and `server.go` has no `MutationObserver`
- [ ] **E:** tracker row 6 in `issue-473.md` reads either "designing" with the case that needs hooks, or "closed: no demand in lvt, prereview or tinkerdown" with this plan linked. Only Phases A–C count toward the gate, and "no demand" covers only the apps this plan audited

## Constraints

- Each repo ships its own PR and release through its `release.sh`. lvt's goldens change in the same PR as its templates.
- The client fix in Phase A ships as a client patch, then a core patch adopts it, as in Phase 4 of `issue-473.md`.

## Risks you'd act on

- Phase A's client fix touches the same patch hook as the checkbox and radio fix. A form control the user is editing must keep the user's value. Reuse the "server rendered a different value than last time" rule rather than a new one.
