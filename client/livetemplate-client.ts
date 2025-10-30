/**
 * LiveTemplate TypeScript Client
 *
 * Reconstructs HTML from tree-based updates using cached static structure,
 * following the Phoenix LiveView optimization approach.
 */

import morphdom from "morphdom";
import { FocusManager } from "./dom/focus-manager";
import {
  handleAnimateDirectives,
  handleHighlightDirectives,
  handleScrollDirectives,
} from "./dom/directives";
import { EventDelegator } from "./dom/event-delegation";
import { ObserverManager } from "./dom/observer-manager";
import { ModalManager } from "./dom/modal-manager";
import { LoadingIndicator } from "./dom/loading-indicator";
import { FormDisabler } from "./dom/form-disabler";
import { TreeRenderer } from "./state/tree-renderer";
import { FormLifecycleManager } from "./state/form-lifecycle-manager";
import { WebSocketManager } from "./transport/websocket";
import type {
  LiveTemplateClientOptions,
  ResponseMetadata,
  TreeNode,
  UpdateResponse,
  UpdateResult,
} from "./types";
import { createLogger, Logger } from "./utils/logger";
export { loadAndApplyUpdate, compareHTML } from "./utils/testing";

export class LiveTemplateClient {
  private readonly treeRenderer: TreeRenderer;
  private readonly focusManager: FocusManager;
  private readonly logger: Logger;
  private lvtId: string | null = null;

  // Transport properties
  private webSocketManager: WebSocketManager;
  public ws: WebSocket | null = null;
  private wrapperElement: Element | null = null;
  private options: LiveTemplateClientOptions;
  private useHTTP: boolean = false; // True when WebSocket is unavailable
  private sessionCookie: string | null = null; // For HTTP mode session tracking

  // Rate limiting: cache of debounced/throttled handlers per element+eventType
  private rateLimitedHandlers: WeakMap<Element, Map<string, Function>> =
    new WeakMap();

  private eventDelegator: EventDelegator;
  private observerManager: ObserverManager;
  private modalManager: ModalManager;
  private formLifecycleManager: FormLifecycleManager;
  private loadingIndicator: LoadingIndicator;
  private formDisabler: FormDisabler;

  // Initialization tracking for loading indicator
  private isInitialized: boolean = false;

  // Message tracking for deterministic E2E testing
  private messageCount: number = 0;

  constructor(options: LiveTemplateClientOptions = {}) {
    const { logger: providedLogger, logLevel, debug, ...restOptions } = options;
    const resolvedLevel = logLevel ?? (debug ? "debug" : "info");
    const baseLogger = providedLogger ?? createLogger({ level: resolvedLevel });

    if (providedLogger) {
      if (logLevel) {
        providedLogger.setLevel(logLevel);
      } else if (debug) {
        providedLogger.setLevel("debug");
      }
    } else {
      baseLogger.setLevel(resolvedLevel);
    }

    this.logger = baseLogger.child("Client");

    this.options = {
      autoReconnect: false, // Disable autoReconnect by default to avoid connection loops
      reconnectDelay: 1000,
      liveUrl: window.location.pathname, // Connect to current page
      ...restOptions,
    };

    this.treeRenderer = new TreeRenderer(this.logger.child("TreeRenderer"));
    this.focusManager = new FocusManager(this.logger.child("FocusManager"));

    this.modalManager = new ModalManager(this.logger.child("ModalManager"));
    this.formLifecycleManager = new FormLifecycleManager(this.modalManager);
    this.loadingIndicator = new LoadingIndicator();
    this.formDisabler = new FormDisabler();

    this.eventDelegator = new EventDelegator(
      {
        getWrapperElement: () => this.wrapperElement,
        getRateLimitedHandlers: () => this.rateLimitedHandlers,
        parseValue: (value: string) => this.parseValue(value),
        send: (message: any) => this.send(message),
        setActiveSubmission: (
          form: HTMLFormElement | null,
          button: HTMLButtonElement | null,
          originalButtonText: string | null
        ) =>
          this.formLifecycleManager.setActiveSubmission(
            form,
            button,
            originalButtonText
          ),
        openModal: (modalId: string) => this.modalManager.open(modalId),
        closeModal: (modalId: string) => this.modalManager.close(modalId),
        getWebSocketReadyState: () => this.webSocketManager.getReadyState(),
      },
      this.logger.child("EventDelegator")
    );

    this.observerManager = new ObserverManager(
      {
        getWrapperElement: () => this.wrapperElement,
        send: (message: any) => this.send(message),
      },
      this.logger.child("ObserverManager")
    );

    this.webSocketManager = new WebSocketManager({
      options: this.options,
      logger: this.logger.child("Transport"),
      onConnected: () => {
        this.ws = this.webSocketManager.getSocket();
        this.logger.info("WebSocket connected");
        this.options.onConnect?.();
        this.wrapperElement?.dispatchEvent(new Event("lvt:connected"));
      },
      onDisconnected: () => {
        this.ws = null;
        this.logger.info("WebSocket disconnected");
        this.options.onDisconnect?.();
        this.wrapperElement?.dispatchEvent(new Event("lvt:disconnected"));
      },
      onMessage: (response, event) => {
        this.handleWebSocketPayload(response, event);
      },
      onReconnectAttempt: () => {
        this.logger.info("Attempting to reconnect...");
      },
      onError: (error) => {
        this.logger.error("WebSocket error:", error);
        this.options.onError?.(error);
      },
    });
  }

