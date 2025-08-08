# Hybrid Fragment Caching Strategy

A technical exploration of sub-fragment caching for StateTemplate that maintains simplicity while dramatically improving bandwidth efficiency.

---

## Overview

This document explores a hybrid caching strategy that separates **static structure** (HTML tags, classes, attributes) from **dynamic content slots** (text, values, simple attributes). The server caches the static structure on the client and only sends updates to the dynamic slots.

## Core Concept

Instead of replacing entire fragments on every update, we identify which parts of a fragment are truly static vs dynamic, cache the static parts, and only transmit updates to the dynamic slots.

---

## Detailed Example: User Profile Card

### Current Approach (Traditional Fragment)

```html
<!-- Traditional fragment - sends entire HTML on every update -->
<section
  fir-id="user-profile"
  class="bg-white rounded-lg shadow-md p-6 max-w-md mx-auto"
>
  <div class="flex items-center space-x-4 mb-4">
    <img
      class="w-16 h-16 rounded-full border-2 border-gray-200"
      src="/avatars/{{.UserID}}.jpg"
      alt="{{.UserName}}'s avatar"
    />
    <div class="flex-1">
      <h2 class="text-xl font-bold text-gray-900">{{.UserName}}</h2>
      <p class="text-sm text-gray-500">{{.Title}}</p>
      <div class="flex items-center mt-1">
        <span
          class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium 
                     {{if .IsOnline}}bg-green-100 text-green-800{{else}}bg-gray-100 text-gray-800{{end}}"
        >
          {{if .IsOnline}}Online{{else}}Offline{{end}}
        </span>
      </div>
    </div>
  </div>

  <div class="grid grid-cols-3 gap-4 mb-4">
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900">{{.Stats.Posts}}</div>
      <div class="text-xs text-gray-500">Posts</div>
    </div>
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900">{{.Stats.Followers}}</div>
      <div class="text-xs text-gray-500">Followers</div>
    </div>
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900">{{.Stats.Following}}</div>
      <div class="text-xs text-gray-500">Following</div>
    </div>
  </div>

  <div class="flex space-x-2">
    <button
      class="flex-1 bg-blue-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-blue-700"
    >
      {{if .IsFollowing}}Unfollow{{else}}Follow{{end}}
    </button>
    <button
      class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
    >
      Message
    </button>
  </div>
</section>
```

**Problem**: Every update sends ~2KB of HTML, even though 90% of it (CSS classes, structure, static text) never changes.

### Hybrid Fragment Solution

```html
<!-- Hybrid fragment - structure cached, only slots updated -->
<section
  fir-id="user-profile"
  data-cache="hybrid"
  class="bg-white rounded-lg shadow-md p-6 max-w-md mx-auto"
>
  <div class="flex items-center space-x-4 mb-4">
    <img
      class="w-16 h-16 rounded-full border-2 border-gray-200"
      data-slot="avatar_src"
      src="/avatars/default.jpg"
      data-slot="avatar_alt"
      alt="User's avatar"
    />
    <div class="flex-1">
      <h2 class="text-xl font-bold text-gray-900" data-slot="user_name">
        Loading...
      </h2>
      <p class="text-sm text-gray-500" data-slot="user_title">Loading...</p>
      <div class="flex items-center mt-1">
        <span
          data-slot="status_badge"
          data-slot-class="status_badge_class"
          class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
        >
          Offline
        </span>
      </div>
    </div>
  </div>

  <div class="grid grid-cols-3 gap-4 mb-4">
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900" data-slot="posts_count">
        0
      </div>
      <div class="text-xs text-gray-500">Posts</div>
    </div>
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900" data-slot="followers_count">
        0
      </div>
      <div class="text-xs text-gray-500">Followers</div>
    </div>
    <div class="text-center">
      <div class="text-2xl font-bold text-gray-900" data-slot="following_count">
        0
      </div>
      <div class="text-xs text-gray-500">Following</div>
    </div>
  </div>

  <div class="flex space-x-2">
    <button
      class="flex-1 bg-blue-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-blue-700"
      data-slot="follow_button_text"
    >
      Follow
    </button>
    <button
      class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
    >
      Message
    </button>
  </div>
</section>
```

**Key Changes**:

