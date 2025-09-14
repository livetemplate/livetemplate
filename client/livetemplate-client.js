/**
 * LiveTemplate Client Library
 * 
 * A unified WebSocket-based client for LiveTemplate tree-based diff updates.
 * Works exclusively with the diff.Update format from the Go backend.
 */

import morphdom from 'morphdom';

class LiveTemplateClient {
  constructor(options = {}) {
    this.wsUrl = options.wsUrl || this.buildWebSocketUrl();
    this.pageToken = null;
    this.ws = null;
    this.staticCache = new Map(); // In-memory cache for current session
    this.hashCache = new Map(); // Track hashes for cache validation
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
    this.reconnectDelay = options.reconnectDelay || 1000;
    
    // Load persistent cache from localStorage
    this.loadPersistentCache();
    
    // Callbacks
    this.onOpen = options.onOpen || (() => {});
    this.onClose = options.onClose || (() => {});
    this.onError = options.onError || ((error) => console.error("LiveTemplate error:", error));
    this.onFragmentUpdate = options.onFragmentUpdate || null;
  }

  loadPersistentCache() {
    try {
      const cacheData = localStorage.getItem('livetemplate-cache');
      if (cacheData) {
        const parsed = JSON.parse(cacheData);
        // Restore static cache
        if (parsed.statics) {
          Object.entries(parsed.statics).forEach(([id, value]) => {
            this.staticCache.set(id, value);
          });
        }
        // Restore hash cache
        if (parsed.hashes) {
          Object.entries(parsed.hashes).forEach(([id, hash]) => {
            this.hashCache.set(id, hash);
          });
        }
        console.log(`Loaded ${this.staticCache.size} cached fragments from localStorage`);
      }
    } catch (error) {
      console.warn('Failed to load cache from localStorage:', error);
      // Clear corrupted cache
      localStorage.removeItem('livetemplate-cache');
    }
  }

  savePersistentCache() {
    try {
      const cacheData = {
        statics: Object.fromEntries(this.staticCache),
        hashes: Object.fromEntries(this.hashCache),
        timestamp: Date.now()
      };
      localStorage.setItem('livetemplate-cache', JSON.stringify(cacheData));
    } catch (error) {
      console.warn('Failed to save cache to localStorage:', error);
      // If quota exceeded, clear old cache and try again
      if (error.name === 'QuotaExceededError') {
        localStorage.removeItem('livetemplate-cache');
        try {
          localStorage.setItem('livetemplate-cache', JSON.stringify(cacheData));
        } catch (retryError) {
          console.error('Failed to save cache even after clearing:', retryError);
        }
      }
    }
  }

  buildWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/ws`;
  }

  connect(token) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      // Already connected
      return;
    }

    this.pageToken = token;
    const url = `${this.wsUrl}?token=${token}`;
    
    // Connecting to LiveTemplate
    this.ws = new WebSocket(url);
    this.setupEventHandlers();
  }

  setupEventHandlers() {
    this.ws.onopen = () => {
      // WebSocket connection established
      this.reconnectAttempts = 0;
      
      this.onOpen();
    };

    this.ws.onclose = (event) => {
      // WebSocket closed
      this.onClose(event);
      
      if (event.code !== 1000) { // Not a normal closure
        this.attemptReconnect();
      }
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      this.onError(error);
    };

    this.ws.onmessage = (event) => {
      try {
        const fragments = JSON.parse(event.data);
        // Received fragments
        this.applyFragments(fragments);
      } catch (error) {
        console.error("Error parsing fragment data:", error);
      }
    };
  }

  applyFragments(fragments) {
    // Handle both array format and object format
    const fragmentArray = Array.isArray(fragments) ? fragments : 
      Object.entries(fragments).map(([id, data]) => ({ id, data }));

    fragmentArray.forEach(fragment => {
      try {
        this.applyDiffUpdate(fragment);
      } catch (error) {
        console.error(`Error applying fragment ${fragment.id}:`, error);
      }
    });
  }

  applyDiffUpdate(fragment) {
    // Try to find element by data-lvt-id first, then by regular id
    let element = document.querySelector(`[data-lvt-id="${fragment.id}"]`);
    if (!element) {
      // Handle numeric IDs safely by using attribute selector instead of # selector
      if (/^\d/.test(fragment.id)) {
        element = document.querySelector(`[id="${fragment.id}"]`);
      } else {
        element = document.querySelector(`#${fragment.id}`);
      }
    }
    if (!element) {
      console.warn(`Element with data-lvt-id="${fragment.id}" or id="${fragment.id}" not found`);
      return;
    }

    if (!fragment.data || typeof fragment.data !== 'object') {
      console.warn(`Invalid diff.Update data for fragment ${fragment.id}`);
      return;
    }

    const update = fragment.data;
    
    // Check hash to determine if cache is stale
    const cachedHash = this.hashCache.get(fragment.id);
    const currentHash = update.h;
    
    if (currentHash && cachedHash && currentHash !== cachedHash) {
      // Hash mismatch - cache is stale, clear it
      console.log(`Cache invalidated for fragment ${fragment.id}: hash ${cachedHash} → ${currentHash}`);
      this.staticCache.delete(fragment.id);
      this.hashCache.delete(fragment.id);
    }
    
    // Handle statics (s) - cache them for future updates
    if (update.s && Array.isArray(update.s)) {
      this.staticCache.set(fragment.id, update.s);
      if (currentHash) {
        this.hashCache.set(fragment.id, currentHash);
      }
      // Save to persistent storage
      this.savePersistentCache();
    }

    // Get cached statics for this fragment
    const statics = this.staticCache.get(fragment.id) || [];
    
    // Reconstruct content from statics and dynamics
    const content = this.reconstructFromDiffUpdate(statics, update);
    
    if (content === null) {
      // No content to update
      return;
    }

    // Special handling for input elements
    if (element.tagName === 'INPUT' && (element.type === 'text' || element.type === 'email' || element.type === 'password' || !element.type)) {
      // For input elements, extract the value attribute from the reconstructed content
      const tempDiv = document.createElement('div');
      tempDiv.innerHTML = content;
      const newInput = tempDiv.querySelector('input');
      if (newInput) {
        const newValue = newInput.getAttribute('value') || '';
        // Direct input value update
        element.value = newValue;
        element.setAttribute('value', newValue);
      }
    } else {
      // Apply the update using morphdom for other elements
      this.applyContentUpdate(element, content, fragment.id);
    }

    // Call user callback if provided
    if (this.onFragmentUpdate) {
      this.onFragmentUpdate(fragment, element);
    }
  }

  reconstructFromDiffUpdate(statics, update) {
    // If no statics cached and none provided, can't reconstruct
    if (!statics.length && (!update.s || !update.s.length)) {
      console.warn("No static segments available for reconstruction");
      return null;
    }

    // Use provided statics or cached ones
    const staticSegments = update.s || statics;
    
    // If no dynamics, just join statics
    const hasDynamics = Object.keys(update).some(key => 
      key !== 's' && key !== 'h' && key !== 'S' && key !== 'H'
    );

    if (!hasDynamics) {
      return staticSegments.join('');
    }

    // Interleave statics and dynamics
    let result = '';
    for (let i = 0; i < staticSegments.length; i++) {
      result += staticSegments[i];
      
      // Add dynamic value if it exists (dynamics are keyed by position)
      if (i < staticSegments.length - 1) {
        const dynamicValue = update[i.toString()];
        if (dynamicValue !== undefined) {
          result += dynamicValue;
        }
      }
    }

    return result;
  }

  applyContentUpdate(element, newContent, fragmentId) {
    // Check if element is a void/self-closing element (meta, link, input, img, br, hr, etc.)
    const voidElements = ['META', 'LINK', 'INPUT', 'IMG', 'BR', 'HR', 'AREA', 'BASE', 'COL', 'EMBED', 'SOURCE', 'TRACK', 'WBR'];
    if (voidElements.includes(element.tagName)) {
      // For void elements, just update attributes directly from the new content
      const attrMatch = newContent.match(/<[^>]+>/);
      if (attrMatch) {
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = attrMatch[0];
        const tempElement = tempDiv.firstChild;
        if (tempElement) {
          // Copy all attributes from temp element to actual element
          Array.from(tempElement.attributes).forEach(attr => {
            element.setAttribute(attr.name, attr.value);
          });
        }
      }
      return;
    }

    // Parse the new content as a complete element
    const parser = new DOMParser();
    const doc = parser.parseFromString(newContent, 'text/html');
    let tempElement = doc.body.firstChild;

    // If not found in body, check head (for elements like meta, link, etc.)
    if (!tempElement) {
      tempElement = doc.head.firstChild;
    }

    if (!tempElement) {
      console.error(`Failed to parse content for fragment ${fragmentId}:`, newContent);
      return;
    }

    // Fragment update in progress

    // Use morphdom to efficiently update only what changed
    morphdom(element, tempElement, {
      onBeforeElUpdated: (fromEl) => {
        // Preserve focus if the element is focused
        if (fromEl === document.activeElement) {
          return true;
        }
        return true;
      },
      childrenOnly: false // FIXED: Allow element attributes to be updated too
    });

    // Post-morphdom: Sync all input field values to match their attributes
    // This ensures that input.value property matches the value attribute after DOM update
    const inputs = element.querySelectorAll('input[type="text"], input[type="email"], input[type="password"], input:not([type])');
    inputs.forEach(input => {
      const attrValue = input.getAttribute('value') || '';
      if (input.value !== attrValue) {
        // Sync input value with attribute
        input.value = attrValue;
      }
    });

    // Update completed
  }


  sendAction(action, data = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn("WebSocket not connected, cannot send action");
      return;
    }

    // Send action message with token if available
    const message = {
      action: action,
      token: this.pageToken || "",
      data: data
    };

    console.log(`Sending action "${action}" with token`);
    this.ws.send(JSON.stringify(message));
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error(`Max reconnection attempts (${this.maxReconnectAttempts}) reached`);
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1); // Exponential backoff
    
    // Attempting reconnection
    
    setTimeout(() => {
      if (this.pageToken) {
        this.connect(this.pageToken);
      }
    }, delay);
  }

  disconnect() {
    if (this.ws) {
      this.ws.close(1000, "Client requested disconnect");
      this.ws = null;
    }
    this.staticCache.clear();
  }
}

