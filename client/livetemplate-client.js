/**
 * LiveTemplate Client Library
 * 
 * A generic WebSocket-based client for LiveTemplate server-side rendering.
 * This client handles fragment updates from the server and applies them to the DOM.
 */
class LiveTemplateClient {
  constructor(options = {}) {
    this.wsUrl = options.wsUrl || this.buildWebSocketUrl();
    this.pageToken = null;
    this.ws = null;
    this.staticCache = {};
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
    this.reconnectDelay = options.reconnectDelay || 1000;
    
    // Callbacks
    this.onOpen = options.onOpen || (() => console.log("WebSocket connected"));
    this.onClose = options.onClose || (() => console.log("WebSocket disconnected"));
    this.onError = options.onError || ((error) => console.error("WebSocket error:", error));
    this.onMessage = options.onMessage || null;
    
    // Fragment processing callback - allows apps to hook into fragment processing
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
    
    console.log("Connecting to WebSocket:", url);
    this.ws = new WebSocket(url);
    
    this.setupEventHandlers();
  }

  setupEventHandlers() {
    this.ws.onopen = () => {
      console.log("WebSocket connection established");
      this.reconnectAttempts = 0;
      this.onOpen();
    };

    this.ws.onclose = (event) => {
      console.log("WebSocket connection closed", event);
      this.onClose();
      this.attemptReconnect();
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      this.onError(error);
    };

    this.ws.onmessage = (event) => {
      try {
        const fragments = JSON.parse(event.data);
        console.log("Received fragments:", fragments);
        
        if (Array.isArray(fragments)) {
          this.applyFragments(fragments);
        }
        
        if (this.onMessage) {
          this.onMessage(fragments);
        }
      } catch (error) {
        console.error("Error processing message:", error);
      }
    };
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("Max reconnection attempts reached");
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    
    console.log(`Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts}) in ${delay}ms...`);
    
    setTimeout(() => {
      if (this.pageToken) {
        this.connect(this.pageToken);
      }
    }, delay);
  }

  applyFragments(fragments) {
    console.log(`Applying ${fragments.length} fragments`);
    
    // Apply fragments sequentially to avoid race conditions
    fragments.forEach((fragment, index) => {
      try {
        console.log(`Applying fragment ${index + 1}/${fragments.length}: ${fragment.id}`);
        this.applyFragment(fragment);
        
        // Call the optional callback for custom processing
        if (this.onFragmentUpdate) {
          this.onFragmentUpdate(fragment);
        }
      } catch (error) {
        console.error(`Error applying fragment ${fragment.id}:`, error);
      }
    });
    console.log(`✅ Completed applying all ${fragments.length} fragments`);
  }

  applyFragment(fragment) {
    console.log(`Applying fragment: ${fragment.id}`);
    
    // Find the element with the matching fragment ID
    // Try both lvt-id and data-lvt-fragment for compatibility
    let element = document.querySelector(`[lvt-id="${fragment.id}"]`);
    if (!element) {
      element = document.querySelector(`[data-lvt-fragment="${fragment.id}"]`);
    }
    
    if (!element) {
      console.warn(`Element not found for fragment: ${fragment.id}`);
      return;
    }
    
    // Handle different fragment strategies
    if (fragment.strategy === "tree_based" || !fragment.strategy) {
      this.applyTreeBasedFragment(element, fragment);
    } else {
      console.warn(`Unknown fragment strategy: ${fragment.strategy}`);
    }
  }

