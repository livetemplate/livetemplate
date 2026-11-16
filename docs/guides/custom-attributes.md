# Custom attributes

When the built-in `lvt-*` attributes don't cover what a page needs, write your own: a plain object in a `<script>` tag, using the interface the built-ins use. The server renders the attribute as ordinary HTML, and the client calls your handler for each element that carries it. The [custom attributes recipe](https://livetemplate.fly.dev/recipes/custom-attributes) runs every example on this page, with a browser test behind each. It needs livetemplate v0.25.5 or later: v0.25.4 brings client 0.25.6 and `ctx.signal`, and v0.25.5 keeps a handler script in `<body>` inside the app.

## A first handler

```html
<button lvt-x:copy="{{.ShareURL}}">Copy link</button>

<script>
  (window.lvtAttributes ||= []).push({
    attribute: "lvt-x:copy",
    // Called once per element, however many renders follow.
    onElementAdded(el, ctx) {
      el.addEventListener(
        "click",
        // ctx.value re-reads the attribute, so the click copies the current
        // link. A destructured `const { value } = ctx` would freeze it.
        () => navigator.clipboard.writeText(ctx.value),
        // Aborts when the server removes the attribute, the element leaves
        // the page or the client shuts down. Without it the button keeps
        // copying, and gets a second listener if the attribute comes back.
        { signal: ctx.signal },
      );
    },
  });
</script>
<script defer src="{{lvtClientScriptURL}}"></script>
```

An empty value (`lvt-x:copy=""`) counts as no attribute: the client logs a warning and skips the element.

Push onto `window.lvtAttributes` instead of calling the client, because an inline `<script>` ignores `defer` and runs before the client exists. The queue works in `<head>` or `<body>` and in any load order. Code that runs after the client can call `LiveTemplateClient.registerAttribute(handler)` instead.

## Sending an action

```html
<div class="stars" lvt-x:rating="SetRating">
  {{range .Stars}}<button type="button" data-star="{{.}}">★</button>{{end}}
</div>
```

```js
(window.lvtAttributes ||= []).push({
  attribute: "lvt-x:rating",
  needsServerChannel: true, // without it, ctx.send is undefined
  onElementAdded(el, ctx) {
    el.addEventListener("click", (e) => {
      const star = e.target.closest("[data-star]");
      if (!star) return;
      // ctx.send resolves the live connection on each read, so it survives a reconnect.
      ctx.send?.({ action: ctx.value, data: { stars: Number(star.dataset.star) } });
    }, { signal: ctx.signal });
  },
});
```

```go
func (c *Controller) SetRating(s State, ctx *livetemplate.Context) (State, error) {
    s.Rating = ctx.GetInt("stars")
    return s, nil
}
```

The server receives the same message a `<button name="setRating">` sends, so authorize it the way you authorize any action.

## The low-level form

Use the declarative form unless the handler:

- keeps state shared across elements, such as one document-wide listener;
- matches more than one attribute, or something that is not a plain attribute;
- batches work across a whole render.

Keyboard shortcuts use one `keydown` listener for every element:

```js
(() => {
  // Every client on the page shares the handler, so key state by wrapperRoot.
  const keysByRoot = new Map();
  const onKey = (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.target.closest?.("input, textarea, select, [contenteditable]")) return;
    for (const keys of keysByRoot.values()) {
      const el = keys.get(e.key);
      if (el) return el.click(); // first client with the key wins; click instead of send, so no connection to hold
    }
  };
  (window.lvtAttributes ||= []).push({
    name: "lvt-x:hotkey",           // required: names the handler in warnings
    selectors: ["[lvt-x\\:hotkey]"], // you escape the selector yourself
    claims: ["lvt-x:hotkey"],       // names this handler owns; "lvt-x:" would claim the namespace
    runOnConnect: true,             // also run once at connect
    // Runs after every render, so rebuild the map: a removed key drops out.
    setup({ wrapperRoot }) {
      const keys = new Map();
      for (const el of wrapperRoot.querySelectorAll("[lvt-x\\:hotkey]")) {
        keys.set(el.getAttribute("lvt-x:hotkey"), el);
      }
      keysByRoot.set(wrapperRoot, keys);
      document.addEventListener("keydown", onKey); // same function each time, so it never stacks
    },
    teardown(root) { keysByRoot.delete(root); },                  // a client went away
    dispose() { document.removeEventListener("keydown", onKey); }, // the last client went away
  });
})();
```

A low-level handler that sends reads `send` from each `setup()` call; the one from the first call dies with the first connection.

## Namespaces and the census

Use `lvt-x:` or a prefix of your own, such as `lvt-acme:`. `lvt-fx:`, `lvt-el:` and `lvt-form:` belong to the framework and will grow.

After the page's `load` event, the client warns once for each `lvt-*` name in the template that no handler claims. That usually means a typo or a script that failed to load:

```
lvt-x:hotky is used by this page's template, but no attribute handler claims it, …
```

## Sharing a handler

Push onto the queue and never bundle `@livetemplate/client`: a bundled copy has its own registry, which renders nothing. Import types with `import type`. A handler runs with the page's full privileges, so load it pinned and with an integrity hash:

```html
<script defer
  src="https://cdn.jsdelivr.net/npm/@acme/lvt-tooltip@1.2.3/dist/tooltip.js"
  integrity="sha384-…"
  crossorigin="anonymous"></script>
```

## Testing a handler

```ts
import type { ElementContext } from "@livetemplate/client";
// el: the rating <div> from your fixture; handler: the object you pushed.
const sent: unknown[] = [];
const controller = new AbortController();
// Partial + cast: the client may add context members in any release.
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

Clipboard, pointer and key behaviour belong in a browser test, like the recipe's.

The handler interface is versioned under semver. The [client README](https://github.com/livetemplate/client#attribute-handlers) lists what may change in which release.
