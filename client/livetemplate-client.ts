/**
 * LiveTemplate TypeScript Client
 * 
 * Reconstructs HTML from tree-based updates using cached static structure,
 * following the Phoenix LiveView optimization approach.
 */

import morphdom from 'morphdom';

export interface TreeNode {
  [key: string]: any;
  s?: string[];  // Static HTML segments (sent once, cached client-side)
}

export interface UpdateResult {
  html: string;
  changed: boolean;
  dom?: Element;
}

export interface LiveTemplateClientOptions {
  wsUrl?: string;  // WebSocket URL (defaults to current host)
  liveUrl?: string; // HTTP endpoint URL (defaults to /live)
  autoReconnect?: boolean;  // Auto-reconnect on disconnect (default: true)
  reconnectDelay?: number;  // Reconnect delay in ms (default: 1000)
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Event) => void;
}

export class LiveTemplateClient {
  private treeState: TreeNode = {};
  private rangeState: { [fieldKey: string]: any[] } = {}; // Track range items by field key
  private lvtId: string | null = null;

  // Transport properties
  private ws: WebSocket | null = null;
  private wrapperElement: Element | null = null;
  private options: LiveTemplateClientOptions;
  private reconnectTimer: number | null = null;
  private useHTTP: boolean = false; // True when WebSocket is unavailable
  private sessionCookie: string | null = null; // For HTTP mode session tracking

  constructor(options: LiveTemplateClientOptions = {}) {
    this.options = {
      autoReconnect: false, // Disable autoReconnect by default to avoid connection loops
      reconnectDelay: 1000,
      liveUrl: '/live',
      ...options
    };
  }

