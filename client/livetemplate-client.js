class LiveTemplateClient {
  constructor(options = {}) {
    this.ws = null;
    this.pageToken = options.presetToken || null;
    this.staticCache = {};
    this.port = options.port || window.location.port || "8080";
    this.host = options.host || "localhost";
    this.protocol = options.protocol || "ws";
    this.endpoint = options.endpoint || "/ws";
    this.onOpen = options.onOpen || (() => console.log("WebSocket connected"));
    this.onClose = options.onClose || (() => console.log("WebSocket disconnected"));
    this.onError = options.onError || ((error) => console.error("WebSocket error:", error));
    this.onMessage = options.onMessage || null;
    
    // Initialize persistent cache
    this.initializePersistentCache();
  }

  // Initialize persistent cache based on page token and URL parameters
  initializePersistentCache() {
    // Check for cache=empty parameter
    const urlParams = new URLSearchParams(window.location.search);
    const forceClear = urlParams.get('cache') === 'empty';
    
    if (forceClear) {
      console.log("Force clearing cache due to ?cache=empty");
      this.clearPersistentCache();
      return;
    }

    if (!this.pageToken) {
      return; // No token yet, can't load cache
    }

    // Load cached statics for this page token
    this.loadStaticsFromCache();
  }

  // Load statics from localStorage for current page token
  loadStaticsFromCache() {
    if (!this.pageToken) return;

    try {
      const cacheKey = `livetemplate_cache_${this.pageToken}`;
      const cached = localStorage.getItem(cacheKey);
      
      if (cached) {
        const cacheData = JSON.parse(cached);
        this.staticCache = cacheData.statics || {};
        console.log(`Loaded ${Object.keys(this.staticCache).length} cached statics for token ${this.pageToken}`);
      } else {
        console.log(`No cached statics found for token ${this.pageToken}`);
      }
    } catch (e) {
      console.warn("Failed to load statics from cache:", e);
      this.staticCache = {};
    }
  }

  // Save statics to localStorage for current page token
  saveStaticsToCache() {
    if (!this.pageToken || Object.keys(this.staticCache).length === 0) return;

    try {
      const cacheKey = `livetemplate_cache_${this.pageToken}`;
      const cacheData = {
        token: this.pageToken,
        timestamp: Date.now(),
        statics: this.staticCache
      };
      
      localStorage.setItem(cacheKey, JSON.stringify(cacheData));
      console.log(`Saved ${Object.keys(this.staticCache).length} statics to cache for token ${this.pageToken}`);
    } catch (e) {
      console.warn("Failed to save statics to cache:", e);
    }
  }

  // Clear all cached statics
  clearPersistentCache() {
    try {
      // Remove all livetemplate cache entries
      const keys = Object.keys(localStorage).filter(key => key.startsWith('livetemplate_cache_'));
      keys.forEach(key => localStorage.removeItem(key));
      console.log(`Cleared ${keys.length} cache entries`);
    } catch (e) {
      console.warn("Failed to clear persistent cache:", e);
    }
    this.staticCache = {};
  }

  // Check if we have cached statics
  hasCachedStatics() {
    return Object.keys(this.staticCache).length > 0;
  }

  connect(customUrl = null) {
    // Load cache before connecting if we have a token
    if (this.pageToken && Object.keys(this.staticCache).length === 0) {
      this.loadStaticsFromCache();
    }
    
    // Build URL with cache status in query parameters
    let url = customUrl;
    if (!url) {
      url = `${this.protocol}://${this.host}:${this.port}${this.endpoint}`;
    }
    
    // Add cache status to URL if not already included
    if (this.pageToken && !url.includes('has_cache=')) {
      const urlObj = new URL(url, `${this.protocol}://${this.host}:${this.port}`);
      urlObj.searchParams.set('has_cache', this.hasCachedStatics() ? 'true' : 'false');
      
      // Add cached fragment IDs if we have cache
      if (this.hasCachedStatics()) {
        urlObj.searchParams.set('cached_fragments', Object.keys(this.staticCache).join(','));
      }
      
      url = urlObj.toString();
      console.log(`Connecting with cache status: ${this.hasCachedStatics() ? 'HAS_CACHE' : 'NO_CACHE'} (${Object.keys(this.staticCache).length} fragments)`);
    }
    
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      // If we have a preset token, log it
      if (this.pageToken) {
        console.log("Using preset token:", this.pageToken);
      }
      this.onOpen();
    };

    this.ws.onmessage = (event) => {
      console.log("Received:", event.data);
      const message = JSON.parse(event.data);

      // Check if message is an array of fragments
      if (Array.isArray(message)) {
        this.updateFragments(message);
      } else if (message.type === "page_token") {
        this.pageToken = message.token;
        console.log("Page token received:", this.pageToken);
      }

      if (this.onMessage) {
        this.onMessage(message);
      }
    };

    this.ws.onclose = () => {
      this.onClose();
    };

    this.ws.onerror = (error) => {
      this.onError(error);
    };
  }

  sendAction(action, data = {}) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const message = {
        type: "action",
        action: action,
        data: data,
      };
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn("WebSocket not connected");
    }
  }

  updateFragments(fragments) {
    let staticsUpdated = false;
    
    fragments.forEach((fragment) => {
      console.log("Updating fragment:", fragment);

      // Cache statics if provided
      if (fragment.data && fragment.data.s) {
        this.staticCache[fragment.id] = fragment.data.s;
        staticsUpdated = true;
        console.log(`Cached statics for fragment ${fragment.id}`);
      }

      // Find element by LiveTemplate ID (lvt-id)
      let element = null;
      if (fragment.id) {
        element = document.querySelector(`[lvt-id="${fragment.id}"]`);
        console.log("Found element for", fragment.id, ":", element);
      }

      if (element && fragment.data) {
        this.updateElementFromTreeData(element, fragment.data, fragment.id);
      }
    });
    
    // Save updated statics to persistent cache
    if (staticsUpdated) {
      this.saveStaticsToCache();
    }
  }

  updateElementFromTreeData(element, treeData, fragmentId) {
    // Generic function to reconstruct content from tree-based fragment data
    // Works with any template structure by using statics and dynamics
    
    if (!treeData) {
      console.log("No tree data found");
      return;
    }
    
    // Use cached statics if not provided in this update
    const statics = treeData.s || this.staticCache[fragmentId] || [""];
    const content = this.reconstructContent(statics, treeData);
    
    console.log("Reconstructed content:", content);
    
    // Check if this looks like HTML content with attributes
    if (content.includes('<') && content.includes('>')) {
      // Parse HTML content and apply to element
      const tempDiv = document.createElement('div');
      tempDiv.innerHTML = content;
      
      if (tempDiv.children.length === 1) {
        // Single element - copy attributes and inner content
        const newElement = tempDiv.children[0];
        
        // Copy all attributes from reconstructed element
        Array.from(newElement.attributes).forEach(attr => {
          element.setAttribute(attr.name, attr.value);
        });
        
        // Copy inner content if it exists
        if (newElement.innerHTML !== element.innerHTML) {
          element.innerHTML = newElement.innerHTML;
        }
        
        console.log("Updated element with HTML content and attributes");
      } else {
        // Multiple elements or complex structure - replace innerHTML
        element.innerHTML = content;
        console.log("Updated element innerHTML with complex content");
      }
    } else {
      // Plain text content
      element.textContent = content;
      console.log("Updated element textContent:", content);
    }
  }

  reconstructContent(statics, dynamics) {
    // Reconstruct content from static parts and dynamic values
    let result = "";
    
    for (let i = 0; i < statics.length; i++) {
      result += statics[i];
      
      // Add dynamic value if it exists
      if (i < statics.length - 1 && dynamics[i.toString()] !== undefined) {
        const dynamicValue = dynamics[i.toString()];
        
        if (typeof dynamicValue === 'object' && dynamicValue !== null) {
          // Nested tree data - recursively reconstruct
          result += this.reconstructContent(dynamicValue.s || [""], dynamicValue);
        } else {
          // Simple value
          result += dynamicValue;
        }
      }
    }
    
    return result;
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
      this.pageToken = null;
      this.staticCache = {};
    }
  }

  isConnected() {
    return this.ws && this.ws.readyState === WebSocket.OPEN;
  }

  getPageToken() {
    return this.pageToken;
  }
}

