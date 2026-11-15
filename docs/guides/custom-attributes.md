# Custom attributes

The built-in `lvt-*` attributes cover debounce, reactive classes, lifecycle hooks and a few effects. Sooner or later a page wants one more: copy a link, drive a star rating, bind a keyboard shortcut. Write it yourself. A handler is a plain object in a `<script>` tag, written against the same interface the built-ins use.

The server never learns the attribute exists. It renders `lvt-x:copy="{{.ShareURL}}"` as ordinary HTML, and the client calls your handler for every element that carries it.

A runnable version of everything on this page is the [custom attributes recipe](https://livetemplate.fly.dev/recipes/custom-attributes): a star rating, a copy button and keyboard shortcuts, with a browser test behind each.

## A first handler

A copy-to-clipboard button:

```html
<button lvt-x:copy="{{.ShareURL}}">Copy link</button>
```

```html
<script>
  (window.lvtAttributes ||= []).push({
    attribute: "lvt-x:copy",
    onElementAdded(el, ctx) {
      el.addEventListener("click", () => navigator.clipboard.writeText(ctx.value), {
        signal: ctx.signal,
      });
    },
  });
</script>
<script defer src="{{lvtClientScriptURL}}"></script>
```

That is the whole handler. You name the attribute, and the client finds the elements, escapes the selector and calls `onElementAdded` once per element, however many renders follow.

Two members of `ctx` carry most of the weight.

**`ctx.value` is read when you touch it.** The listener is attached once and fires later, possibly many renders later, and `{{.ShareURL}}` can change in between. `ctx.value` re-reads the attribute, so the click copies the current link. A destructured `const { value } = ctx` would freeze the link as it was when the button was wired.

**`ctx.signal` undoes the wiring.** It aborts when the element stops being armed: the server removes the attribute or renders it empty, the element leaves the page, or the client shuts down. Pass it to every listener `onElementAdded` adds. Leave it out and two things go wrong. The button keeps copying after the server took the attribute away. And if the attribute comes back, `onElementAdded` runs again and the button gets a second listener.

An empty value (`lvt-x:copy=""`) counts as no attribute. The client logs a warning and skips the element. The value is the handler's configuration, so an empty one is treated as a template bug.

## Load order: always use the queue

The script pushes onto `window.lvtAttributes` instead of calling the client directly. That is on purpose. An inline `<script>` ignores `defer`, so it runs before the deferred client has loaded, and there is no `LiveTemplateClient` global to call yet. The queue works in any order. The script can sit in `<head>` or anywhere in `<body>`; inside `<body>` it also survives a host page that inlines only the app's body, as the docs site's live embeds do. When the client loads it registers whatever was queued, then replaces `push` so later pushes register immediately.

Code that runs after the client, such as a deferred bundle or an ES module import, can call `LiveTemplateClient.registerAttribute(handler)` instead. The queue still works there too, so there is little reason to switch.

A handler registered before the client connects runs over the server-rendered page at connect, without waiting for a render. One registered later runs against the live page straight away.

## Sending an action

A star rating needs the server. Ask for the transport with `needsServerChannel: true`, and `ctx.send` appears:

```html
<div class="stars" lvt-x:rating="SetRating">
  {{range .Stars}}<button type="button" data-star="{{.}}">★</button>{{end}}
</div>
```

```js
(window.lvtAttributes ||= []).push({
  attribute: "lvt-x:rating",
  needsServerChannel: true,
  onElementAdded(el, ctx) {
    el.addEventListener("click", (e) => {
      const star = e.target.closest("[data-star]");
      if (!star) return;
      ctx.send?.({ action: ctx.value, data: { stars: Number(star.dataset.star) } });
    }, { signal: ctx.signal });
  },
});
```

Here the attribute's value is the action name. `ctx.send` resolves the live connection each time it is read, so the listener keeps working after a WebSocket reconnect. It is typed as optional, because a handler without `needsServerChannel` gets none; hence `ctx.send?.`.

The server side is an ordinary action:

```go
func (c *Controller) SetRating(s State, ctx *livetemplate.Context) (State, error) {
    s.Rating = ctx.GetInt("stars")
    return s, nil
}
```

Nothing on the server can tell this action came from a custom attribute rather than a `<button name="setRating">`. It is the same message on the same connection. So the server authorizes it the way it authorizes any action. A registered handler proves nothing about who is calling.

## The low-level form

Everything above uses the declarative form: name an attribute, get called per element. The built-ins mostly use a lower one, `selectors` plus `setup()`, and it is public too. It is rarely the right choice. Use it when one of these is true:

- the handler keeps state shared across elements, such as one document-wide listener or a drag in progress;
- it matches more than one attribute, or something that is not a plain attribute;
- it needs to batch work across a whole render.

Keyboard shortcuts are the first case. One `keydown` listener serves every element, and it needs a map from key to element:

```js
(() => {
  const keysByRoot = new Map();
  const onKey = (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.target.closest?.("input, textarea, select, [contenteditable]")) return;
    for (const keys of keysByRoot.values()) {
      const el = keys.get(e.key);
      if (el) return el.click();
    }
  };
  (window.lvtAttributes ||= []).push({
    name: "lvt-x:hotkey",
    selectors: ["[lvt-x\\:hotkey]"],
    claims: ["lvt-x:hotkey"],
    runOnConnect: true,
    setup({ wrapperRoot }) {
      const keys = new Map();
      for (const el of wrapperRoot.querySelectorAll("[lvt-x\\:hotkey]")) {
        keys.set(el.getAttribute("lvt-x:hotkey"), el);
      }
      keysByRoot.set(wrapperRoot, keys);
      document.addEventListener("keydown", onKey);
    },
    teardown(root) {
      keysByRoot.delete(root);
    },
    dispose() {
      document.removeEventListener("keydown", onKey);
    },
  });
})();
```

`setup()` runs after every render, and `runOnConnect: true` adds one run when the page connects. Handlers are shared by every client on the page, and `setup()` runs for each one, so state that belongs to a client is keyed by its `wrapperRoot`. A single map would let one client's render wipe another's keys. `teardown(root)` drops a client's entry when it goes, and `dispose()` removes the document listener once the last client has. Everything the declarative form did for you is now yours: the escaped selector, the scan, and the guarantees below. `name` is required here, since a selector list cannot identify the handler in a warning. `claims` tells the census which attribute names the handler owns.

### Three mistakes, and why the declarative form cannot make them

| Mistake | What happens | Declarative form |
|---------|--------------|------------------|
| No sweep | the server removes the attribute and the element keeps working | `ctx.signal` aborts and `onElementRemoved` fires |
| Setup that is not idempotent | `setup()` runs after every render, so a plain `addEventListener` adds one more listener each time | `onElementAdded` fires once per arming |
| A `send` captured once | after a reconnect, actions go to a dead connection | `ctx.send` resolves the live connection when read |

The hotkey handler avoids all three. It rebuilds its client's map on every run, so a key whose attribute the server removed simply drops out. That rebuild is the sweep. It adds the same `onKey` function each time, and the DOM ignores an identical listener added twice, so nothing stacks. And it calls `el.click()` instead of sending an action itself, so it never holds a connection at all. A handler that does send should read `send` from each `setup()` call, never from the first.

## Choosing a category

A handler's category decides when it runs.

| Category | Runs | Use when |
|----------|------|----------|
| `fire-on-change` | every render | the handler reacts to a value changing on an element that already exists |
| `wire-idempotent` | only when a render added nodes or gave an element an `lvt-*` attribute it lacked | the handler attaches listeners, and an unchanged page has nothing new to attach |

A declarative handler rarely sets this. It is derived from the callbacks you supply. Supply `onElement`, which runs on every render for every matching element, and you get `fire-on-change`. Supply only `onElementAdded` and `onElementRemoved`, and you get `wire-idempotent`. An explicit `category` always wins. Removals ignore the category: the client checks for disarmed elements on every render regardless.

`onElement` sees the same `ctx` and the same `signal` on every render, so read in it and wire nothing. A listener added in `onElement` stacks.

A low-level handler has nothing to derive from, so an omitted category means `fire-on-change`. The difference matters on big pages. The client's own comment puts the saving from `wire-idempotent` at roughly 150–200 ms per render at 80,000 nodes, for a handler that walks many descendants.

## Pick a namespace the framework won't use

Use `lvt-x:` for your own attributes, or a prefix of your own such as `lvt-acme:`. Stay out of `lvt-fx:`, `lvt-el:` and `lvt-form:`. Those belong to the framework and will grow, so a community `lvt-fx:tooltip` collides on the release that adds one. Nothing enforces this. When a second handler registers a name that is already taken, the client warns and runs both, which is rarely what either author wanted.

## The attribute census

The server sends the client every `lvt-*` attribute name the template uses. Once the page's `load` event has fired, the client warns once for each name no handler claims:

```
lvt-x:hotky is used by this page's template, but no attribute handler claims it, …
```

That warning usually means a typo, or a script that failed to load. Declarative handlers claim their `attribute` automatically. Low-level handlers claim what they list in `claims`; a name ending in `:` claims a whole namespace. Waiting for `load` gives an `async` or external bundle time to arrive before anything is reported.

## Sharing a handler with other apps

Everything so far is a handler that lives in one app's template. To publish one, as an npm package or a file on a CDN, two things change.

**Push onto the queue, and never bundle the client.** The registry belongs to the client that the page loaded. A package that bundles its own copy of `@livetemplate/client` registers into that copy, which renders nothing. Push onto `window.lvtAttributes` and the package works in any page and in any load order. For types, import them type-only (`import type { AttributeHandler } from "@livetemplate/client"`); they are erased from the output.

**Load it with a pinned version and an integrity hash.**

```html
<script defer
  src="https://cdn.jsdelivr.net/npm/@acme/lvt-tooltip@1.2.3/dist/tooltip.js"
  integrity="sha384-…"
  crossorigin="anonymous"></script>
```

A handler runs with the page's full privileges. It reads the DOM, it can send any action, and it sees whatever the user types. So a third-party handler is trusted exactly as much as any other script on the page. Pin the version, check the hash, and keep authorizing every action on the server.

## Testing a handler

A declarative handler is a plain object, so unit tests need no framework at all. Call `onElementAdded(el, ctx)` with a stub and assert on what happens:

```ts
const sent: unknown[] = [];
const controller = new AbortController();
const ctx = {
  get value() { return el.getAttribute("lvt-x:rating") ?? ""; },
  send: (m: unknown) => sent.push(m),
  signal: controller.signal,
} as Partial<ElementContext> as ElementContext;

handler.onElementAdded!(el, ctx);
el.querySelector<HTMLElement>('[data-star="3"]')!.click();
expect(sent).toEqual([{ action: "SetRating", data: { stars: 3 } }]);

controller.abort();
el.querySelector<HTMLElement>('[data-star="3"]')!.click();
expect(sent).toHaveLength(1);
```

Type the stub as `Partial<ElementContext>` and cast it. The client may add members to its context types in any release, and a full stub would stop compiling.

A low-level handler is called the same way the client calls it: `setup({ scanRoot, wrapperRoot, send })`. Anything that depends on the real browser, such as the clipboard, pointer capture or key events, belongs in a browser test. The recipe's test drives all three handlers through a real Chrome.

## Stability

The handler interface is public and versioned under semver. The rules for what may change in which release are in the [client README](https://github.com/livetemplate/client#attribute-handlers).