- Added `data-cache="hybrid"` to enable hybrid mode
- Added `data-slot="slot_name"` attributes for dynamic content
- Added `data-slot-class="class_slot_name"` for dynamic class changes
- Provided default/placeholder content for all slots

---

## API Extensions

### Updated Update Model

```go
// Extended update model to support hybrid fragments
type Update struct {
    FragmentID  string                 `json:"fragment_id"`
    Type        string                 `json:"type"`                  // "replace", "hybrid_slots"
    HTML        string                 `json:"html,omitempty"`        // For traditional updates
    Slots       map[string]interface{} `json:"slots,omitempty"`       // For hybrid updates
    Action      string                 `json:"action,omitempty"`      // "replace", "append", "remove", etc.
    TargetID    string                 `json:"target_id,omitempty"`
    Timestamp   time.Time              `json:"timestamp"`
    HTMLHash    string                 `json:"html_hash,omitempty"`
    DataChanged []string               `json:"data_changed,omitempty"`
}

// New hybrid-specific update type
type HybridUpdate struct {
    FragmentID  string                 `json:"fragment_id"`
    Type        string                 `json:"type"`  // "hybrid_slots"
    Slots       map[string]interface{} `json:"slots"`
    Timestamp   time.Time              `json:"timestamp"`
    DataChanged []string               `json:"data_changed,omitempty"`
}
```

### Server-Side Template Processing

```go
// Server extracts slot values from template data
func (p *Page) renderHybridFragment(fragmentID string, data interface{}) *HybridUpdate {
    // Extract slot values based on template analysis and data
    slots := map[string]interface{}{
        "avatar_src":         fmt.Sprintf("/avatars/%s.jpg", data.UserID),
        "avatar_alt":         fmt.Sprintf("%s's avatar", data.UserName),
        "user_name":          data.UserName,
        "user_title":         data.Title,
        "status_badge":       data.IsOnline ? "Online" : "Offline",
        "status_badge_class": data.IsOnline ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800",
        "posts_count":        data.Stats.Posts,
        "followers_count":    data.Stats.Followers,
        "following_count":    data.Stats.Following,
        "follow_button_text": data.IsFollowing ? "Unfollow" : "Follow",
    }

    return &HybridUpdate{
        FragmentID: fragmentID,
        Type:       "hybrid_slots",
        Slots:      slots,
        Timestamp:  time.Now(),
        DataChanged: p.detectDataChanges(data),
    }
}

// Enhanced fragment detection for hybrid mode
func (p *Page) isHybridFragment(fragmentID string) bool {
    // Check if fragment has data-cache="hybrid" attribute
    return p.fragmentCache[fragmentID].IsHybrid
}

// Automatic slot detection from template
func (p *Page) extractSlots(fragmentHTML string) map[string]SlotInfo {
    slots := make(map[string]SlotInfo)

    // Parse HTML and find all data-slot attributes
    doc, _ := html.Parse(strings.NewReader(fragmentHTML))
    var findSlots func(*html.Node)
    findSlots = func(n *html.Node) {
        if n.Type == html.ElementNode {
            for _, attr := range n.Attr {
                if attr.Key == "data-slot" {
                    slots[attr.Val] = SlotInfo{
                        Type: "text_content",
                        Element: getNodePath(n),
                    }
                } else if attr.Key == "data-slot-class" {
                    slots[attr.Val] = SlotInfo{
                        Type: "class_attribute",
                        Element: getNodePath(n),
                    }
                } else if strings.HasPrefix(attr.Key, "data-slot-") {
                    attrName := strings.TrimPrefix(attr.Key, "data-slot-")
                    slots[attr.Val] = SlotInfo{
                        Type: fmt.Sprintf("%s_attribute", attrName),
                        Element: getNodePath(n),
                    }
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            findSlots(c)
        }
    }
    findSlots(doc)

    return slots
}

type SlotInfo struct {
    Type    string // "text_content", "class_attribute", "src_attribute", etc.
    Element string // CSS selector or node path
}
```

---

## Wire Protocol Comparison

### Traditional Fragment Update (2,147 bytes)