// Auto-initialization function
function autoInitializeLiveTemplate() {
  // Look for embedded page token in script tags or meta tags
  let pageToken = null;
  
  // Method 1: Look for token in script tag content
  const scripts = document.querySelectorAll('script');
  for (const script of scripts) {
    const content = script.textContent || script.innerText;
    const tokenMatch = content.match(/LIVETEMPLATE_TOKEN['"]\s*:\s*['"]([^'"]+)['"]/);
    if (tokenMatch) {
      pageToken = tokenMatch[1];
      break;
    }
  }
  
  // Method 2: Look for meta tag
  if (!pageToken) {
    const metaTag = document.querySelector('meta[name="livetemplate-token"]');
    if (metaTag) {
      pageToken = metaTag.getAttribute('content');
    }
  }
  
  // Method 3: Look for data attribute on body
  if (!pageToken) {
    pageToken = document.body.getAttribute('data-livetemplate-token');
  }
  
  if (pageToken && pageToken !== 'PAGE_TOKEN_PLACEHOLDER') {
    console.log('LiveTemplate: Auto-detected page token');
    
    // Create and connect client automatically
    const client = new LiveTemplateClient({
      presetToken: pageToken,
      onOpen: () => console.log('LiveTemplate: Connected'),
      onClose: () => console.log('LiveTemplate: Disconnected'),
      onError: (error) => console.error('LiveTemplate error:', error)
    });
    
    // Auto-connect using session cookies (no token needed)
    const port = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');
    const wsUrl = `ws://${window.location.hostname}:${port}/ws?has_cache=${client.hasCachedStatics() ? 'true' : 'false'}`;
    console.log('LiveTemplate: Connecting to:', wsUrl);
    console.log('LiveTemplate: Detected port:', port, 'from window.location.port:', window.location.port);
    client.connect(wsUrl);
    
    // Make client globally available for actions
    window.liveTemplateClient = client;
    
    // Set up automatic action handling for elements with data-lvt-action
    document.addEventListener('click', (e) => {
      const action = e.target.getAttribute('data-lvt-action');
      if (action && client.isConnected()) {
        client.sendAction(action);
      }
    });
    
    return client;
  }
  
  return null;
}

// Export for both CommonJS and ES modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = LiveTemplateClient;
} else if (typeof window !== 'undefined') {
  window.LiveTemplateClient = LiveTemplateClient;
  
  // Auto-initialize when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', autoInitializeLiveTemplate);
  } else {
    autoInitializeLiveTemplate();
  }
}