// Auto-initialization function
function autoInit() {
  // Only auto-init in browser environment
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }

  // Auto-initialize when DOM is ready
  function initLiveTemplate() {
    // Look for livetemplate token
    const tokenMeta = document.querySelector('meta[name="livetemplate-token"]');
    if (!tokenMeta) {
      console.warn('LiveTemplate: No token found. Add <meta name="livetemplate-token" content="{{.Token}}"> to your template.');
      return;
    }

    const token = tokenMeta.getAttribute('content');
    if (!token) {
      console.warn('LiveTemplate: Empty token found.');
      return;
    }

    // Create client with default options
    const client = new LiveTemplateClient({
      onOpen: () => {},
      onError: (error) => console.error('LiveTemplate error:', error)
    });

    // Expose client globally for debugging
    window.liveTemplateClient = client;

    // Connect to server
    client.connect(token);

    // Auto-handle action buttons
    document.addEventListener('click', function(e) {
      const action = e.target.getAttribute('data-lvt-action');
      if (action) {
        e.preventDefault(); // Prevent default button behavior
        
        let actionData = {};
        
        // Check for parameters in data-lvt-params attribute
        const paramsAttr = e.target.getAttribute('data-lvt-params');
        if (paramsAttr) {
          try {
            const params = JSON.parse(paramsAttr);
            actionData = { ...actionData, ...params };
            // Captured params from data attributes
          } catch (error) {
            console.warn(`Failed to parse data-lvt-params: ${paramsAttr}`, error);
          }
        }
        
        // Check if the button specifies an element to capture data from
        const elementId = e.target.getAttribute('data-lvt-element');
        if (elementId) {
          const element = document.getElementById(elementId);
          if (element) {
            // Capture the element's value based on its type
            let value = '';
            if (element.type === 'checkbox' || element.type === 'radio') {
              value = element.checked;
            } else if (element.value !== undefined) {
              value = element.value;
            } else {
              value = element.textContent || element.innerText || '';
            }
            
            // Use the element's name attribute as the key, fallback to id
            const key = element.name || elementId;
            actionData[key] = value;
            
            // Captured data from form element
          } else {
            console.warn(`Element with id "${elementId}" not found`);
          }
        }
        
        client.sendAction(action, actionData);
      }
    });

    // LiveTemplate auto-initialized
  }

  // Initialize when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initLiveTemplate);
  } else {
    // DOM already loaded
    initLiveTemplate();
  }
}

// For IIFE bundle, assign to window directly to avoid module wrapper issues
if (typeof window !== 'undefined') {
  window.LiveTemplateClient = LiveTemplateClient;
  
  // Auto-initialize by default
  autoInit();
}

// Still export for ES module compatibility
export default LiveTemplateClient;