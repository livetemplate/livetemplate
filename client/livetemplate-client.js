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
  }

  connect(customUrl = null) {
    const url = customUrl || `${this.protocol}://${this.host}:${this.port}${this.endpoint}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      // If we have a preset token, log it and notify ready
      if (this.pageToken) {
        console.log("Using preset token:", this.pageToken);
      }
      this.onOpen();
    };

    this.ws.onmessage = (event) => {
      console.log("Received:", event.data);
      const message = JSON.parse(event.data);

      if (message.type === "page_token") {
        this.pageToken = message.token;
        console.log("Page token received:", this.pageToken);
      } else if (message.type === "fragments") {
        this.updateFragments(message.fragments);
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
    if (this.ws && this.pageToken) {
      const message = {
        type: "action",
        action: action,
        token: this.pageToken,
        data: data,
      };
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn("WebSocket not connected or page token not available");
    }
  }

  updateFragments(fragments) {
    fragments.forEach((fragment) => {
      console.log("Updating fragment:", fragment);

      // Cache statics if provided
      if (fragment.data && fragment.data.s) {
        this.staticCache[fragment.id] = fragment.data.s;
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

// Export for both CommonJS and ES modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = LiveTemplateClient;
} else if (typeof window !== 'undefined') {
  window.LiveTemplateClient = LiveTemplateClient;
}