  applyTreeBasedFragment(element, fragment) {
    if (!fragment.data || typeof fragment.data !== 'object') {
      console.warn(`Invalid fragment data for ${fragment.id}`);
      return;
    }
    
    // Check if this is a tree structure with static parts
    // Handle both lowercase 's' and uppercase 'S' for backward compatibility
    const statics = fragment.data.s || fragment.data.S;
    if (statics && Array.isArray(statics)) {
      // Convert Dynamics object to numbered keys format if needed
      const dynamics = fragment.data.Dynamics || fragment.data;
      const content = this.reconstructContent(statics, dynamics);
      
      console.log(`🔧 Fragment ${fragment.id}:`);
      console.log(`   Target element:`, element.tagName, element.className, element.id);
      console.log(`   Current innerHTML length:`, element.innerHTML.length);
      console.log(`   New content length:`, content.length);
      console.log(`   New content preview:`, content.substring(0, 100) + '...');
      
      // Apply the content to the element with explicit replacement
      const oldHTML = element.innerHTML;
      
      // Check if content is a complete element that matches the target element
      // Handle both self-closing elements (like input) and regular elements (like div)
      const tagName = element.tagName.toLowerCase();
      let elementMatch = null;
      
      if (tagName === 'input') {
        // Self-closing input element
        elementMatch = content.match(new RegExp(`^<input([^>]*)\\s*/?\\s*>$`, 'is'));
        if (elementMatch) {
          elementMatch[2] = ''; // Input elements don't have inner content
        }
      } else {
        // Regular elements with opening and closing tags
        elementMatch = content.match(new RegExp(`^<${tagName}([^>]*)>(.*)</${tagName}>$`, 'is'));
      }
      
      if (elementMatch) {
        // Content is a complete element - extract attributes and inner content separately
        const attributesStr = elementMatch[1];
        const innerContent = elementMatch[2];
        
        console.log(`🔄 Fragment ${fragment.id}: Detected complete ${element.tagName} element`);
        console.log(`   Target element:`, element);
        console.log(`   Attributes: ${attributesStr.trim()}`);
        console.log(`   Inner content: ${innerContent.substring(0, 50)}...`);
        console.log(`   Full reconstructed content:`, content);
        
        // Update attributes
        if (attributesStr.trim()) {
          this.updateElementAttributes(element, attributesStr);
        }
        
        // Update inner content
        element.innerHTML = '';  // Clear first
        
        // Check if innerContent is plain text or contains HTML
        const trimmedContent = innerContent.trim();
        if (trimmedContent && !trimmedContent.includes('<')) {
          // Plain text content - use textContent to avoid browser translation wrapping
          element.textContent = trimmedContent;
          console.log(`   Used textContent for plain text: "${trimmedContent}"`);
        } else {
          // HTML content - use innerHTML
          element.innerHTML = innerContent;
          console.log(`   Used innerHTML for HTML content`);
        }
        
        console.log(`✅ Fragment ${fragment.id}: Updated both attributes and content`);
      } else if (element.tagName === 'INPUT' && element.type !== 'checkbox' && element.type !== 'radio') {
        // Special handling for input elements
        const valueMatch = content.match(/value="([^"]*)"/);
        if (valueMatch) {
          element.value = valueMatch[1];
          console.log(`✅ Fragment ${fragment.id}: Updated input value to "${element.value}"`);
        } else {
          console.log(`✅ Fragment ${fragment.id}: No value attribute found, clearing input`);
          element.value = '';
        }
      } else {
        // Standard content-only update
        element.innerHTML = '';  // Clear first
        element.innerHTML = content;  // Then set new content
        
        console.log(`✅ Fragment ${fragment.id}: ${oldHTML.length} → ${element.innerHTML.length} chars`);
      }
    } else {
      console.warn(`Fragment ${fragment.id} does not have expected tree structure`);
    }
  }

  updateElementAttributes(element, attributesStr) {
    // Parse attributes from string like ' style="color: red" class="highlight"'
    const attributePattern = /\s+([a-zA-Z-]+)=["']([^"']*)["']/g;
    let match;
    
    console.log(`🔧 Updating attributes on ${element.tagName}: ${attributesStr}`);
    
    // Special check for todo input
    if (element.tagName === 'INPUT' && element.getAttribute('name') === 'todo-input') {
      console.log(`🎯 UPDATING TODO INPUT ATTRIBUTES!`);
      console.log(`   Before update - element.value: "${element.value}"`);
      console.log(`   Before update - value attr: "${element.getAttribute('value')}"`);
    }
    
    while ((match = attributePattern.exec(attributesStr)) !== null) {
      const [, attrName, attrValue] = match;
      
      // Handle special attribute mappings
      if (attrName === 'class') {
        element.className = attrValue;
      } else if (attrName === 'style') {
        element.style.cssText = attrValue;
      } else if (attrName === 'value' && element.tagName === 'INPUT') {
        // Special handling for input value - set both the DOM property AND the attribute
        // The DOM property takes precedence for what user sees, so set it AFTER the attribute
        element.setAttribute('value', attrValue);  // Updates HTML attribute first
        element.value = attrValue;  // Force DOM property update (what user sees)
        
        // Force a change event to ensure any listeners are notified
        element.dispatchEvent(new Event('change', { bubbles: true }));
      } else {
        element.setAttribute(attrName, attrValue);
      }
      
      console.log(`   Set ${attrName}="${attrValue}"`);
    }
  }

  reconstructContent(statics, dynamics) {
    // Reconstruct content from static parts and dynamic values
    let result = "";
    
    for (let i = 0; i < statics.length; i++) {
      result += statics[i];
      
      // Add dynamic value if it exists
      // Check both string and number keys for compatibility
      const dynamicValue = dynamics[i.toString()] || dynamics[i];
      
      if (i < statics.length - 1 && dynamicValue !== undefined) {
        if (Array.isArray(dynamicValue)) {
          // Handle arrays (e.g., range constructs with multiple items)
          const arrayContent = dynamicValue.map(item => {
            if (typeof item === 'object' && item !== null && (item.s || item.S)) {
              // This is a tree structure for a single array item
              const itemStatics = item.s || item.S;
              const itemDynamics = item.Dynamics || item;
              return this.reconstructContent(itemStatics, itemDynamics);
            } else {
              // Simple value
              return item;
            }
          }).join('');
          result += arrayContent;
        } else if (typeof dynamicValue === 'object' && dynamicValue !== null) {
          // Nested tree data - recursively reconstruct
          const nestedStatics = dynamicValue.s || dynamicValue.S || [""];
          const nestedDynamics = dynamicValue.Dynamics || dynamicValue;
          result += this.reconstructContent(nestedStatics, nestedDynamics);
        } else {
          // Simple value
          result += dynamicValue;
        }
      }
    }
    
    return result;
  }

  sendAction(action, params = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error("WebSocket is not connected");
      return;
    }
    
    const message = {
      action: action,
      data: params
    };
    
    console.log("Sending action:", message);
    this.ws.send(JSON.stringify(message));
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
}