  /**
   * Auto-initialize when DOM is ready
   * Called automatically when script loads
   */
  static autoInit(): void {
    const init = () => {
      const wrapper = document.querySelector('[data-lvt-id]');
      if (wrapper) {
        const client = new LiveTemplateClient();
        client.wrapperElement = wrapper;

        // Try WebSocket first (most efficient)
        client.connectWebSocket();

        // Set up event delegation
        client.setupEventDelegation();

        // Expose as global for programmatic access
        (window as any).liveTemplateClient = client;
      }
    };

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

  /**
   * Check if WebSocket is available on the server
   * Makes a HEAD request to probe the endpoint without fetching data
   */
  private async checkWebSocketAvailability(): Promise<boolean> {
    try {
      const liveUrl = this.options.liveUrl || '/live';

      // Try HEAD request first (most efficient)
      const response = await fetch(liveUrl, {
        method: 'HEAD'
      });

      // Check the X-LiveTemplate-WebSocket header
      const wsHeader = response.headers.get('X-LiveTemplate-WebSocket');

      if (wsHeader) {
        return wsHeader === 'enabled';
      }

      // If no header, assume WebSocket is enabled (backward compatibility)
      return true;
    } catch (error) {
      console.error('Failed to check WebSocket availability:', error);
      // On error, assume WebSocket is available and try to connect
      return true;
    }
  }

  /**
   * Fetch initial state via HTTP GET
   */
  private async fetchInitialState(): Promise<void> {
    try {
      const liveUrl = this.options.liveUrl || '/live';
      const response = await fetch(liveUrl, {
        method: 'GET',
        credentials: 'include', // Include cookies for session
        headers: {
          'Accept': 'application/json'
        }
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch initial state: ${response.status}`);
      }

      const update = await response.json();
      if (this.wrapperElement) {
        this.updateDOM(this.wrapperElement, update);
      }
    } catch (error) {
      console.error('Failed to fetch initial state:', error);
    }
  }

  /**
   * Connect via WebSocket
   */
  private connectWebSocket(): void {
    // Determine WebSocket URL
    const wsUrl = this.options.wsUrl || `ws://${window.location.host}${this.options.liveUrl || '/live'}`;

    // Create WebSocket connection
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('LiveTemplate: WebSocket connected');
      if (this.options.onConnect) {
        this.options.onConnect();
      }
    };

    this.ws.onmessage = (event) => {
      try {
        const update = JSON.parse(event.data);

        if (this.wrapperElement) {
          this.updateDOM(this.wrapperElement, update);
        }
      } catch (error) {
        console.error('LiveTemplate error:', error);
      }
    };

    this.ws.onclose = () => {
      console.log('LiveTemplate: WebSocket disconnected');
      if (this.options.onDisconnect) {
        this.options.onDisconnect();
      }

      if (this.options.autoReconnect) {
        this.reconnectTimer = window.setTimeout(() => {
          console.log('LiveTemplate: Attempting to reconnect...');
          this.connectWebSocket();
        }, this.options.reconnectDelay);
      }
    };

    this.ws.onerror = (error) => {
      console.error('LiveTemplate WebSocket error:', error);
      if (this.options.onError) {
        this.options.onError(error);
      }
    };
  }

  /**
   * Connect to WebSocket and start receiving updates
   * @param wrapperSelector - CSS selector for the LiveTemplate wrapper (defaults to '[data-lvt-id]')
   */
  async connect(wrapperSelector: string = '[data-lvt-id]'): Promise<void> {
    // Find the wrapper element
    this.wrapperElement = document.querySelector(wrapperSelector);
    if (!this.wrapperElement) {
      throw new Error(`LiveTemplate wrapper not found with selector: ${wrapperSelector}`);
    }

    // Clear any existing reconnect timer
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    // Check if WebSocket is available on the server
    // Note: checkWebSocketAvailability() will also fetch initial state if WS is disabled
    const wsAvailable = await this.checkWebSocketAvailability();

    if (wsAvailable) {
      // Use WebSocket mode
      this.connectWebSocket();
    } else {
      // Fall back to HTTP mode
      // Initial state already fetched by checkWebSocketAvailability()
      console.log('LiveTemplate: WebSocket not available, using HTTP mode');
      this.useHTTP = true;
      if (this.options.onConnect) {
        this.options.onConnect();
      }
    }

    // Set up event delegation for lvt-* attributes
    this.setupEventDelegation();
  }

  /**
   * Set up event delegation for elements with lvt-* attributes
   * Uses event delegation to handle dynamically updated elements
   * Supports: lvt-click, lvt-submit, lvt-change, lvt-input, lvt-keydown, lvt-keyup
   */
  private setupEventDelegation(): void {
    if (!this.wrapperElement) return;

    const eventTypes = ['click', 'submit', 'change', 'input', 'keydown', 'keyup'];
    const wrapperId = this.wrapperElement.getAttribute('data-lvt-id');

    eventTypes.forEach((eventType) => {
      // Remove existing delegated listener if any
      const listenerKey = `__lvt_delegated_${eventType}_${wrapperId}`;
      const existingListener = (document as any)[listenerKey];
      if (existingListener) {
        document.removeEventListener(eventType, existingListener, false);
      }

      // Create delegated listener on document
      const listener = (e: Event) => {
        const target = e.target as Element;
        if (!target) return;


        // Check if target is within our LiveTemplate wrapper
        let element: Element | null = target;
        let inWrapper = false;

        while (element) {
          if (element === this.wrapperElement) {
            inWrapper = true;
            break;
          }
          element = element.parentElement;
        }


        if (!inWrapper) return;

        // Check if target or any parent has the lvt-* attribute
        const attrName = `lvt-${eventType}`;
        element = target;

        while (element && element !== this.wrapperElement!.parentElement) {
          const action = element.getAttribute(attrName);
          if (action) {
            // Prevent default for submit events
            if (eventType === 'submit') {
              e.preventDefault();
            }

            // Build message with action and data map
            const message: any = { action, data: {} };

            // 1. Form data (for submit events)
            if (eventType === 'submit' && element instanceof HTMLFormElement) {
              const formData = new FormData(element);
              formData.forEach((value, key) => {
                message.data[key] = this.parseValue(value as string);
              });
            }

            // 2. Input value (for change/input events)
            if ((eventType === 'change' || eventType === 'input') && element instanceof HTMLInputElement) {
              message.data.value = this.parseValue(element.value);
            }

            // 3. lvt-data-* attributes (custom data)
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith('lvt-data-')) {
                const key = attr.name.replace('lvt-data-', '');
                message.data[key] = this.parseValue(attr.value);
              }
            });

            // 4. lvt-value-* attributes (explicit multiple values)
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith('lvt-value-')) {
                const key = attr.name.replace('lvt-value-', '');
                message.data[key] = this.parseValue(attr.value);
              }
            });

            // Send message to server
            this.send(message);
            return;
          }
          element = element.parentElement;
        }
      };