```json
{
  "fragment_id": "user-profile",
  "type": "replace",
  "html": "<section fir-id=\"user-profile\" class=\"bg-white rounded-lg shadow-md p-6 max-w-md mx-auto\">\n  <div class=\"flex items-center space-x-4 mb-4\">\n    <img class=\"w-16 h-16 rounded-full border-2 border-gray-200\" \n         src=\"/avatars/jane-doe.jpg\" \n         alt=\"Jane Doe's avatar\">\n    <div class=\"flex-1\">\n      <h2 class=\"text-xl font-bold text-gray-900\">Jane Doe</h2>\n      <p class=\"text-sm text-gray-500\">Senior Developer</p>\n      <div class=\"flex items-center mt-1\">\n        <span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800\">\n          Online\n        </span>\n      </div>\n    </div>\n  </div>\n  \n  <div class=\"grid grid-cols-3 gap-4 mb-4\">\n    <div class=\"text-center\">\n      <div class=\"text-2xl font-bold text-gray-900\">42</div>\n      <div class=\"text-xs text-gray-500\">Posts</div>\n    </div>\n    <div class=\"text-center\">\n      <div class=\"text-2xl font-bold text-gray-900\">1,234</div>\n      <div class=\"text-xs text-gray-500\">Followers</div>\n    </div>\n    <div class=\"text-center\">\n      <div class=\"text-2xl font-bold text-gray-900\">567</div>\n      <div class=\"text-xs text-gray-500\">Following</div>\n    </div>\n  </div>\n  \n  <div class=\"flex space-x-2\">\n    <button class=\"flex-1 bg-blue-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-blue-700\">\n      Unfollow\n    </button>\n    <button class=\"px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50\">\n      Message\n    </button>\n  </div>\n</section>",
  "action": "replace",
  "timestamp": "2025-08-08T14:30:26.123Z"
}
```

### Hybrid Fragment Update (312 bytes - 85% reduction!)

```json
{
  "fragment_id": "user-profile",
  "type": "hybrid_slots",
  "slots": {
    "avatar_src": "/avatars/jane-doe.jpg",
    "avatar_alt": "Jane Doe's avatar",
    "user_name": "Jane Doe",
    "user_title": "Senior Developer",
    "status_badge": "Online",
    "status_badge_class": "bg-green-100 text-green-800",
    "posts_count": 42,
    "followers_count": 1234,
    "following_count": 567,
    "follow_button_text": "Unfollow"
  },
  "timestamp": "2025-08-08T14:30:26.123Z",
  "data_changed": ["user_name", "is_online", "stats", "is_following"]
}
```

---

## Client-Side Implementation

### Enhanced Fragment Update Handler

```javascript
function handleFragmentUpdate(update) {
  const element = document.querySelector(`[fir-id="${update.fragment_id}"]`);
  if (!element) return;

  if (update.type === "hybrid_slots") {
    // Handle hybrid slot updates
    handleHybridSlotUpdate(element, update.slots);
  } else {
    // Handle traditional fragment replacement
    switch (update.action) {
      case "replace":
        element.outerHTML = update.html;
        break;
      case "append":
        element.insertAdjacentHTML("beforeend", update.html);
        break;
      case "remove":
        element.remove();
        break;
    }
  }
}

function handleHybridSlotUpdate(fragment, slots) {
  Object.entries(slots).forEach(([slotName, value]) => {
    updateSlot(fragment, slotName, value);
  });
}

function updateSlot(fragment, slotName, value) {
  const slotElement = fragment.querySelector(`[data-slot="${slotName}"]`);
  if (!slotElement) return;

  // Handle different slot types
  if (slotName.endsWith("_class")) {
    // Handle class updates
    const baseSlotName = slotName.replace("_class", "");
    const targetElement = fragment.querySelector(
      `[data-slot-class="${slotName}"]`
    );
    if (targetElement) {
      targetElement.className = value;
    }
  } else if (slotName.endsWith("_src")) {
    // Handle src attribute updates
    slotElement.src = value;
  } else if (slotName.endsWith("_alt")) {
    // Handle alt attribute updates
    slotElement.alt = value;
  } else if (slotName.endsWith("_href")) {
    // Handle href attribute updates
    slotElement.href = value;
  } else {
    // Handle text content updates (most common case)
    slotElement.textContent = value;
  }
}
```

### Advanced Slot Types