  /**
   * Auto-initialize when DOM is ready
   * Called automatically when script loads
   */
  static autoInit(): void {
    const autoInitLogger = createLogger({ scope: "Client:autoInit" });
    const init = () => {
      const wrapper = document.querySelector("[data-lvt-id]");
      if (wrapper) {
        const client = new LiveTemplateClient();
        client.wrapperElement = wrapper;

        // Check if loading indicator should be shown
        const shouldShowLoading =
          wrapper.getAttribute("data-lvt-loading") === "true";
        if (shouldShowLoading) {
          client.loadingIndicator.show();
          client.formDisabler.disable(client.wrapperElement);
        }

        client.connect().catch((error) => {
          autoInitLogger.error("Auto-initialization connect failed:", error);
        });

        // Expose as global for programmatic access
        (window as any).liveTemplateClient = client;
      }
    };

    // Initialize when DOM is ready
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", init);
    } else {
      init();
    }
  }

  /**
   * Handle server-sent updates delivered via WebSocket or HTTP fallback.
   */
  private handleWebSocketPayload(
    response: UpdateResponse,
    event?: MessageEvent<string>
  ): void {
    if (event) {
      (window as any).__lastWSMessage = event.data;

      if (!(window as any).__wsMessages) {
        (window as any).__wsMessages = [];
      }
      (window as any).__wsMessages.push(response);
    }

    if (!this.isInitialized) {
      this.loadingIndicator.hide();
      this.formDisabler.enable(this.wrapperElement);
      if (
        this.wrapperElement &&
        this.wrapperElement.hasAttribute("data-lvt-loading")
      ) {
        this.wrapperElement.removeAttribute("data-lvt-loading");
      }
      this.isInitialized = true;
    }

    if (this.wrapperElement) {
      this.updateDOM(this.wrapperElement, response.tree, response.meta);
      this.messageCount++;
      (window as any).__wsMessageCount = this.messageCount;

      this.wrapperElement.dispatchEvent(
        new CustomEvent("lvt:updated", {
          detail: {
            messageCount: this.messageCount,
            action: response.meta?.action,
            success: response.meta?.success,
          },
        })
      );
    }
  }

  /**
   * Connect to WebSocket and start receiving updates
   * @param wrapperSelector - CSS selector for the LiveTemplate wrapper (defaults to '[data-lvt-id]')
   */
  async connect(wrapperSelector: string = "[data-lvt-id]"): Promise<void> {
    // Find the wrapper element
    this.wrapperElement = document.querySelector(wrapperSelector);
    if (!this.wrapperElement) {
      throw new Error(
        `LiveTemplate wrapper not found with selector: ${wrapperSelector}`
      );
    }

    this.webSocketManager.disconnect();

    const connectionResult = await this.webSocketManager.connect();
    this.useHTTP = !connectionResult.usingWebSocket;

    if (this.useHTTP) {
      this.ws = null;
      this.logger.info("WebSocket not available, using HTTP mode");
      this.options.onConnect?.();
      if (connectionResult.initialState && this.wrapperElement) {
        this.handleWebSocketPayload(connectionResult.initialState);
      }
    }
    // Set up event delegation for lvt-* attributes
    this.eventDelegator.setupEventDelegation();

    // Set up window-* event delegation
    this.eventDelegator.setupWindowEventDelegation();

    // Set up click-away delegation
    this.eventDelegator.setupClickAwayDelegation();

    // Set up modal delegation
    this.eventDelegator.setupModalDelegation();

    // Initialize focus tracking
    this.focusManager.attach(this.wrapperElement);

    // Set up infinite scroll observers
    this.observerManager.setupInfiniteScrollObserver();
    this.observerManager.setupInfiniteScrollMutationObserver();
  }

  /**
   * Disconnect from WebSocket
   */
  disconnect(): void {
    this.webSocketManager.disconnect();
    this.ws = null;
    this.useHTTP = false;
    this.observerManager.teardown();
    this.formLifecycleManager.reset();
    this.loadingIndicator.hide();
    this.formDisabler.enable(this.wrapperElement);
  }

  /**
   * Determine whether the client finished its initial load and has an active transport.
   */
  isReady(): boolean {
    const wrapper = this.wrapperElement;

    if (!wrapper || wrapper.hasAttribute("data-lvt-loading")) {
      return false;
    }

    if (this.useHTTP) {
      return true;
    }

    const readyState = this.webSocketManager.getReadyState();
    return readyState === WebSocket.OPEN;
  }

  /**
   * Send a message to the server via WebSocket or HTTP
   * @param message - Message to send (will be JSON stringified)
   */
  send(message: any): void {
    // Debug flag for testing
    (window as any).__lvtSendCalled = true;
    (window as any).__lvtMessageAction = message?.action;

    const readyState = this.webSocketManager.getReadyState();

    if (this.logger.isDebugEnabled()) {
      this.logger.debug("send() invoked", {
        message,
        useHTTP: this.useHTTP,
        hasWebSocket: readyState !== undefined,
        readyState,
      });
    }

    if (this.useHTTP) {
      // HTTP mode: send via POST and handle response
      this.logger.debug("Using HTTP mode for send");
      (window as any).__lvtSendPath = "http";
      this.sendHTTP(message);
    } else if (readyState === WebSocket.OPEN) {
      // WebSocket mode
      this.logger.debug("Sending via WebSocket");
      (window as any).__lvtSendPath = "websocket";
      (window as any).__lvtWSMessage = JSON.stringify(message);
      this.webSocketManager.send(JSON.stringify(message));
      this.logger.debug("WebSocket send complete");
      (window as any).__lvtWSSendComplete = true;
    } else if (readyState !== undefined) {
      // WebSocket is connecting or closing, fall back to HTTP temporarily
      this.logger.warn(
        `WebSocket not ready (state: ${readyState}), using HTTP fallback`
      );
      (window as any).__lvtSendPath = "http-fallback";
      this.sendHTTP(message);
    } else {
      this.logger.error("No transport available");
      (window as any).__lvtSendPath = "no-transport";
    }
  }

  /**
   * Send action via HTTP POST
   */
  private async sendHTTP(message: any): Promise<void> {
    try {
      const liveUrl = this.options.liveUrl || "/live";
      const response = await fetch(liveUrl, {
        method: "POST",
        credentials: "include", // Include cookies for session
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(message),
      });

      if (!response.ok) {
        throw new Error(`HTTP request failed: ${response.status}`);
      }

      // Handle the update response
      const updateResponse: UpdateResponse = await response.json();
      if (this.wrapperElement) {
        this.updateDOM(
          this.wrapperElement,
          updateResponse.tree,
          updateResponse.meta
        );
      }
    } catch (error) {
      this.logger.error("Failed to send HTTP request:", error);
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
    if (value === "true") return true;
    if (value === "false") return false;

    // Return as string
    return value;
  }

  /**
   * Apply an update to the current state and reconstruct HTML
   * @param update - Tree update object from LiveTemplate server
   * @returns Reconstructed HTML and whether anything changed
   */
  applyUpdate(update: TreeNode): UpdateResult {
    return this.treeRenderer.applyUpdate(update);
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
    const wrapperStart = `<div data-lvt-id="${this.lvtId || "lvt-unknown"}">`;
    const wrapperEnd = "</div>";
    const newBodyContent = wrapperStart + innerContent + wrapperEnd;

    return existingHTML.replace(
      /<body>[\s\S]*?<\/body>/,
      `<body>${newBodyContent}</body>`
    );
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

    // Helper to recursively check if there are any statics in the tree
    const hasStaticsInTree = (node: any): boolean => {
      if (!node || typeof node !== "object") return false;
      if (node.s && Array.isArray(node.s)) return true;
      return Object.values(node).some((v) => hasStaticsInTree(v));
    };

    if (!result.changed && !hasStaticsInTree(update)) {
      // No changes detected and no statics in update, skip morphdom
      return;
    }

    // Create a temporary wrapper to hold the new content
    // We need to create a DOM element of the same type as 'element' to avoid browser HTML corrections
    // For example, if we put <tr> elements in a <div>, the browser strips them out
    const tempWrapper = document.createElement(element.tagName);

    if (this.logger.isDebugEnabled()) {
      this.logger.debug("[updateDOM] element.tagName:", element.tagName);
      this.logger.debug(
        "[updateDOM] result.html (first 500 chars):",
        result.html.substring(0, 500)
      );
      this.logger.debug(
        "[updateDOM] Has <table> tag:",
        result.html.includes("<table>")
      );
      this.logger.debug(
        "[updateDOM] Has <tbody> tag:",
        result.html.includes("<tbody>")
      );
      this.logger.debug(
        "[updateDOM] Has <tr> tag:",
        result.html.includes("<tr")
      );
    }

    tempWrapper.innerHTML = result.html;

    if (this.logger.isDebugEnabled()) {
      this.logger.debug(
        "[updateDOM] tempWrapper.innerHTML (first 500 chars):",
        tempWrapper.innerHTML.substring(0, 500)
      );
      this.logger.debug(
        "[updateDOM] tempWrapper has <table>:",
        tempWrapper.innerHTML.includes("<table>")
      );
      this.logger.debug(
        "[updateDOM] tempWrapper has <tbody>:",
        tempWrapper.innerHTML.includes("<tbody>")
      );
      this.logger.debug(
        "[updateDOM] tempWrapper has <tr>:",
        tempWrapper.innerHTML.includes("<tr")
      );
    }

    // Use morphdom to efficiently update the element
    morphdom(element, tempWrapper, {
      childrenOnly: true, // Only update children, preserve the wrapper element itself
      getNodeKey: (node: any) => {
        // Use data-key or data-lvt-key for efficient reconciliation
        if (node.nodeType === 1) {
          return (
            node.getAttribute("data-key") ||
            node.getAttribute("data-lvt-key") ||
            undefined
          );
        }
      },
      onBeforeElUpdated: (fromEl, toEl) => {
        // Preserve value for the last focused textual input
        const lastFocused = this.focusManager.getLastFocusedElement();
        if (lastFocused && this.focusManager.isTextualInput(fromEl)) {
          if (fromEl === lastFocused) {
            // Preserve the current value being typed
            (toEl as any).value = (fromEl as any).value;
          }
        }

        // Only update if content actually changed
        if (fromEl.isEqualNode(toEl)) {
          return false;
        }
        // Execute lvt-updated lifecycle hook
        this.executeLifecycleHook(fromEl, "lvt-updated");
        return true;
      },
      onNodeAdded: (node) => {
        // Execute lvt-mounted lifecycle hook
        if (node.nodeType === Node.ELEMENT_NODE) {
          this.executeLifecycleHook(node as Element, "lvt-mounted");
        }
      },
      onBeforeNodeDiscarded: (node) => {
        // Execute lvt-destroyed lifecycle hook
        if (node.nodeType === Node.ELEMENT_NODE) {
          this.executeLifecycleHook(node as Element, "lvt-destroyed");
        }
        return true;
      },
    });

    // Restore focus to previously focused element
    this.focusManager.restoreFocusedElement();

    // Handle scroll directives
    handleScrollDirectives(element);

    // Handle highlight directives
    handleHighlightDirectives(element);

    // Handle animate directives
    handleAnimateDirectives(element);

    // Handle form lifecycle if metadata is present
    if (meta) {
      this.formLifecycleManager.handleResponse(meta);
    }
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
      const hookFunction = new Function("element", hookValue);
      hookFunction.call(element, element);
    } catch (error) {
      this.logger.error(`Error executing ${hookName} hook:`, error);
    }
  }

  /**
   * Reset client state (useful for testing)
   */
  reset(): void {
    this.treeRenderer.reset();
    this.focusManager.reset();
    this.observerManager.teardown();
    this.formLifecycleManager.reset();
    this.loadingIndicator.hide();
    this.formDisabler.enable(this.wrapperElement);
    this.lvtId = null;
  }

  /**
   * Get current tree state (for debugging)
   */
  getTreeState(): TreeNode {
    return this.treeRenderer.getTreeState();
  }

  /**
   * Get the static structure if available
   */
  getStaticStructure(): string[] | null {
    return this.treeRenderer.getStaticStructure();
  }
}

// Auto-initialize when script loads (for browser use)
if (typeof window !== "undefined") {
  LiveTemplateClient.autoInit();
}