      // Store and add listener on document with bubble phase
      (document as any)[listenerKey] = listener;
      document.addEventListener(eventType, listener, false);
    });
  }

  /**
   * Disconnect from WebSocket
   */
  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /**
   * Send a message to the server via WebSocket or HTTP
   * @param message - Message to send (will be JSON stringified)
   */
  send(message: any): void {
    if (this.useHTTP) {
      // HTTP mode: send via POST and handle response
      this.sendHTTP(message);
    } else if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      // WebSocket mode
      this.ws.send(JSON.stringify(message));
    } else if (this.ws) {
      // WebSocket is connecting or closing, fall back to HTTP temporarily
      console.log('LiveTemplate: WebSocket not ready, using HTTP fallback');
      this.sendHTTP(message);
    } else {
      console.error('LiveTemplate: No transport available');
    }
  }

  /**
   * Send action via HTTP POST
   */
  private async sendHTTP(message: any): Promise<void> {
    try {
      const liveUrl = this.options.liveUrl || '/live';
      const response = await fetch(liveUrl, {
        method: 'POST',
        credentials: 'include', // Include cookies for session
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json'
        },
        body: JSON.stringify(message)
      });

      if (!response.ok) {
        throw new Error(`HTTP request failed: ${response.status}`);
      }

      // Handle the update response
      const update = await response.json();
      if (this.wrapperElement) {
        this.updateDOM(this.wrapperElement, update);
      }
    } catch (error) {
      console.error('Failed to send HTTP request:', error);
    }
  }

  /**
   * Parse a string value into appropriate type (number, boolean, or string)
   * @param value - String value to parse
   * @returns Parsed value with correct type
   */
  private parseValue(value: string): any {
    // Try to parse as number
    const num = parseFloat(value);
    if (!isNaN(num) && value.trim() === num.toString()) {
      return num;
    }

    // Try to parse as boolean
    if (value === 'true') return true;
    if (value === 'false') return false;

    // Return as string
    return value;
  }

  /**
   * Apply an update to the current state and reconstruct HTML
   * @param update - Tree update object from LiveTemplate server
   * @returns Reconstructed HTML and whether anything changed
   */
  applyUpdate(update: TreeNode): UpdateResult {
    let changed = false;

    // Merge the update into our tree state
    for (const [key, value] of Object.entries(update)) {
      if (JSON.stringify(this.treeState[key]) !== JSON.stringify(value)) {
        this.treeState[key] = value;
        changed = true;
      }
    }

    // Reconstruct HTML from the complete tree state
    const html = this.reconstructFromTree(this.treeState);
    
    return { html, changed };
  }

  /**
   * Reconstruct HTML from a tree structure
   * This is the core algorithm that matches the Go server implementation
   * Dynamic values are interleaved between static segments: static[0] + dynamic[0] + static[1] + ...
   * Invariant: len(statics) == len(dynamics) + 1
   */
  private reconstructFromTree(node: TreeNode): string {
    // If node has static segments, use them as the template
    if (node.s && Array.isArray(node.s)) {
      let html = '';
      
      // Interleave static segments with dynamic values
      // Pattern: static[0] + dynamic[0] + static[1] + dynamic[1] + ... + static[n]
      for (let i = 0; i < node.s.length; i++) {
        const staticSegment = node.s[i];
        
        // Add static segment
        html += staticSegment;
        
        // After adding the static segment, add the corresponding dynamic value if it exists
        // But only if we're not at the last static segment
        if (i < node.s.length - 1) {
          const dynamicKey = i.toString();
          if (node[dynamicKey] !== undefined) {
            html += this.renderValue(node[dynamicKey], dynamicKey);
          }
        }
      }
      
      // Remove the <root> wrapper that was added for parsing
      html = html.replace(/<root>/g, '').replace(/<\/root>/g, '');
      
      return html;
    }
    
    // If no static segments, just render the values
    return this.renderValue(node);
  }

  /**
   * Render a dynamic value (could be string, nested tree, or array)
   */
  private renderValue(value: any, fieldKey?: string): string {
    if (value === null || value === undefined) {
      return '';
    }

    // Skip template control expressions
    if (typeof value === 'string' && value.startsWith('{{') && value.endsWith('}}')) {
      return ''; // Don't render template expressions
    }

    // Handle range structures with 'd' (dynamics) and 's' (statics) arrays
    if (typeof value === 'object' && !Array.isArray(value)) {
      // Check if this is a range structure with 'd' and 's'
      if (value.d && Array.isArray(value.d) && value.s && Array.isArray(value.s)) {
        // Store the range items in our state for differential operations
        if (fieldKey) {
          this.rangeState[fieldKey] = value.d;
        }
        return this.renderRangeStructure(value);
      }
      // Regular nested tree structure
      if (value.s) {
        return this.reconstructFromTree(value);
      }
    }

    // Handle differential operations array
    if (Array.isArray(value)) {
      // Check if this is a differential operations array
      if (value.length > 0 && Array.isArray(value[0]) && typeof value[0][0] === 'string') {
        return this.applyDifferentialOperations(value, fieldKey);
      }

      // Regular array (from range iteration)
      return value.map(item => {
        // Each item should be a tree node with its own static/dynamic structure
        if (typeof item === 'object' && item.s) {
          return this.reconstructFromTree(item);
        }
        return this.renderValue(item);
      }).join('');
    }

    // Simple string/number value
    return String(value);
  }

  /**
   * Render a range structure with 'd' (dynamics) and 's' (statics) arrays
   */
  private renderRangeStructure(rangeNode: any): string {
    const { d: dynamics, s: statics } = rangeNode;

    if (!dynamics || !Array.isArray(dynamics)) {
      return '';
    }

    // For empty ranges
    if (dynamics.length === 0) {
      // Check if there's alternative content for empty ranges
      if (rangeNode['else']) {
        return this.renderValue(rangeNode['else']);
      }
      return '';
    }

    // If we have statics, use them as the template for each item
    if (statics && Array.isArray(statics)) {
      return dynamics.map((item: any) => {
        let html = '';

        for (let i = 0; i < statics.length; i++) {
          html += statics[i];

          // Add dynamic value if not at the last static segment
          if (i < statics.length - 1) {
            const fieldKey = i.toString();
            if (item[fieldKey] !== undefined) {
              html += this.renderValue(item[fieldKey]);
            }
          }
        }

        return html;
      }).join('');
    }

    // Fallback: render items as-is if no statics template
    return dynamics.map(item => this.renderValue(item)).join('');
  }

  /**
   * Apply differential operations to existing range items
   * Operations: ["r", key] for remove, ["u", key, changes] for update, ["a", key, data] for add
   */
  private applyDifferentialOperations(operations: any[], fieldKey?: string): string {
    if (!fieldKey || !this.rangeState[fieldKey]) {
      // If we don't have previous range state, we can't apply differential operations
      // This happens on the first load - just return empty for now
      return '';
    }

    const currentItems = [...this.rangeState[fieldKey]]; // Clone current items

    // Apply each operation
    for (const operation of operations) {
      if (!Array.isArray(operation) || operation.length < 2) {
        continue;
      }

      const [opType, key, data] = operation;
      const itemIndex = currentItems.findIndex((item: any) => item['0'] === key); // Field '0' contains the key

      switch (opType) {
        case 'r': // Remove
          if (itemIndex >= 0) {
            currentItems.splice(itemIndex, 1);
          }
          break;

        case 'u': // Update
          if (itemIndex >= 0 && data) {
            // Merge the changes into the existing item
            currentItems[itemIndex] = { ...currentItems[itemIndex], ...data };
          }
          break;

        case 'a': // Add (append) - support both single item and array
          if (data) {
            if (Array.isArray(data)) {
              currentItems.push(...data);
            } else {
              currentItems.push(data);
            }
          }
          break;

        case 'i': // Insert with position - support both single item and array
          const [, targetKey, position, insertData] = operation;
          if (insertData) {
            const itemsToInsert = Array.isArray(insertData) ? insertData : [insertData];

            if (targetKey === null) {
              if (position === "start") {
                currentItems.unshift(...itemsToInsert);
              } else { // "end"
                currentItems.push(...itemsToInsert);
              }
            } else {
              const targetIndex = currentItems.findIndex(item => item['0'] === targetKey);
              if (targetIndex >= 0) {
                const insertIndex = position === "before" ? targetIndex : targetIndex + 1;
                currentItems.splice(insertIndex, 0, ...itemsToInsert);
              }
            }
          }
          break;

        case 'o': // Order (reordering)
          // key contains the new order array
          const newOrder = key as string[];
          const reorderedItems: any[] = [];

          // Build a map of current items by key for efficient lookup
          const itemsByKey = new Map();
          for (const item of currentItems) {
            if (item['0']) {
              itemsByKey.set(item['0'], item);
            }
          }

          // Reorder items according to the new key order
          for (const orderedKey of newOrder) {
            const item = itemsByKey.get(orderedKey);
            if (item) {
              reorderedItems.push(item);
            }
          }

          // Replace currentItems with reordered items
          currentItems.length = 0;
          currentItems.push(...reorderedItems);
          break;
      }
    }

    // Update our range state
    this.rangeState[fieldKey] = currentItems;

    // Render using the current range structure template
    const rangeStructure = this.getCurrentRangeStructure(fieldKey);
    if (rangeStructure && rangeStructure.s) {
      return this.renderItemsWithStatics(currentItems, rangeStructure.s);
    }

    return currentItems.map(item => this.renderValue(item)).join('');
  }

  /**
   * Get the current range structure for a field
   */
  private getCurrentRangeStructure(fieldKey: string): any {
    const fieldValue = this.treeState[fieldKey];
    if (fieldValue && typeof fieldValue === 'object' && fieldValue.s) {
      return fieldValue;
    }
    return null;
  }

  /**
   * Render items using static template
   */
  private renderItemsWithStatics(items: any[], statics: string[]): string {
    return items.map((item: any) => {
      let html = '';

      for (let i = 0; i < statics.length; i++) {
        html += statics[i];

        // Add dynamic value if not at the last static segment
        if (i < statics.length - 1) {
          const fieldKey = i.toString();
          if (item[fieldKey] !== undefined) {
            html += this.renderValue(item[fieldKey]);
          }
        }
      }

      return html;
    }).join('');
  }

  /**
   * Apply updates to existing HTML using morphdom for efficient DOM updates
   * @param existingHTML - Current full HTML document
   * @param update - Tree update object from LiveTemplate server
   * @returns Updated HTML content
   */
  applyUpdateToHTML(existingHTML: string, update: TreeNode): string {
    // Apply the update to our internal state
    const result = this.applyUpdate(update);
    
    // Extract lvt-id from existing HTML if we don't have it
    if (!this.lvtId) {
      const match = existingHTML.match(/data-lvt-id="([^"]+)"/);
      if (match) {
        this.lvtId = match[1];
      }
    }
    
    // The new tree includes complete HTML structure, so we can reconstruct properly
    const innerContent = result.html;
    
    // Find where to insert the reconstructed content
    const bodyMatch = existingHTML.match(/<body>([\s\S]*?)<\/body>/);
    if (!bodyMatch) {
      return existingHTML;
    }
    
    // Replace the body content with our reconstructed HTML
    // We need to preserve the wrapper div with data-lvt-id
    const wrapperStart = `<div data-lvt-id="${this.lvtId || 'lvt-unknown'}">`;
    const wrapperEnd = '</div>';
    const newBodyContent = wrapperStart + innerContent + wrapperEnd;
    
    return existingHTML.replace(/<body>[\s\S]*?<\/body>/, `<body>${newBodyContent}</body>`);
  }

  /**
   * Update a live DOM element with new tree data
   * @param element - DOM element containing the LiveTemplate content (the wrapper div)
   * @param update - Tree update object from LiveTemplate server
   */
  updateDOM(element: Element, update: TreeNode): void {
    // Apply update to internal state and get reconstructed HTML
    const result = this.applyUpdate(update);

    if (!result.changed && !update.s) {
      // No changes detected and no statics in update, skip morphdom
      return;
    }

    // Create temporary container with the reconstructed content
    // Note: The reconstructed HTML is the content WITHOUT the wrapper div
    // The wrapper is preserved on the client side
    const tempContainer = document.createElement('div');
    tempContainer.innerHTML = result.html;

    // Use morphdom to efficiently update only the children of the wrapper element
    morphdom(element, tempContainer, {
      childrenOnly: true,  // Only update children, preserve the wrapper element itself
      onBeforeElUpdated: (fromEl, toEl) => {
        // Only update if content actually changed
        if (fromEl.isEqualNode(toEl)) {
          return false;
        }
        return true;
      }
    });
  }

  /**
   * Reset client state (useful for testing)
   */
  reset(): void {
    this.treeState = {};
    this.rangeState = {};
    this.lvtId = null;
  }

  /**
   * Get current tree state (for debugging)
   */
  getTreeState(): TreeNode {
    return { ...this.treeState };
  }

  /**
   * Get the static structure if available
   */
  getStaticStructure(): string[] | null {
    return this.treeState.s || null;
  }
}

