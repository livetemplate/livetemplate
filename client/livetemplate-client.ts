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

export interface ResponseMetadata {
  success: boolean;      // true if no validation errors
  errors: { [key: string]: string };  // field errors
  action?: string;       // action name
}

export interface UpdateResponse {
  tree: TreeNode;
  meta?: ResponseMetadata;
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
  private rangeState: { [fieldKey: string]: { items: any[], statics: any[] } } = {}; // Track range items and statics by field key
  private lvtId: string | null = null;

  // Transport properties
  private ws: WebSocket | null = null;
  private wrapperElement: Element | null = null;
  private options: LiveTemplateClientOptions;
  private reconnectTimer: number | null = null;
  private useHTTP: boolean = false; // True when WebSocket is unavailable
  private sessionCookie: string | null = null; // For HTTP mode session tracking

  // Form lifecycle tracking
  private activeForm: HTMLFormElement | null = null; // The form that submitted the current action
  private activeButton: HTMLButtonElement | null = null; // The button that triggered the action
  private originalButtonText: string | null = null; // Original button text for restore

  // Rate limiting: cache of debounced/throttled handlers per element+eventType
  private rateLimitedHandlers: WeakMap<Element, Map<string, Function>> = new WeakMap();

  constructor(options: LiveTemplateClientOptions = {}) {
    this.options = {
      autoReconnect: false, // Disable autoReconnect by default to avoid connection loops
      reconnectDelay: 1000,
      liveUrl: '/',
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
        client.setupWindowEventDelegation();
        client.setupClickAwayDelegation();

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
      // Dispatch connected event on wrapper element
      if (this.wrapperElement) {
        this.wrapperElement.dispatchEvent(new Event('lvt:connected'));
      }
    };

    this.ws.onmessage = (event) => {
      try {
        const response: UpdateResponse = JSON.parse(event.data);

        if (this.wrapperElement) {
          this.updateDOM(this.wrapperElement, response.tree, response.meta);
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
      // Dispatch disconnected event on wrapper element
      if (this.wrapperElement) {
        this.wrapperElement.dispatchEvent(new Event('lvt:disconnected'));
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

    // Set up window-* event delegation
    this.setupWindowEventDelegation();

    // Set up click-away delegation
    this.setupClickAwayDelegation();
  }

  /**
   * Set up event delegation for elements with lvt-* attributes
   * Uses event delegation to handle dynamically updated elements
   * Supports: lvt-click, lvt-submit, lvt-change, lvt-input, lvt-keydown, lvt-keyup,
   *           lvt-focus, lvt-blur, lvt-mouseenter, lvt-mouseleave
   */
  private setupEventDelegation(): void {
    if (!this.wrapperElement) return;

    const eventTypes = ['click', 'submit', 'change', 'input', 'keydown', 'keyup', 'focus', 'blur', 'mouseenter', 'mouseleave'];
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
          let action = element.getAttribute(attrName);
          let actionElement = element; // Element that has the action attribute

          // For change/input events, also check if element is inside a form with lvt-change
          if (!action && (eventType === 'change' || eventType === 'input')) {
            const formElement: HTMLFormElement | null = element.closest('form');
            if (formElement && formElement.hasAttribute('lvt-change')) {
              action = formElement.getAttribute('lvt-change');
              actionElement = formElement; // Use the form as the action element
            }
          }

          if (action && actionElement) {
            // Prevent default for submit events
            if (eventType === 'submit') {
              e.preventDefault();
            }

            // Check for lvt-key filtering on keyboard events
            if ((eventType === 'keydown' || eventType === 'keyup') && actionElement.hasAttribute('lvt-key')) {
              const keyFilter = actionElement.getAttribute('lvt-key');
              const keyboardEvent = e as KeyboardEvent;
              if (keyFilter && keyboardEvent.key !== keyFilter) {
                // Key doesn't match filter, skip this handler
                element = element.parentElement;
                continue;
              }
            }

            // Capture element reference for closure
            const targetElement = actionElement;

            // Define the action handler
            const handleAction = () => {
              // Build message with action and data map
              const message: any = { action, data: {} };

              // 1. Form data (for submit events or form-level change events)
              if (targetElement instanceof HTMLFormElement) {
                const formData = new FormData(targetElement);
                formData.forEach((value, key) => {
                  message.data[key] = this.parseValue(value as string);
                });
              }
              // 2. Input value (for change/input events on inputs)
              else if ((eventType === 'change' || eventType === 'input') && targetElement instanceof HTMLInputElement) {
                message.data.value = this.parseValue(targetElement.value);
              }

              // 3. lvt-data-* attributes (custom data)
              Array.from(targetElement.attributes).forEach((attr) => {
                if (attr.name.startsWith('lvt-data-')) {
                  const key = attr.name.replace('lvt-data-', '');
                  message.data[key] = this.parseValue(attr.value);
                }
              });

              // 4. lvt-value-* attributes (explicit multiple values)
              Array.from(targetElement.attributes).forEach((attr) => {
                if (attr.name.startsWith('lvt-value-')) {
                  const key = attr.name.replace('lvt-value-', '');
                  message.data[key] = this.parseValue(attr.value);
                }
              });

              // Track form lifecycle for submit events
              if (eventType === 'submit' && targetElement instanceof HTMLFormElement) {
                this.activeForm = targetElement;

                // Find submit button if it exists and has lvt-disable-with
                const submitEvent = e as SubmitEvent;
                const submitButton = submitEvent.submitter as HTMLButtonElement | null;
                if (submitButton && submitButton.hasAttribute('lvt-disable-with')) {
                  this.activeButton = submitButton;
                  this.originalButtonText = submitButton.textContent;
                  submitButton.disabled = true;
                  submitButton.textContent = submitButton.getAttribute('lvt-disable-with');
                }

                // Emit lvt:pending event
                targetElement.dispatchEvent(new CustomEvent('lvt:pending', { detail: message }));
              }

              // Send message to server
              this.send(message);
            };

            // Apply rate limiting if specified
            // Note: throttle takes precedence over debounce
            const throttleValue = actionElement.getAttribute('lvt-throttle');
            const debounceValue = actionElement.getAttribute('lvt-debounce');

            if (throttleValue || debounceValue) {
              // Get or create handler cache for this element
              if (!this.rateLimitedHandlers.has(actionElement)) {
                this.rateLimitedHandlers.set(actionElement, new Map());
              }
              const handlerCache = this.rateLimitedHandlers.get(actionElement)!;
              const cacheKey = `${eventType}:${action}`;

              // Get or create rate-limited handler
              let rateLimitedHandler = handlerCache.get(cacheKey);
              if (!rateLimitedHandler) {
                if (throttleValue) {
                  const limit = parseInt(throttleValue, 10);
                  rateLimitedHandler = throttle(handleAction, limit);
                } else if (debounceValue) {
                  const wait = parseInt(debounceValue, 10);
                  rateLimitedHandler = debounce(handleAction, wait);
                }
                if (rateLimitedHandler) {
                  handlerCache.set(cacheKey, rateLimitedHandler);
                }
              }

              // Call rate-limited handler
              if (rateLimitedHandler) {
                rateLimitedHandler();
              }
            } else {
              // No rate limiting, call directly
              handleAction();
            }

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
   * Set up window-level event delegation for lvt-window-* attributes
   * Supports: lvt-window-keydown, lvt-window-keyup, lvt-window-scroll,
   *           lvt-window-resize, lvt-window-focus, lvt-window-blur
   */
  private setupWindowEventDelegation(): void {
    if (!this.wrapperElement) return;

    const windowEvents = ['keydown', 'keyup', 'scroll', 'resize', 'focus', 'blur'];
    const wrapperId = this.wrapperElement.getAttribute('data-lvt-id');

    windowEvents.forEach((eventType) => {
      const listenerKey = `__lvt_window_${eventType}_${wrapperId}`;
      const existingListener = (window as any)[listenerKey];
      if (existingListener) {
        window.removeEventListener(eventType, existingListener);
      }

      const listener = (e: Event) => {
        if (!this.wrapperElement) return;

        // Find all elements with lvt-window-* attribute for this event
        const attrName = `lvt-window-${eventType}`;
        const elements = this.wrapperElement.querySelectorAll(`[${attrName}]`);

        elements.forEach((element) => {
          const action = element.getAttribute(attrName);
          if (!action) return;

          // Check for lvt-key filtering on keyboard events
          if ((eventType === 'keydown' || eventType === 'keyup') && element.hasAttribute('lvt-key')) {
            const keyFilter = element.getAttribute('lvt-key');
            const keyboardEvent = e as KeyboardEvent;
            if (keyFilter && keyboardEvent.key !== keyFilter) {
              return; // Key doesn't match filter
            }
          }

          // Build and send message
          const message: any = { action, data: {} };

          // Add lvt-data-* attributes
          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith('lvt-data-')) {
              const key = attr.name.replace('lvt-data-', '');
              message.data[key] = this.parseValue(attr.value);
            }
          });

          // Add lvt-value-* attributes
          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith('lvt-value-')) {
              const key = attr.name.replace('lvt-value-', '');
              message.data[key] = this.parseValue(attr.value);
            }
          });

          // Apply rate limiting if specified
          const throttleValue = element.getAttribute('lvt-throttle');
          const debounceValue = element.getAttribute('lvt-debounce');

          const handleAction = () => this.send(message);

          if (throttleValue || debounceValue) {
            if (!this.rateLimitedHandlers.has(element)) {
              this.rateLimitedHandlers.set(element, new Map());
            }
            const handlerCache = this.rateLimitedHandlers.get(element)!;
            const cacheKey = `window-${eventType}:${action}`;

            let rateLimitedHandler = handlerCache.get(cacheKey);
            if (!rateLimitedHandler) {
              if (throttleValue) {
                const limit = parseInt(throttleValue, 10);
                rateLimitedHandler = throttle(handleAction, limit);
              } else if (debounceValue) {
                const wait = parseInt(debounceValue, 10);
                rateLimitedHandler = debounce(handleAction, wait);
              }
              if (rateLimitedHandler) {
                handlerCache.set(cacheKey, rateLimitedHandler);
              }
            }

            if (rateLimitedHandler) {
              rateLimitedHandler();
            }
          } else {
            handleAction();
          }
        });
      };

      (window as any)[listenerKey] = listener;
      window.addEventListener(eventType, listener);
    });
  }

