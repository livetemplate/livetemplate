# Lifecycle Hooks

**Status:** Proposed
**Extracted from:** bindings-proposal.md (Phase 4)

## TL;DR

**Problem:** Integrating third-party JavaScript libraries (charts, maps, rich text editors) requires manual DOM lifecycle management that's error-prone and repetitive.

**Solution:** `lvt-hook` attribute connecting DOM elements to JavaScript lifecycle callbacks.

**Pattern:**
```html
<canvas lvt-hook="chart" data-chart-type="line" data-chart-data="{{.ChartData}}"></canvas>
```
```typescript
const hooks = {
  chart: {
    mounted() { /* init chart with this.el, this.el.dataset */ },
    updated() { /* re-render on data change */ },
    destroyed() { /* cleanup */ },
    connected() { /* websocket connected */ },
    disconnected() { /* websocket lost */ },
  }
};
new LiveTemplate('/dashboard', { hooks });
```

## The Problem

Many web applications need to integrate JavaScript libraries that manage their own DOM state — chart libraries, map renderers, rich text editors, video players, code editors. These libraries typically require:

1. **Initialization** when the element appears in the DOM
2. **Updates** when data changes (re-render chart with new data)
3. **Cleanup** when the element is removed (destroy chart instance, release memory)
4. **Connection awareness** to handle WebSocket reconnection gracefully

Currently, developers must manually wire this up with `MutationObserver`, `addEventListener`, and careful lifecycle tracking. This is repetitive, error-prone, and hard to coordinate with LiveTemplate's DOM patching.

## Hook Interface

### Registration

Hooks are registered when creating a LiveTemplate client instance:

```typescript
const hooks = {
  chart: {
    mounted() { ... },
    updated() { ... },
    destroyed() { ... },
    connected() { ... },
    disconnected() { ... },
  },
  editor: {
    mounted() { ... },
    destroyed() { ... },
  }
};

new LiveTemplate('/path', { hooks });
```

### Hook Context (`this`)

Inside each callback, `this` provides:

| Property | Type | Description |
|----------|------|-------------|
| `this.el` | `HTMLElement` | The DOM element with `lvt-hook` |
| `this.data` | `Record<string, string>` | Parsed `data-*` attributes from the element |
| `this.pushEvent(event, payload)` | `Function` | Send a custom event to the server |

### Lifecycle Callbacks

| Callback | When Called | Use Case |
|----------|-----------|----------|
| `mounted()` | Element first added to DOM | Initialize library, bindings |
| `updated()` | Element's `data-*` attributes changed after DOM patch | Re-render with new data |
| `destroyed()` | Element removed from DOM | Cleanup, release resources |
| `connected()` | WebSocket connection established | Resume real-time features |
| `disconnected()` | WebSocket connection lost | Pause, show offline state |

### Callback Guarantees

- `destroyed()` is always called if `mounted()` was called, even on page navigation
- `updated()` only fires when `data-*` attributes actually change (shallow comparison)
- `connected()`/`disconnected()` fire on the same triggers as the existing `lvt:connected`/`lvt:disconnected` document-level events

## Template Usage

```html
<!-- Chart with server-driven data -->
<canvas lvt-hook="chart"
        data-chart-type="line"
        data-chart-data="{{.ChartJSON}}">
</canvas>

<!-- Rich text editor -->
<div lvt-hook="editor"
     data-content="{{.Content}}"
     data-readonly="{{.ReadOnly}}">
</div>

<!-- Map with markers -->
<div lvt-hook="map"
     data-lat="{{.Latitude}}"
     data-lng="{{.Longitude}}"
     data-zoom="{{.Zoom}}">
</div>
```

## Use Cases

### Chart Library (Chart.js)

```typescript
const hooks = {
  chart: {
    mounted() {
      const type = this.data.chartType;
      const data = JSON.parse(this.data.chartData);
      this.chart = new Chart(this.el, { type, data });
    },
    updated() {
      const data = JSON.parse(this.data.chartData);
      this.chart.data = data;
      this.chart.update();
    },
    destroyed() {
      this.chart.destroy();
    }
  }
};
```

### Map (Leaflet)

