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
    this.staticCache = new Map(); // Cache static segments by fragment ID
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
    this.reconnectDelay = options.reconnectDelay || 1000;
    
    // Callbacks
    this.onOpen = options.onOpen || (() => console.log("🔌 LiveTemplate connected"));
    this.onClose = options.onClose || (() => console.log("🔌 LiveTemplate disconnected"));
    this.onError = options.onError || ((error) => console.error("❌ LiveTemplate error:", error));
    this.onFragmentUpdate = options.onFragmentUpdate || null;
  }

  buildWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/ws`;
  }

  connect(token) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      console.log("Already connected");
      return;
    }

    this.pageToken = token;
    const url = `${this.wsUrl}?token=${token}`;
    
    console.log("🔌 Connecting to LiveTemplate:", url);
    this.ws = new WebSocket(url);
    this.setupEventHandlers();
  }

  setupEventHandlers() {
    this.ws.onopen = () => {
      console.log("✅ LiveTemplate WebSocket connection established");
      this.reconnectAttempts = 0;
      this.onOpen();
    };

    this.ws.onclose = (event) => {
      console.log(`🔌 LiveTemplate WebSocket closed (code: ${event.code})`);
      this.onClose(event);
      
      if (event.code !== 1000) { // Not a normal closure
        this.attemptReconnect();
      }
    };

    this.ws.onerror = (error) => {
      console.error("❌ LiveTemplate WebSocket error:", error);
      this.onError(error);
    };

    this.ws.onmessage = (event) => {
      try {
        const fragments = JSON.parse(event.data);
        console.log("📦 Received fragments:", fragments);
        this.applyFragments(fragments);
      } catch (error) {
        console.error("❌ Error parsing fragment data:", error, event.data);
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
        console.error(`❌ Error applying fragment ${fragment.id}:`, error);
      }
    });
  }

  applyDiffUpdate(fragment) {
    const element = document.querySelector(`[lvt-id="${fragment.id}"]`);
    if (!element) {
      console.warn(`⚠️ Element with lvt-id="${fragment.id}" not found`);
      return;
    }

    if (!fragment.data || typeof fragment.data !== 'object') {
      console.warn(`⚠️ Invalid diff.Update data for fragment ${fragment.id}`);
      return;
    }

    const update = fragment.data;
    
    // Handle statics (s) - cache them for future updates
    if (update.s && Array.isArray(update.s)) {
      this.staticCache.set(fragment.id, update.s);
      console.log(`💾 Cached ${update.s.length} static segments for fragment ${fragment.id}`);
    }

    // Get cached statics for this fragment
    const statics = this.staticCache.get(fragment.id) || [];
    
    // Reconstruct content from statics and dynamics
    const content = this.reconstructFromDiffUpdate(statics, update);
    
    if (content === null) {
      console.log(`⏸️ Fragment ${fragment.id}: No content to update`);
      return;
    }

    // Apply the update using morphdom
    this.applyContentUpdate(element, content, fragment.id);

    // Call user callback if provided
    if (this.onFragmentUpdate) {
      this.onFragmentUpdate(fragment, element);
    }
  }

  reconstructFromDiffUpdate(statics, update) {
    // If no statics cached and none provided, can't reconstruct
    if (!statics.length && (!update.s || !update.s.length)) {
      console.warn("⚠️ No static segments available for reconstruction");
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
    // Create a temporary element with the new content
    const tempElement = element.cloneNode(false);
    tempElement.innerHTML = newContent;

    console.log(`🔄 Updating fragment ${fragmentId}:`, {
      element: element.tagName + (element.className ? '.' + element.className : ''),
      oldContent: element.innerHTML.substring(0, 50) + (element.innerHTML.length > 50 ? '...' : ''),
      newContent: newContent.substring(0, 50) + (newContent.length > 50 ? '...' : '')
    });

    // Use morphdom to efficiently update only what changed
    morphdom(element, tempElement, {
      onBeforeElUpdated: (fromEl, toEl) => {
        // Preserve focus if the element is focused
        if (fromEl === document.activeElement) {
          return true;
        }
        return true;
      },
      childrenOnly: true // Only update children, preserve the element itself
    });

    console.log(`✅ Fragment ${fragmentId}: Applied diff update`);
  }

  sendAction(action, data = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn("⚠️ WebSocket not connected, cannot send action");
      return;
    }

    // Include cache information to optimize server responses
    const cachedFragments = Array.from(this.staticCache.keys());
    const message = {
      action: action,
      cache: cachedFragments,
      ...data
    };

    console.log("📤 Sending action:", message);
    console.log(`📦 Cache info: ${cachedFragments.length} cached fragments: [${cachedFragments.join(', ')}]`);
    this.ws.send(JSON.stringify(message));
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error(`❌ Max reconnection attempts (${this.maxReconnectAttempts}) reached`);
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1); // Exponential backoff
    
    console.log(`🔄 Reconnection attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);
    
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
      console.warn('⚠️ LiveTemplate: No token found. Add <meta name="livetemplate-token" content="{{.Token}}"> to your template.');
      return;
    }

    const token = tokenMeta.getAttribute('content');
    if (!token) {
      console.warn('⚠️ LiveTemplate: Empty token found.');
      return;
    }

    // Create client with default options
    const client = new LiveTemplateClient({
      onOpen: () => console.log('🔌 Connected to LiveTemplate'),
      onError: (error) => console.error('❌ LiveTemplate error:', error)
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
        client.sendAction(action);
      }
    });

    console.log('✅ LiveTemplate auto-initialized');
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