```html
<!-- Conditional visibility slots -->
<div
  data-slot="notification_badge"
  data-slot-visible="has_notifications"
  class="hidden absolute -top-1 -right-1 bg-red-500 text-white rounded-full w-5 h-5 text-xs flex items-center justify-center"
>
  <span data-slot="notification_count">0</span>
</div>

<!-- Multiple attribute slots -->
<a
  data-slot="profile_link"
  data-slot-href="profile_url"
  data-slot-title="profile_tooltip"
  href="#"
  title="View profile"
>
  <span data-slot="link_text">View Profile</span>
</a>

<!-- List slots for dynamic repeating content -->
<ul data-slot-list="recent_activities" data-slot-template="activity-item">
  <!-- This gets populated with slot data -->
</ul>

<template id="activity-item">
  <li class="flex items-center space-x-3 py-2">
    <img data-slot="activity_avatar" class="w-8 h-8 rounded-full" src="" />
    <div class="flex-1">
      <p data-slot="activity_text" class="text-sm text-gray-900"></p>
      <p data-slot="activity_time" class="text-xs text-gray-500"></p>
    </div>
  </li>
</template>
```

### Extended Client-Side Handler for Advanced Slots

```javascript
function updateSlot(fragment, slotName, value) {
  // Handle visibility slots
  if (slotName.endsWith("_visible")) {
    const baseSlotName = slotName.replace("_visible", "");
    const targetElement = fragment.querySelector(
      `[data-slot-visible="${slotName}"]`
    );
    if (targetElement) {
      if (value) {
        targetElement.classList.remove("hidden");
      } else {
        targetElement.classList.add("hidden");
      }
    }
    return;
  }

  // Handle list slots
  if (slotName.endsWith("_list")) {
    const listElement = fragment.querySelector(
      `[data-slot-list="${slotName}"]`
    );
    const templateId = listElement?.getAttribute("data-slot-template");
    const template = document.getElementById(templateId);

    if (listElement && template && Array.isArray(value)) {
      // Clear existing content
      listElement.innerHTML = "";

      // Create elements from template
      value.forEach((itemData) => {
        const clone = template.content.cloneNode(true);

        // Populate slots in the cloned template
        Object.entries(itemData).forEach(([key, val]) => {
          const slotEl = clone.querySelector(`[data-slot="${key}"]`);
          if (slotEl) {
            slotEl.textContent = val;
          }
        });

        listElement.appendChild(clone);
      });
    }
    return;
  }

  // Standard slot handling
  const slotElement = fragment.querySelector(`[data-slot="${slotName}"]`);
  if (!slotElement) return;

  // Handle attribute slots
  if (slotName.includes("_")) {
    const parts = slotName.split("_");
    const attrName = parts[parts.length - 1];

    if (["src", "alt", "href", "title", "data"].includes(attrName)) {
      slotElement.setAttribute(attrName, value);
      return;
    }
  }

  // Handle class slots
  const classSlotElement = fragment.querySelector(
    `[data-slot-class="${slotName}"]`
  );
  if (classSlotElement) {
    classSlotElement.className = value;
    return;
  }

  // Default: text content
  slotElement.textContent = value;
}
```

---

## Backward Compatibility

The system maintains full backward compatibility:

```html
<!-- Traditional fragment - works exactly as before -->
<section fir-id="simple-counter">
  <h2>Counter: {{.Count}}</h2>
  <button onclick="increment()">+</button>
</section>

<!-- Hybrid fragment - opt-in optimization -->
<section fir-id="user-profile" data-cache="hybrid">
  <!-- Slot-based content here -->
</section>

<!-- Mixed approach in same page -->
<div fir-id="complex-dashboard" data-cache="hybrid">
  <!-- Hybrid slots for frequently changing data -->
</div>
<div fir-id="static-sidebar">
  <!-- Traditional fragment for infrequent updates -->
</div>
```

## Cache State Management

Hybrid fragments extend the existing cache tracking:

```go
type SessionCacheState struct {
    FragmentHashes map[string]string  // fragment_id -> current_hash_on_client
    HybridSlots    map[string]map[string]interface{} // fragment_id -> slot_values
    LastSync       time.Time
    TabID          string
    IsDirty        bool
}

// Track hybrid slot state
func (p *Page) updateHybridSlotState(fragmentID string, slots map[string]interface{}) {
    if p.cacheState.HybridSlots == nil {
        p.cacheState.HybridSlots = make(map[string]map[string]interface{})
    }
    p.cacheState.HybridSlots[fragmentID] = slots
}

// Check if slot update is needed
func (p *Page) isSlotUpdateNeeded(fragmentID, slotName string, newValue interface{}) bool {
    currentSlots := p.cacheState.HybridSlots[fragmentID]
    if currentSlots == nil {
        return true
    }

    currentValue, exists := currentSlots[slotName]
    return !exists || !reflect.DeepEqual(currentValue, newValue)
}
```