/**
 * Utility function to load and apply updates from JSON files
 */
export async function loadAndApplyUpdate(
  client: LiveTemplateClient, 
  updatePath: string
): Promise<UpdateResult> {
  try {
    // In Node.js environment
    if (typeof require !== 'undefined') {
      const fs = require('fs');
      const updateData = JSON.parse(fs.readFileSync(updatePath, 'utf8'));
      return client.applyUpdate(updateData);
    }
    
    // In browser environment
    const response = await fetch(updatePath);
    const updateData = await response.json();
    return client.applyUpdate(updateData);
  } catch (error) {
    throw new Error(`Failed to load update from ${updatePath}: ${error}`);
  }
}

/**
 * Compare two HTML strings, ignoring whitespace differences
 */
export function compareHTML(expected: string, actual: string): {
  match: boolean;
  differences: string[];
} {
  const differences: string[] = [];
  
  // Normalize whitespace for comparison
  const normalizeHTML = (html: string) => {
    return html
      .replace(/\s+/g, ' ')           // Collapse multiple spaces
      .replace(/>\s+</g, '><')        // Remove spaces between tags
      .trim();
  };
  
  const normalizedExpected = normalizeHTML(expected);
  const normalizedActual = normalizeHTML(actual);
  
  if (normalizedExpected === normalizedActual) {
    return { match: true, differences: [] };
  }
  
  // Find specific differences
  const expectedLines = normalizedExpected.split('\n');
  const actualLines = normalizedActual.split('\n');
  
  const maxLines = Math.max(expectedLines.length, actualLines.length);
  for (let i = 0; i < maxLines; i++) {
    const expectedLine = expectedLines[i] || '';
    const actualLine = actualLines[i] || '';
    
    if (expectedLine !== actualLine) {
      differences.push(`Line ${i + 1}:`);
      differences.push(`  Expected: ${expectedLine}`);
      differences.push(`  Actual:   ${actualLine}`);
    }
  }
  
  return { match: false, differences };
}

// Auto-initialize when script loads (for browser use)
if (typeof window !== 'undefined') {
  LiveTemplateClient.autoInit();
}
