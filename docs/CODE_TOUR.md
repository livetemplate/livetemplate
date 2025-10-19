# Code Tour: A Guided Walkthrough

This document provides a guided tour through the LiveTemplate codebase, helping you understand how the pieces fit together.

## Table of Contents

- [Starting Point](#starting-point)
- [Tour 1: Simple Template Rendering](#tour-1-simple-template-rendering)
- [Tour 2: Interactive Updates](#tour-2-interactive-updates)
- [Tour 3: Template Parsing](#tour-3-template-parsing)
- [Tour 4: Tree Generation](#tour-4-tree-generation)
- [Tour 5: Client-Side Updates](#tour-5-client-side-updates)
- [Advanced Topics](#advanced-topics)

## Starting Point

Before diving in, familiarize yourself with:

1. **Run the counter example** to see LiveTemplate in action
   ```bash
   cd examples/counter
   go run main.go
   # Open http://localhost:8080 and click the buttons
   ```

2. **Read the README** for basic concepts

3. **Check examples/** for real applications:
   - `counter/` - Simple state management
   - `todos/` - Full CRUD application

## Tour 1: Simple Template Rendering

**Goal:** Understand how a template is loaded and rendered to HTML

### Step 1: Create a Template (`template.go:71-98`)

```go
// examples/counter/main.go
tmpl := livetemplate.New("counter")
```

**What happens:**
1. `New()` looks for `counter.tmpl` in current directory
2. Parses template using `html/template`
3. Generates unique wrapper ID (`lvt-[random]`)
4. Creates key generator for this template instance

**File:** `template.go`
```go
func New(name string) *Template {
    // Find template file (counter.tmpl, templates/counter.tmpl, etc.)
    templatePath := findTemplateFile(name)

    // Read template content
    content, _ := os.ReadFile(templatePath)

    // Create Template instance
    return &Template{
        name:        name,
        templateStr: string(content),
        wrapperID:   generateRandomID(), // e.g., "lvt-abc123"
        keyGen:      newKeyGenerator(),
    }
}
```

### Step 2: First Render - HTML (`template.go:173-224`)

```go
// First request: GET /
html, _ := tmpl.ExecuteToHTML(state)
```

**What happens:**
1. Parse template to tree structure
2. Inject wrapper div around content
3. Embed tree data in HTML (for client-side hydration)
4. Return complete HTML document

**File:** `template.go`
```go
func (t *Template) ExecuteToHTML(data interface{}) (string, error) {
    // Parse template to tree structure
    tree, _ := parseTemplateToTree(t.templateStr, data, t.keyGen)

    // Store for future diffs
    t.lastTree = tree
    t.lastData = data

    // Execute template normally to get HTML
    var buf bytes.Buffer
    t.tmpl.Execute(&buf, data)
    html := buf.String()

    // Inject wrapper div with unique ID
    html = injectWrapperDiv(html, t.wrapperID, false)

    return html, nil
}
```

**Key insight:** Even the first render generates a tree structure. This tree will be used for subsequent updates.

### Step 3: Tree Generation (`tree.go:266-275`)

```go
// Parse template to tree
tree, _ := parseTemplateToTree(templateStr, data, keyGen)
```

**What happens:**
1. Call AST parser (tree_ast.go)
2. Return tree with statics and dynamics separated

**File:** `tree.go`
```go
func parseTemplateToTree(templateStr string, data interface{}, keyGen *keyGenerator) (tree treeNode, err error) {
    // Recover from panics (fuzz testing safety)
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("template execution panic: %v", r)
        }
    }()

    return parseTemplateToTreeAST(templateStr, data, keyGen)
}
```

**Next:** See Tour 3 for AST parsing details.

## Tour 2: Interactive Updates

**Goal:** Understand how user actions trigger updates

### Step 1: User Clicks Button (`client/livetemplate-client.ts:500-550`)

**Template:**
```html
<button lvt-click="increment">+</button>
```

**Client:**
```typescript
// Event listener attached on page load
element.addEventListener('click', () => {
    const action = element.getAttribute('lvt-click'); // "increment"
    const data = collectFormData(element); // {...}

    // Send to server
    sendAction(action, data);
});
```

### Step 2: Server Receives Action (`mount.go:272-320`)

**WebSocket handler:**
```go
func handleWebSocket(conn *websocket.Conn, tmpl *Template, store Store) {
    for {
        // Read message from client
        messageType, message, _ := conn.ReadMessage()

        // Parse action message
        msg, _ := parseActionFromWebSocket(message)
        // msg.Action = "increment"
        // msg.Data = {...}

        // Call user's Change() method
        ctx := &ActionContext{
            Action: msg.Action,
            Data:   newActionData(msg.Data),
        }
        err := store.Change(ctx)

        // Re-render template
        update, _ := tmpl.ExecuteToUpdate(store)

        // Send update to client
        updateJSON, _ := json.Marshal(update)
        conn.WriteMessage(websocket.TextMessage, updateJSON)
    }
}
```

### Step 3: User's Change Method (`examples/counter/main.go`)

```go
type CounterState struct {
    Counter int `json:"counter"`
}

func (s *CounterState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "increment":
        s.Counter++  // Modify state
    case "decrement":
        s.Counter--
    }
    return nil
}
```

**Key insight:** User only writes business logic. Framework handles rendering and updates automatically.

### Step 4: Generate Update (`template.go:226-286`)

```go
update, _ := tmpl.ExecuteToUpdate(data)
```

**What happens:**
1. Parse template with new data
2. Diff new tree vs last tree
3. Generate minimal update (only changes)
4. Return UpdateResponse

**File:** `template.go`
```go
func (t *Template) ExecuteToUpdate(data interface{}) (*UpdateResponse, error) {
    // Parse template to tree with new data
    newTree, _ := parseTemplateToTree(t.templateStr, data, t.keyGen)

    // Calculate fingerprint for change detection
    newFingerprint := calculateFingerprint(newTree)

    // Early exit if nothing changed
    if newFingerprint == t.lastFingerprint {
        return &UpdateResponse{Tree: map[string]interface{}{}}, nil
    }

    // Store for next diff
    t.lastTree = newTree
    t.lastFingerprint = newFingerprint

    return &UpdateResponse{
        Tree: newTree, // Minimal update (only changed values)
        Meta: &ResponseMetadata{
            WrapperID: t.wrapperID,
        },
    }, nil
}
```

### Step 5: Client Applies Update (`client/livetemplate-client.ts:800-900`)

**Client receives:**
```json
{
  "tree": {
    "0": "42"  // Updated counter value
  },
  "meta": {
    "wrapper_id": "lvt-abc123"
  }
}
```

**Client applies:**
```typescript
function applyUpdate(update: UpdateResponse) {
    const wrapper = document.querySelector(`[data-lvt-id="${update.meta.wrapper_id}"]`);

    // Resolve statics from cache + new dynamics
    const tree = resolveTree(update.tree);

    // Build DOM from tree
    const newContent = buildDOM(tree);

    // Replace wrapper content
    wrapper.innerHTML = newContent;
}
```

**Key insight:** Client only receives changed data, not full HTML. Statics are cached from first render.

## Tour 3: Template Parsing

**Goal:** Understand how templates become tree structures

### Step 1: AST Parsing (`tree_ast.go:50-150`)

```go
tree, _ := parseTemplateToTreeAST(templateStr, data, keyGen)
```

**What happens:**
1. Parse template using stdlib `html/template`
2. Walk AST to find template constructs
3. Compile constructs (define structure)
4. Hydrate constructs (fill with data)
5. Build tree

**File:** `tree_ast.go`
```go
func parseTemplateToTreeAST(templateStr string, data interface{}, keyGen *keyGenerator) (treeNode, error) {
    // Parse template
    tmpl := template.New("").Funcs(customFuncs)
    tmpl, _ = tmpl.Parse(templateStr)

    // Compile: Find all constructs
    constructs := compileConstruct(tmpl.Tree.Root, ...)

    // Hydrate: Fill with actual data
    tree := hydrateConstruct(constructs, data, keyGen, ...)

    return tree, nil
}
```

### Step 2: Compile Constructs (`tree_ast.go:200-500`)

**Goal:** Identify template structure (fields, conditionals, ranges)

**Example template:**
```html
<div>
    Hello {{.Name}}
    {{if .ShowMessage}}
        <p>{{.Message}}</p>
    {{end}}
</div>
```

**Compiled constructs:**
```go
[]Construct{
    TextConstruct{Value: "<div>\n    Hello "},
    FieldConstruct{FieldName: "Name"},
    TextConstruct{Value: "\n    "},
    ConditionalConstruct{
        Condition:  ".ShowMessage",
        TrueBranch: []Construct{
            TextConstruct{Value: "\n        <p>"},
            FieldConstruct{FieldName: "Message"},
            TextConstruct{Value: "</p>\n    "},
        },
        FalseBranch: nil,
    },
    TextConstruct{Value: "\n</div>"},
}
```

**File:** `tree_ast.go`
```go
func compileConstruct(node parse.Node, ...) []Construct {
    switch n := node.(type) {
    case *parse.ActionNode:
        // {{.Field}} or {{if}} or {{range}}
        return compileAction(n, ...)

    case *parse.TextNode:
        // Static text
        return []Construct{TextConstruct{Value: n.Text}}

    case *parse.ListNode:
        // List of nodes - recurse
        var constructs []Construct
        for _, node := range n.Nodes {
            constructs = append(constructs, compileConstruct(node, ...)...)
        }
        return constructs
    }
}
```

### Step 3: Hydrate Constructs (`tree_ast.go:600-1000`)

**Goal:** Fill constructs with actual data to generate tree

**Example data:**
```go
data := struct {
    Name        string
    ShowMessage bool
    Message     string
}{
    Name:        "World",
    ShowMessage: true,
    Message:     "Welcome!",
}
```

**Hydrated tree:**
```json
{
  "s": [
    "<div>\n    Hello ",
    "\n    ",
    "\n</div>"
  ],
  "0": "World",
  "1": {
    "s": ["\n        <p>", "</p>\n    "],
    "0": "Welcome!"
  }
}
```

**File:** `tree_ast.go`
```go
func hydrateConstruct(constructs []Construct, data interface{}, keyGen *keyGenerator, ...) treeNode {
    tree := treeNode{}
    statics := []string{}
    dynamicIdx := 0

    for _, construct := range constructs {
        switch c := construct.(type) {
        case TextConstruct:
            // Add to statics
            statics = append(statics, c.Value)

        case FieldConstruct:
            // Extract field value from data
            value, _ := getFieldValue(data, c.FieldName)
            tree[fmt.Sprintf("%d", dynamicIdx)] = value
            dynamicIdx++

        case ConditionalConstruct:
            // Evaluate condition
            if evaluateCondition(c.Condition, data) {
                // Hydrate true branch recursively
                subtree := hydrateConstruct(c.TrueBranch, data, keyGen, ...)
                tree[fmt.Sprintf("%d", dynamicIdx)] = subtree
                dynamicIdx++
            }
        }
    }

    tree["s"] = statics
    return tree
}
```

**Key insight:** Compilation happens once (or is cached), hydration happens on every render with new data.

## Tour 4: Tree Generation

**Goal:** Understand tree structure and keys

### Tree Structure (`tree.go`)

**Simple tree:**
```go
tree := treeNode{
    "s": []string{"<div>", "</div>"},
    "0": "Dynamic content",
}
```

**Nested tree:**
```go
tree := treeNode{
    "s": []string{"<div>", "</div>"},
    "0": treeNode{
        "s": []string{"<span>", "</span>"},
        "0": "Nested content",
    },
}
```

**Range tree:**
```go
tree := treeNode{
    "s": []string{"<ul>", "</ul>"},
    "0": []interface{}{
        treeNode{"s": []string{"<li>", "</li>"}, "0": "Item 1"},
        treeNode{"s": []string{"<li>", "</li>"}, "0": "Item 2"},
    },
}
```

### Key Generation (`tree.go:340-397`)

**Sequential keys:**
```go
keyGen := newKeyGenerator()
key1 := keyGen.nextKey() // "1"
key2 := keyGen.nextKey() // "2"
key3 := keyGen.nextKey() // "3"
```

**File:** `tree.go`
```go
type keyGenerator struct {
    counter      int
    usedKeys     map[string]bool
    fallbackKeys []string
    keyConfig    keyAttributeConfig
}

func (kg *keyGenerator) nextKey() string {
    kg.counter++
    return fmt.Sprintf("%d", kg.counter)
}
```

**Key insight:** Keys are simple sequential integers, reset on each render. Stable within a single render, deterministic across renders with same data.

### Fingerprinting (`tree.go:21-66`)

**MD5 hash for change detection:**
```go
fingerprint := calculateFingerprint(tree)
// Returns: "a1b2c3d4e5f6g7h8" (first 16 chars of MD5)
```

**File:** `tree.go`
```go
func calculateFingerprint(tree treeNode) string {
    hasher := md5.New()

    // Add statics
    if statics, exists := tree["s"]; exists {
        staticsJSON, _ := json.Marshal(statics)
        hasher.Write(staticsJSON)
    }

    // Add dynamics in sorted order
    for _, k := range sortedKeys(tree) {
        valueJSON, _ := json.Marshal(tree[k])
        hasher.Write([]byte(k))
        hasher.Write(valueJSON)
    }

    fullHash := hex.EncodeToString(hasher.Sum(nil))
    return fullHash[:16] // 64-bit hash
}
```

**Key insight:** Fingerprinting enables O(1) change detection vs O(n) tree diff.

## Tour 5: Client-Side Updates

**Goal:** Understand how client applies tree updates

### Step 1: WebSocket Connection (`client/livetemplate-client.ts:100-200`)

```typescript
class LiveTemplate {
    private ws: WebSocket;
    private treeCache: Map<string, any> = new Map();

    connect() {
        this.ws = new WebSocket('ws://localhost:8080/ws');

        this.ws.onmessage = (event) => {
            const update = JSON.parse(event.data);
            this.applyUpdate(update);
        };
    }

    sendAction(action: string, data: any) {
        this.ws.send(JSON.stringify({action, data}));
    }
}
```

### Step 2: Cache Statics (`client/livetemplate-client.ts:300-350`)

**First render:**
```typescript
function cacheStatics(tree: TreeNode) {
    if (tree.s) {
        const hash = hashStatics(tree.s);
        this.treeCache.set(hash, tree.s);
    }

    // Recurse for nested trees
    for (const key in tree) {
        if (key !== 's' && typeof tree[key] === 'object') {
            cacheStatics(tree[key]);
        }
    }
}
```

### Step 3: Apply Update (`client/livetemplate-client.ts:400-500`)

**Receive update:**
```json
{
  "tree": {"0": "New value"},
  "meta": {"wrapper_id": "lvt-abc123"}
}
```

**Apply:**
```typescript
function applyUpdate(update: UpdateResponse) {
    // Find wrapper element
    const wrapper = document.querySelector(
        `[data-lvt-id="${update.meta.wrapper_id}"]`
    );

    // Resolve tree (merge statics from cache + new dynamics)
    const fullTree = resolveTree(update.tree);

    // Build HTML from tree
    const html = treeToHTML(fullTree);

    // Update DOM
    wrapper.innerHTML = html;
}
```

### Step 4: Tree to HTML (`client/livetemplate-client.ts:600-700`)

```typescript
function treeToHTML(tree: TreeNode): string {
    if (typeof tree === 'string') {
        return tree; // Leaf value
    }

    if (Array.isArray(tree)) {
        return tree.map(treeToHTML).join(''); // Range
    }

    // Build from statics and dynamics
    const statics = tree.s || [];
    let html = '';

    for (let i = 0; i < statics.length; i++) {
        html += statics[i];

        if (tree[i.toString()]) {
            html += treeToHTML(tree[i.toString()]);
        }
    }

    return html;
}
```

**Key insight:** Client reconstructs HTML from tree structure. Statics from cache, dynamics from update.

## Advanced Topics

### Multi-Store Pattern (`mount.go:150-250`)

**Namespace actions by store:**
```go
stores := livetemplate.Stores{
    "counter": &CounterState{},
    "todos":   &TodosState{},
}

handler := livetemplate.HandleStores(tmpl, stores)
```

**Action routing:**
```html
<button lvt-click="counter.increment">+</button>
<button lvt-click="todos.add">Add Todo</button>
```

**Server routes:**
```go
func parseAction(action string) (store string, actualAction string) {
    parts := strings.SplitN(action, ".", 2)
    if len(parts) == 2 {
        return parts[0], parts[1] // "counter", "increment"
    }
    return "", parts[0] // "", "increment" (single store)
}
```

### Broadcasting (`broadcast.go`)

**Notify all connected clients:**
```go
type Broadcaster interface {
    Broadcast(data interface{}) error
}

func (s *State) Change(ctx *ActionContext) error {
    s.Counter++

    // Broadcast to all clients
    if b, ok := ctx.Data.Get("broadcaster").(Broadcaster); ok {
        b.Broadcast(s)
    }

    return nil
}
```

### Session Management (`session.go`)

**Per-session stores:**
```go
func Handle(store Store) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Clone store for this session
        sessionStore := cloneStore(store)

        // Initialize if needed
        if init, ok := sessionStore.(StoreInitializer); ok {
            init.Init()
        }

        // Handle requests with session store
        handleSession(w, r, sessionStore)
    })
}
```

### Validation (`action.go:44-55`)

**Bind and validate:**
```go
type TodoInput struct {
    Title string `json:"title" validate:"required,min=3"`
    Done  bool   `json:"done"`
}

func (s *TodosState) Change(ctx *ActionContext) error {
    var input TodoInput
    if err := ctx.BindAndValidate(&input, validator.New()); err != nil {
        return err // Returns validation errors to client
    }

    // Use validated input
    s.Todos = append(s.Todos, input)
    return nil
}
```

## Where to Go Next

1. **Experiment with examples**
   - Modify `examples/counter` to add new actions
   - Add validation to `examples/todos`

2. **Read tests**
   - `template_test.go` - Core functionality
   - `e2e_test.go` - Full rendering sequences
   - `client/livetemplate-client.test.ts` - Client tests

3. **Explore advanced features**
   - Broadcasting for real-time collaboration
   - Multi-store for complex apps
   - Custom validation rules

4. **Build something**
   - Start with the counter example
   - Add your own features
   - Share what you build!

---

For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md)

For contributing guidelines, see [CONTRIBUTING.md](../CONTRIBUTING.md)