```typescript
const hooks = {
  map: {
    mounted() {
      this.map = L.map(this.el).setView(
        [parseFloat(this.data.lat), parseFloat(this.data.lng)],
        parseInt(this.data.zoom)
      );
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png').addTo(this.map);
    },
    updated() {
      this.map.setView(
        [parseFloat(this.data.lat), parseFloat(this.data.lng)],
        parseInt(this.data.zoom)
      );
    },
    destroyed() {
      this.map.remove();
    }
  }
};
```

### Rich Text Editor (Tiptap)

```typescript
const hooks = {
  editor: {
    mounted() {
      this.editor = new Editor({
        element: this.el,
        content: this.data.content,
        editable: this.data.readonly !== 'true',
        onUpdate: ({ editor }) => {
          this.pushEvent('content-changed', { html: editor.getHTML() });
        }
      });
    },
    updated() {
      if (this.data.readonly === 'true') {
        this.editor.setEditable(false);
      }
    },
    destroyed() {
      this.editor.destroy();
    }
  }
};
```

### Code Editor (CodeMirror)

```typescript
const hooks = {
  codeEditor: {
    mounted() {
      this.view = new EditorView({
        doc: this.data.code,
        extensions: [basicSetup, javascript()],
        parent: this.el,
        dispatch: (tr) => {
          this.view.update([tr]);
          if (tr.docChanged) {
            this.pushEvent('code-changed', { code: this.view.state.doc.toString() });
          }
        }
      });
    },
    destroyed() {
      this.view.destroy();
    }
  }
};
```

## Server-Side Communication

Hooks can communicate with the server via `pushEvent`:

```typescript
// Client: push event from hook
this.pushEvent('chart-clicked', { pointIndex: 3, value: 42 });
```

```go
// Server: handle hook event in controller action
func (c *DashboardController) ChartClicked(state DashboardState, ctx *livetemplate.Context) (DashboardState, error) {
    pointIndex := ctx.GetInt("pointIndex")
    state.SelectedPoint = pointIndex
    return state, nil
}
```

## Implementation Notes

### Client-Side Only

The hook system is entirely client-side. No server-side changes are needed. The implementation involves:

1. **Hook Registry** — Store hook definitions keyed by name
2. **DOM Patch Integration** — After each DOM patch, detect:
   - New elements with `lvt-hook` → call `mounted()`
   - Changed `data-*` on existing hooked elements → call `updated()`
   - Removed hooked elements → call `destroyed()`
3. **Connection Events** — Wire `lvt:connected`/`lvt:disconnected` events to hook callbacks

### Change Detection for `updated()`

After each DOM patch:
1. For each element with `lvt-hook` that existed before the patch
2. Compare `data-*` attributes before and after
3. If any changed, call `updated()` with refreshed `this.data`

### Cleanup Guarantees

- Track all mounted hooks in a `WeakMap<HTMLElement, HookInstance>`
- On `beforeunload`, call `destroyed()` for all active hooks
- On element removal during DOM patch, call `destroyed()` before removal

### TypeScript Types

```typescript
interface HookCallbacks {
  mounted?(): void;
  updated?(): void;
  destroyed?(): void;
  connected?(): void;
  disconnected?(): void;
}

interface HookContext {
  el: HTMLElement;
  data: Record<string, string>;
  pushEvent(event: string, payload?: Record<string, any>): void;
}

type HookDefinition = HookCallbacks & ThisType<HookContext & Record<string, any>>;

interface LiveTemplateOptions {
  hooks?: Record<string, HookDefinition>;
}
```

## Integration with Existing System

- The client already emits `lvt:connected` and `lvt:disconnected` document-level events. The hook `connected()`/`disconnected()` callbacks fire on the same triggers.
- Hooks work alongside all existing `lvt-*` attributes — they don't replace event handlers, they complement them for cases where JavaScript library integration is needed.
- `pushEvent()` sends messages through the existing WebSocket connection using the standard action message format.

## Open Questions

1. **Hook naming collisions** — Should hook names be globally unique, or scoped per-page?
2. **Multiple hooks per element** — Should `lvt-hook="chart editor"` be supported?
3. **Hook inheritance** — Should child elements inherit parent hooks?
4. **SSR hydration** — How should hooks behave when hydrating server-rendered HTML?