---

## Performance Analysis

### Bandwidth Comparison

| Scenario            | Traditional | Hybrid      | Savings |
| ------------------- | ----------- | ----------- | ------- |
| User Profile Update | 2,147 bytes | 312 bytes   | 85%     |
| Status Change Only  | 2,147 bytes | 89 bytes    | 96%     |
| Counter Increment   | 156 bytes   | 45 bytes    | 71%     |
| Large Dashboard     | 8,934 bytes | 1,234 bytes | 86%     |

### CPU and Memory Benefits

1. **Reduced DOM Manipulation**: Only specific text nodes/attributes updated vs full fragment replacement
2. **State Preservation**: Focus, scroll position, CSS transitions preserved
3. **Reduced Parsing**: No HTML parsing on client for hybrid updates
4. **Memory Efficiency**: Existing DOM nodes reused vs recreated

### Network Benefits

1. **Smaller Payloads**: 70-96% reduction in typical scenarios
2. **Faster Transmission**: Especially beneficial on mobile/slow connections
3. **Reduced Latency**: Smaller packets = faster delivery
4. **Better Compression**: JSON compresses better than HTML

---

## Use Cases

### Perfect For:

- **User profiles/cards** with stable layout, dynamic content
- **Dashboard widgets** with consistent structure
- **Data tables** with dynamic rows but stable columns
- **Status indicators** and badges
- **Shopping cart items** with quantity/price changes
- **Chat messages** with stable message structure
- **Social media posts** with consistent layout

### Not Ideal For:

- **Completely dynamic content** where structure changes frequently
- **Complex nested conditionals** that affect layout
- **One-time rendering** scenarios
- **Simple static content** that rarely changes

---

## Implementation Strategy

### Phase 1: Core Infrastructure

1. Extend Update model to support hybrid type
2. Add slot detection and extraction logic
3. Implement basic client-side slot updating
4. Add hybrid cache state tracking

### Phase 2: Advanced Features

1. Support for attribute slots (src, href, class, etc.)
2. Conditional visibility slots
3. List/template slots for repeating content
4. Auto-detection of slot opportunities in templates

### Phase 3: Developer Experience

1. Template analysis tools to suggest hybrid opportunities
2. Performance monitoring and metrics
3. Developer tools for debugging slot updates
4. Documentation and migration guides

### Phase 4: Optimizations

1. Slot batching for multiple updates
2. Compressed slot update format
3. Predictive slot caching
4. Integration with existing fragment optimization

---

## Questions for Analysis

1. **Complexity vs Benefit**: Does the 85% bandwidth reduction justify the additional complexity?

2. **Developer Adoption**: How easy is it for developers to identify and convert fragments to hybrid mode?

3. **Template Tooling**: Should we auto-detect hybrid opportunities or require manual annotation?

4. **Error Handling**: What happens when slot updates fail or client state becomes inconsistent?

5. **Performance Edge Cases**: Are there scenarios where hybrid fragments perform worse than traditional ones?

6. **Migration Path**: How do we help existing applications gradually adopt hybrid fragments?

7. **Testing**: How do we test hybrid fragments effectively, both on server and client side?

8. **Debugging**: What tools do developers need to debug slot update issues?

---

## Next Steps

This document serves as a comprehensive exploration of hybrid fragment caching. Before integration into the main design document, we should consider:

1. **Prototype Implementation**: Build a minimal working version to validate concepts
2. **Performance Benchmarks**: Measure actual bandwidth and CPU improvements
3. **Developer Feedback**: Get input from potential users on API design
4. **Edge Case Analysis**: Identify potential failure modes and mitigation strategies
5. **Documentation**: Create comprehensive guides and examples

The strategy offers significant potential benefits while maintaining the core simplicity of the fragment-based approach. The opt-in nature ensures backward compatibility and allows for gradual adoption.
