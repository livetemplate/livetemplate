# `livetemplate.css` semantic-tag coverage scope

**Status:** Decision Needed
**Tracking issue:** [livetemplate/livetemplate#317](https://github.com/livetemplate/livetemplate/issues/317)
**Related:** Client [`livetemplate.css`](https://github.com/livetemplate/client/blob/main/livetemplate.css)

## TL;DR

Issue #317 asks to "extend picocss semantic tags in `livetemplate.css`" without naming a target set. The current file is small and tightly scoped: directive custom properties + one semantic-tag rule (`output[data-flash]`) + layout + utility classes + a chat pattern. **Before adding more, we need a written policy on what coverage is in scope.** This doc frames the choice; it does not decide it.

## Current `livetemplate.css` (at time of writing; see the **Related** link above for the live file)

| Category | Rules |
|----------|-------|
| Directive defaults | `:root` custom properties: `--lvt-scroll-behavior`, `--lvt-scroll-threshold`, `--lvt-highlight-color`, `--lvt-highlight-duration`, `--lvt-animate-duration` |
| Semantic-tag styling | `output[data-flash]` only |
| Layout | `.container` (narrow 640px max-width override of Pico's breakpoint defaults) |
| Utility classes | `.compact`, `.visually-hidden`, `.inline` |
| Reusable patterns | `.messages`, `.message`, `.message.mine` (chat layout) |

Total: ~95 lines. The file ships via npm and apps load it with a single `<link>` tag.

## What "extending picocss semantic tags" could mean

Picocss already styles most HTML5 semantic tags out of the box: `header`, `nav`, `main`, `footer`, `aside`, `article`, `section`, `figure`, `details`/`summary`, `dialog`, etc. Adding "coverage" in `livetemplate.css` is therefore additive on top of picocss — either filling gaps where picocss is silent, or overriding picocss for LVT-specific patterns.

Three coherent scope options:

### Option A — Minimal (current posture)

`livetemplate.css` only styles tags/patterns where LVT-specific behavior interacts with the DOM (currently: `output[data-flash]`). Everything else defers to picocss. Issue #317 is closed as "not in scope" with a pointer to apps' own stylesheets.

- **Pro:** smallest possible CSS surface; no new design decisions to maintain.
- **Pro:** apps that don't use LVT-specific patterns see no overhead.
- **Pro:** zero risk of fighting picocss defaults.
- **Con:** apps using LVT's reactive patterns (flash, dialog, navigate spinner, range diff visual cues) have to write CSS themselves.
- **Con:** the chat pattern that already ships in `.messages`/`.message` is arguably already past this option's bar — Option A would arguably remove it.

### Option B — Curated LVT-specific patterns

Add styling only for tags/patterns that LVT-specific behaviors produce, not for general HTML5 coverage. Concretely, this means hooks like:

- `output[data-flash]` (already shipping) — flash messages.
- `dialog[open]` — the standard Tier-1 modal pattern uses `<dialog>` with native `command`/`commandfor` (no `lvt-*` attributes; the client polyfills the Invoker Commands API for older browsers). `livetemplate.css` could ship transition/backdrop tweaks picocss doesn't cover, including the auto-close-on-success behavior the client implements.
- `[data-lvt-state="pending"]`, `[data-lvt-state="error"]` — connection-state visual cues for the loading-indicator and disconnected states (selector names illustrative; actual attribute names belong to the client and should be confirmed at implementation time).
- `[lvt-fx\:highlight]` (escape needed because `:` is reserved in CSS selectors) — the highlight directive flashes elements via custom-property-driven animations; picocss styles `<mark>` for static highlights but doesn't cover the animation hook that `lvt-fx:highlight` needs.
- Range diff visual cues (insert/remove transitions) — currently apps wire these themselves.

The chat pattern (`.messages`, `.message`, `.message.mine`) stays under this option as a "reusable pattern" alongside layout and utilities.

- **Pro:** users opting into LVT directives get a working visual language out of the box.
- **Pro:** keeps the file scope-bounded — only styles things LVT itself emits or expects.
- **Con:** boundary is fuzzy. "LVT-specific" can be argued for many things (dialog, connection state, flash) that are also general HTML5 patterns.
- **Con:** requires per-pattern decisions about what the default style should look like.

### Option C — LVT-flavored picocss extension

Treat `livetemplate.css` as a curated picocss companion: opinionated styles for all HTML5 semantic tags that picocss leaves bare, plus the LVT-specific patterns above. This is the "extend picocss semantic tags" reading taken literally.

Candidate tags picocss is currently silent or thin on:
- `<aside>`, `<figure>`/`<figcaption>` (picocss styles these but minimally)
- `<details>`/`<summary>` (picocss has light coverage)
- `<dialog>` (picocss has some, LVT-specific pattern adds more)
- `<address>`, `<cite>`, `<q>`, `<blockquote>` (typography-heavy, picocss minimal)
- `<kbd>`, `<samp>`, `<var>` (technical inline, picocss styles `<code>` only)
- `<time>`, `<abbr>`, `<dfn>` (semantic inline)
- `<progress>`, `<meter>` (form-like; picocss styles `<progress>` minimally)

- **Pro:** apps get a substantially more complete out-of-the-box visual language.
- **Pro:** LVT becomes a "batteries included" choice for prototyping.
- **Con:** ~5x the CSS surface and ~10x the design decisions. Every new tag means picking a default style, documenting it, and maintaining it.
- **Con:** style opinions get baked in. Users who disagree with the defaults have to override — and the more LVT styles, the more overriding apps need.
- **Con:** drifts toward becoming a full design system, which isn't LVT's stated mission.

## Scope-deciding questions

Before #317 is implemented, these need answers:

1. **What problem is the issue solving?** "Extend picocss semantic tags" is a *what*, not a *why*. Is it driven by:
    - Existing apps repeating the same custom-style boilerplate? (If yes → which patterns?)
    - User complaints that LVT examples look unstyled?
    - A perceived mismatch between LVT's "Tier 1 standard HTML" philosophy and picocss's coverage?
    - Something else?
    Without a stated motivation, the implementer can't tell A/B/C apart.

2. **Where is the scope boundary?** Picocss already covers most semantic tags. Where does "LVT extends picocss" end and "LVT becomes a design system" begin? Without a written line, future PRs will keep nibbling at C.

3. **How does this interact with apps that bring their own picocss alternative?** Some apps don't use picocss at all (Tailwind, custom CSS). Today `livetemplate.css` references `var(--pico-...)` custom properties — apps without picocss break or get bare styles. Option B/C amplify this coupling. Worth deciding whether `livetemplate.css` is picocss-coupled or framework-agnostic.

4. **Is dark mode in scope?** Picocss handles dark mode via `[data-theme]` and `prefers-color-scheme`. If LVT extends picocss, every new rule needs a dark-mode counterpart. If LVT-specific patterns ship colors (the chat `.message.mine` does), they should respect picocss's theme variables — currently it does (via `var(--pico-primary)` etc.). Option C makes this commitment more weighty.

5. **What's the maintenance contract?** Picocss releases new versions that adjust defaults. Every Option-C rule that overrides a picocss default needs review on each picocss bump to make sure we're not undoing a good upstream change. This is real ongoing work.

## Recommendation

**Default to Option B for any concrete request, and write a one-paragraph "scope policy" at the top of `livetemplate.css` so future contributors know where the line is.**

Concretely:

- Add a header comment to `livetemplate.css` stating: "This file ships styling only for (a) `lvt-fx:*` directive defaults, (b) tags/attributes LVT emits or processes (e.g. `output[data-flash]`, `[data-lvt-state]`), and (c) opinionated reusable patterns (`.container`, `.compact`, chat layout). General-purpose HTML5 tag styling is delegated to picocss — apps wanting broader coverage should pick a CSS framework alongside LVT."
- File a follow-up issue per concrete pattern someone wants added (e.g. "Style `dialog[open]` for the lvt-form:no-intercept Tier 1 pattern") so each addition is justified by a concrete need.
- Close #317 as "needs concrete scope" or convert it into a tracking issue for the policy commit + first follow-up.

Option A is too minimal (the chat pattern already shipped under it would have to come out). Option C is too maintenance-heavy and pushes LVT toward a domain (design system) outside its stated mission. Option B with a written policy threads the needle.

## Implementation checklist (for the follow-up PR)

This proposal is pre-decision (see the **Status** field at the top). The items below describe what the *implementing* PR should land once a scope option is chosen — they are not gates for merging this proposal.

1. The chosen policy is written into `livetemplate.css`'s header comment.
2. Issue #317 is either closed or converted to a tracking issue with a concrete next step.
3. Future style additions to `livetemplate.css` cite the policy in their PR description.
4. CHANGELOG entry on the next client release notes the scope policy.

## Out of Scope

- **Replacing picocss with a different framework.** This proposal assumes picocss stays as the recommended default. Switching is a much larger discussion.
- **Tailwind / utility-first compatibility.** Apps choosing Tailwind already opt out of `livetemplate.css`; making it Tailwind-compatible is a separate project.
- **Dark-mode design decisions.** Picocss handles dark mode; LVT-specific rules should use picocss theme variables (current practice). If/when LVT goes framework-agnostic, dark mode becomes a separate ADR.

## Appendix: References

- Current `livetemplate.css`: client repo, root path.
- Picocss default semantic-tag coverage: [picocss/pico/css](https://github.com/picocss/pico/tree/main/css).
- [Issue #41 (client)](https://github.com/livetemplate/client/issues/41) — closed as done; the CSS extension API (custom properties + npm distribution) is in place. This proposal is about *what styles ship*, not *how users override*.