// Auto-initialize when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
  // Extract the LiveTemplate token from the page
  let token = null;
  
  // Try to get token from embedded config
  if (typeof config !== 'undefined' && config.LIVETEMPLATE_TOKEN) {
    token = config.LIVETEMPLATE_TOKEN;
  }
  
  // Fallback: try to extract from script tag
  if (!token) {
    const scripts = document.getElementsByTagName('script');
    for (let script of scripts) {
      const match = script.textContent.match(/["']?LIVETEMPLATE_TOKEN["']?\s*:\s*["']([^"']+)["']/);
      if (match) {
        token = match[1];
        break;
      }
    }
  }
  
  if (!token || token === 'PAGE_TOKEN_PLACEHOLDER') {
    console.log('LiveTemplate: No valid token found, skipping auto-initialization');
    return;
  }
  
  // Initialize the client
  window.liveTemplate = new LiveTemplateClient({
    onOpen: () => console.log('LiveTemplate: Connected'),
    onClose: () => console.log('LiveTemplate: Disconnected'),
    onError: (error) => console.error('LiveTemplate error:', error)
  });
  
  // Connect with the token
  window.liveTemplate.connect(token);
  
  // Set up action handlers for elements with data-lvt-action
  document.addEventListener('click', function(e) {
    const target = e.target.closest('[data-lvt-action]');
    if (!target) return;
    
    e.preventDefault();
    
    const action = target.getAttribute('data-lvt-action');
    if (!action) return;
    
    // Collect form data from the surrounding form or container
    const actionData = {};
    
    // Check for explicit params
    const paramsAttr = target.getAttribute('data-lvt-params');
    if (paramsAttr) {
      try {
        const explicitParams = JSON.parse(paramsAttr);
        Object.assign(actionData, explicitParams);
        console.log('LiveTemplate: Parsed action data:', actionData);
      } catch (err) {
        console.error('Invalid data-lvt-params JSON:', err);
      }
    }
    
    // Collect input values from the form or container
    // Strategy 1: Look for form element
    let container = target.closest('form');
    
    // Strategy 2: Try parent elements walking up the DOM tree
    if (!container) {
      let current = target.parentElement;
      while (current && current !== document.body) {
        const inputs = current.querySelectorAll('input, select, textarea');
        if (inputs.length > 0) {
          container = current;
          break;
        }
        current = current.parentElement;
      }
    }
    
    // Strategy 3: Try generic semantic containers
    if (!container) {
      container = target.closest('form, .form, [role="form"]');
    }
    
    // Strategy 4: Search entire document for inputs if container not found
    if (!container) {
      container = document;
    }
    
    // Collect form data from the container
    const inputs = container.querySelectorAll('input, select, textarea');
    inputs.forEach(input => {
      if (input.name) {
        if (input.type === 'checkbox') {
          actionData[input.name] = input.checked;
        } else if (input.type === 'radio') {
          if (input.checked) {
            actionData[input.name] = input.value;
          }
        } else {
          actionData[input.name] = input.value;
        }
      }
    });
    
    console.log(`LiveTemplate: Sending action '${action}' with data:`, actionData);
    
    if (window.liveTemplate && window.liveTemplate.isConnected()) {
      window.liveTemplate.sendAction(action, actionData);
    } else {
      console.error('LiveTemplate: Not connected to server');
    }
  });
});