  /**
   * Set up click-away event delegation for lvt-click-away attribute
   * Triggers when clicking outside the element
   */
  private setupClickAwayDelegation(): void {
    if (!this.wrapperElement) return;

    const wrapperId = this.wrapperElement.getAttribute('data-lvt-id');
    const listenerKey = `__lvt_click_away_${wrapperId}`;
    const existingListener = (document as any)[listenerKey];
    if (existingListener) {
      document.removeEventListener('click', existingListener);
    }

    const listener = (e: Event) => {
      if (!this.wrapperElement) return;

      const target = e.target as Element;
      const elements = this.wrapperElement.querySelectorAll('[lvt-click-away]');

      elements.forEach((element) => {
        // Check if click was outside this element
        if (!element.contains(target)) {
          const action = element.getAttribute('lvt-click-away');
          if (!action) return;

          // Build and send message
          const message: any = { action, data: {} };

          // Add lvt-data-* attributes
          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith('lvt-data-')) {
              const key = attr.name.replace('lvt-data-', '');
              message.data[key] = this.parseValue(attr.value);
            }
          });

          // Add lvt-value-* attributes
          Array.from(element.attributes).forEach((attr) => {
            if (attr.name.startsWith('lvt-value-')) {
              const key = attr.name.replace('lvt-value-', '');
              message.data[key] = this.parseValue(attr.value);
            }
          });

          this.send(message);
        }
      });
    };

    (document as any)[listenerKey] = listener;
    document.addEventListener('click', listener);
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
      const updateResponse: UpdateResponse = await response.json();
      if (this.wrapperElement) {
        this.updateDOM(this.wrapperElement, updateResponse.tree, updateResponse.meta);
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
      // Check if this is a differential operations array
      const isDifferentialOps = Array.isArray(value) && value.length > 0 &&
          Array.isArray(value[0]) && typeof value[0][0] === 'string';

      if (isDifferentialOps) {
        // This is a differential operations array - just store it
        // rangeState will be used during rendering
        this.treeState[key] = value;
        changed = true;
      } else {
        // Regular value update (including initial range structures with d and s)
        if (JSON.stringify(this.treeState[key]) !== JSON.stringify(value)) {
          this.treeState[key] = value;
          changed = true;
        }
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
        // Store the range items AND statics for differential operations
        if (fieldKey) {
          this.rangeState[fieldKey] = {
            items: value.d,
            statics: value.s
          };
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
   * Find the position where the key attribute appears in statics array
   * Priority order: data-lvt-key, data-key, key, id (same as server-side)
   */
  private findKeyPositionFromStatics(statics: any[]): number {
    const keyAttrs = ['data-lvt-key="', 'data-key="', 'key="', 'id="'];

    for (let i = 0; i < statics.length; i++) {
      const staticStr = String(statics[i]);
      for (const keyAttr of keyAttrs) {
        if (staticStr.includes(keyAttr)) {
          return i; // The next position after this static contains the key value
        }
      }
    }

    return 0; // Default to position 0 for backward compatibility
  }

  /**
   * Get item key from item data using statics to find correct position
   */
  private getItemKey(item: any, statics: any[]): string | null {
    const keyPos = this.findKeyPositionFromStatics(statics);
    const keyPosStr = keyPos.toString();
    return item[keyPosStr] || null;
  }

  /**
   * Apply differential operations to existing range items
   * Operations: ["r", key] for remove, ["u", key, changes] for update, ["a", items] for append
   */
  private applyDifferentialOperations(operations: any[], fieldKey?: string): string {
    if (!fieldKey || !this.rangeState[fieldKey]) {
      // If we don't have previous range state, we can't apply differential operations
      // This happens on the first load - just return empty for now
      return '';
    }

    const rangeData = this.rangeState[fieldKey];
    const currentItems = [...rangeData.items]; // Clone current items
    const statics = rangeData.statics;

    // Apply each operation
    for (const operation of operations) {
      if (!Array.isArray(operation) || operation.length < 2) {
        continue;
      }

      const opType = operation[0];

      switch (opType) {
        case 'r': // Remove: ["r", key]
          const removeKey = operation[1];
          const removeIndex = currentItems.findIndex((item: any) =>
            this.getItemKey(item, statics) === removeKey
          );
          if (removeIndex >= 0) {
            currentItems.splice(removeIndex, 1);
          }
          break;

        case 'u': // Update: ["u", key, changes]
          const updateKey = operation[1];
          const changes = operation[2];
          const updateIndex = currentItems.findIndex((item: any) =>
            this.getItemKey(item, statics) === updateKey
          );
          if (updateIndex >= 0 && changes) {
            // Merge the changes into the existing item
            currentItems[updateIndex] = { ...currentItems[updateIndex], ...changes };
          }
          break;

        case 'a': // Append: ["a", items] (items can be single item or array)
          const itemsToAppend = operation[1];
          if (itemsToAppend) {
            if (Array.isArray(itemsToAppend)) {
              currentItems.push(...itemsToAppend);
            } else {
              currentItems.push(itemsToAppend);
            }
          }
          break;

        case 'i': // Insert: ["i", targetKey, position, items]
          const targetKey = operation[1];
          const position = operation[2];
          const insertData = operation[3];

          if (insertData) {
            const itemsToInsert = Array.isArray(insertData) ? insertData : [insertData];

            if (targetKey === null) {
              if (position === "start") {
                currentItems.unshift(...itemsToInsert);
              } else { // "end"
                currentItems.push(...itemsToInsert);
              }
            } else {
              const targetIndex = currentItems.findIndex((item: any) =>
                this.getItemKey(item, statics) === targetKey
              );
              if (targetIndex >= 0) {
                const insertIndex = position === "before" ? targetIndex : targetIndex + 1;
                currentItems.splice(insertIndex, 0, ...itemsToInsert);
              }
            }
          }
          break;

        case 'o': // Order (reordering): ["o", [key1, key2, ...]]
          const newOrder = operation[1] as string[];
          const reorderedItems: any[] = [];

          // Build a map of current items by key for efficient lookup
          const itemsByKey = new Map();
          for (const item of currentItems) {
            const itemKey = this.getItemKey(item, statics);
            if (itemKey) {
              itemsByKey.set(itemKey, item);
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

    // Update our range state with new items (keep statics unchanged)
    this.rangeState[fieldKey] = {
      items: currentItems,
      statics: statics
    };

    // IMPORTANT: Replace the differential operations in treeState with the updated range structure
    // This prevents the operations from being applied again on the next render
    this.treeState[fieldKey] = {
      d: currentItems,
      s: statics
    };

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
    // First check if we have it in rangeState (from differential operations)
    if (this.rangeState[fieldKey]) {
      return {
        d: this.rangeState[fieldKey].items,
        s: this.rangeState[fieldKey].statics
      };
    }

    // Fallback to treeState
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
    const result = items.map((item: any) => {
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

    console.log('[renderItemsWithStatics] statics:', statics);
    console.log('[renderItemsWithStatics] items count:', items.length);
    console.log('[renderItemsWithStatics] result:', result.substring(0, 200));

    return result;
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
   * @param meta - Optional metadata about the update (action, success, errors)
   */
  updateDOM(element: Element, update: TreeNode, meta?: ResponseMetadata): void {
    // Apply update to internal state and get reconstructed HTML
    const result = this.applyUpdate(update);

    if (!result.changed && !update.s) {
      // No changes detected and no statics in update, skip morphdom
      return;
    }

    // Create a temporary wrapper to hold the new content
    // We need to create a DOM element of the same type as 'element' to avoid browser HTML corrections
    // For example, if we put <tr> elements in a <div>, the browser strips them out
    const tempWrapper = document.createElement(element.tagName);

    console.log('[updateDOM] element.tagName:', element.tagName);
    console.log('[updateDOM] result.html (first 500 chars):', result.html.substring(0, 500));
    console.log('[updateDOM] Has <table> tag:', result.html.includes('<table>'));
    console.log('[updateDOM] Has <tbody> tag:', result.html.includes('<tbody>'));
    console.log('[updateDOM] Has <tr> tag:', result.html.includes('<tr'));

    tempWrapper.innerHTML = result.html;

    console.log('[updateDOM] tempWrapper.innerHTML after setting (first 500 chars):', tempWrapper.innerHTML.substring(0, 500));
    console.log('[updateDOM] tempWrapper has <table>:', tempWrapper.innerHTML.includes('<table>'));
    console.log('[updateDOM] tempWrapper has <tbody>:', tempWrapper.innerHTML.includes('<tbody>'));
    console.log('[updateDOM] tempWrapper has <tr>:', tempWrapper.innerHTML.includes('<tr'));

    // Use morphdom to efficiently update the element
    morphdom(element, tempWrapper, {
      childrenOnly: true,  // Only update children, preserve the wrapper element itself
      getNodeKey: (node: any) => {
        // Use data-key or data-lvt-key for efficient reconciliation
        if (node.nodeType === 1) {
          return node.getAttribute('data-key') || node.getAttribute('data-lvt-key') || undefined;
        }
      },
      onBeforeElUpdated: (fromEl, toEl) => {
        // Only update if content actually changed
        if (fromEl.isEqualNode(toEl)) {
          return false;
        }
        // Execute lvt-updated lifecycle hook
        this.executeLifecycleHook(fromEl, 'lvt-updated');
        return true;
      },
      onNodeAdded: (node) => {
        // Execute lvt-mounted lifecycle hook
        if (node.nodeType === Node.ELEMENT_NODE) {
          this.executeLifecycleHook(node as Element, 'lvt-mounted');
        }
      },
      onBeforeNodeDiscarded: (node) => {
        // Execute lvt-destroyed lifecycle hook
        if (node.nodeType === Node.ELEMENT_NODE) {
          this.executeLifecycleHook(node as Element, 'lvt-destroyed');
        }
        return true;
      }
    });

    // Handle form lifecycle if metadata is present
    if (meta) {
      this.handleFormLifecycle(meta);
    }
  }

  /**
   * Handle form lifecycle after receiving server response
   * @param meta - Response metadata containing success status and errors
   */
  private handleFormLifecycle(meta: ResponseMetadata): void {
    // Emit lvt:done event
    if (this.activeForm) {
      this.activeForm.dispatchEvent(new CustomEvent('lvt:done', { detail: meta }));
    }

    if (meta.success) {
      // Success: no validation errors
      if (this.activeForm) {
        // Emit lvt:success event
        this.activeForm.dispatchEvent(new CustomEvent('lvt:success', { detail: meta }));

        // Auto-reset form unless lvt-preserve is present
        if (!this.activeForm.hasAttribute('lvt-preserve')) {
          this.activeForm.reset();
        }
      }
    } else {
      // Error: validation errors present
      if (this.activeForm) {
        // Emit lvt:error event
        this.activeForm.dispatchEvent(new CustomEvent('lvt:error', { detail: meta }));
      }
    }

    // Re-enable button if it was disabled
    if (this.activeButton && this.originalButtonText !== null) {
      this.activeButton.disabled = false;
      this.activeButton.textContent = this.originalButtonText;
    }

    // Clear active form/button state
    this.activeForm = null;
    this.activeButton = null;
    this.originalButtonText = null;
  }

  /**
   * Execute lifecycle hook on an element
   * @param element - Element with lifecycle hook attribute
   * @param hookName - Name of the lifecycle hook attribute (e.g., 'lvt-mounted')
   */
  private executeLifecycleHook(element: Element, hookName: string): void {
    const hookValue = element.getAttribute(hookName);
    if (!hookValue) {
      return;
    }

    try {
      // Create a function from the hook value and execute it
      // The function has access to 'this' (the element) and 'event'
      const hookFunction = new Function('element', hookValue);
      hookFunction.call(element, element);
    } catch (error) {
      console.error(`Error executing ${hookName} hook:`, error);
    }
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

/**
 * Debounce function: delays execution until after a pause in calls
 * @param func - Function to debounce
 * @param wait - Wait time in milliseconds
 * @returns Debounced function
 */
function debounce<T extends (...args: any[]) => any>(
  func: T,
  wait: number
): (...args: Parameters<T>) => void {
  let timeout: number | null = null;

  return function(this: any, ...args: Parameters<T>) {
    const context = this;

    if (timeout !== null) {
      clearTimeout(timeout);
    }

    timeout = window.setTimeout(() => {
      func.apply(context, args);
    }, wait);
  };
}

/**
 * Throttle function: limits execution to at most once per time period
 * First call executes immediately, subsequent calls are delayed
 * @param func - Function to throttle
 * @param limit - Minimum time between executions in milliseconds
 * @returns Throttled function
 */
function throttle<T extends (...args: any[]) => any>(
  func: T,
  limit: number
): (...args: Parameters<T>) => void {
  let inThrottle = false;

  return function(this: any, ...args: Parameters<T>) {
    const context = this;

    if (!inThrottle) {
      func.apply(context, args);
      inThrottle = true;

      setTimeout(() => {
        inThrottle = false;
      }, limit);
    }
  };
}

// Auto-initialize when script loads (for browser use)
if (typeof window !== 'undefined') {
  LiveTemplateClient.autoInit();
}
