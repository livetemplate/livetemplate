"use strict";
var LiveTemplateClient = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __require = /* @__PURE__ */ ((x) => typeof require !== "undefined" ? require : typeof Proxy !== "undefined" ? new Proxy(x, {
    get: (a, b) => (typeof require !== "undefined" ? require : a)[b]
  }) : x)(function(x) {
    if (typeof require !== "undefined") return require.apply(this, arguments);
    throw Error('Dynamic require of "' + x + '" is not supported');
  });
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // livetemplate-client.ts
  var livetemplate_client_exports = {};
  __export(livetemplate_client_exports, {
    LiveTemplateClient: () => LiveTemplateClient,
    compareHTML: () => compareHTML,
    loadAndApplyUpdate: () => loadAndApplyUpdate
  });

  // node_modules/morphdom/dist/morphdom-esm.js
  var DOCUMENT_FRAGMENT_NODE = 11;
  function morphAttrs(fromNode, toNode) {
    var toNodeAttrs = toNode.attributes;
    var attr;
    var attrName;
    var attrNamespaceURI;
    var attrValue;
    var fromValue;
    if (toNode.nodeType === DOCUMENT_FRAGMENT_NODE || fromNode.nodeType === DOCUMENT_FRAGMENT_NODE) {
      return;
    }
    for (var i = toNodeAttrs.length - 1; i >= 0; i--) {
      attr = toNodeAttrs[i];
      attrName = attr.name;
      attrNamespaceURI = attr.namespaceURI;
      attrValue = attr.value;
      if (attrNamespaceURI) {
        attrName = attr.localName || attrName;
        fromValue = fromNode.getAttributeNS(attrNamespaceURI, attrName);
        if (fromValue !== attrValue) {
          if (attr.prefix === "xmlns") {
            attrName = attr.name;
          }
          fromNode.setAttributeNS(attrNamespaceURI, attrName, attrValue);
        }
      } else {
        fromValue = fromNode.getAttribute(attrName);
        if (fromValue !== attrValue) {
          fromNode.setAttribute(attrName, attrValue);
        }
      }
    }
    var fromNodeAttrs = fromNode.attributes;
    for (var d = fromNodeAttrs.length - 1; d >= 0; d--) {
      attr = fromNodeAttrs[d];
      attrName = attr.name;
      attrNamespaceURI = attr.namespaceURI;
      if (attrNamespaceURI) {
        attrName = attr.localName || attrName;
        if (!toNode.hasAttributeNS(attrNamespaceURI, attrName)) {
          fromNode.removeAttributeNS(attrNamespaceURI, attrName);
        }
      } else {
        if (!toNode.hasAttribute(attrName)) {
          fromNode.removeAttribute(attrName);
        }
      }
    }
  }
  var range;
  var NS_XHTML = "http://www.w3.org/1999/xhtml";
  var doc = typeof document === "undefined" ? void 0 : document;
  var HAS_TEMPLATE_SUPPORT = !!doc && "content" in doc.createElement("template");
  var HAS_RANGE_SUPPORT = !!doc && doc.createRange && "createContextualFragment" in doc.createRange();
  function createFragmentFromTemplate(str) {
    var template = doc.createElement("template");
    template.innerHTML = str;
    return template.content.childNodes[0];
  }
  function createFragmentFromRange(str) {
    if (!range) {
      range = doc.createRange();
      range.selectNode(doc.body);
    }
    var fragment = range.createContextualFragment(str);
    return fragment.childNodes[0];
  }
  function createFragmentFromWrap(str) {
    var fragment = doc.createElement("body");
    fragment.innerHTML = str;
    return fragment.childNodes[0];
  }
  function toElement(str) {
    str = str.trim();
    if (HAS_TEMPLATE_SUPPORT) {
      return createFragmentFromTemplate(str);
    } else if (HAS_RANGE_SUPPORT) {
      return createFragmentFromRange(str);
    }
    return createFragmentFromWrap(str);
  }
  function compareNodeNames(fromEl, toEl) {
    var fromNodeName = fromEl.nodeName;
    var toNodeName = toEl.nodeName;
    var fromCodeStart, toCodeStart;
    if (fromNodeName === toNodeName) {
      return true;
    }
    fromCodeStart = fromNodeName.charCodeAt(0);
    toCodeStart = toNodeName.charCodeAt(0);
    if (fromCodeStart <= 90 && toCodeStart >= 97) {
      return fromNodeName === toNodeName.toUpperCase();
    } else if (toCodeStart <= 90 && fromCodeStart >= 97) {
      return toNodeName === fromNodeName.toUpperCase();
    } else {
      return false;
    }
  }
  function createElementNS(name, namespaceURI) {
    return !namespaceURI || namespaceURI === NS_XHTML ? doc.createElement(name) : doc.createElementNS(namespaceURI, name);
  }
  function moveChildren(fromEl, toEl) {
    var curChild = fromEl.firstChild;
    while (curChild) {
      var nextChild = curChild.nextSibling;
      toEl.appendChild(curChild);
      curChild = nextChild;
    }
    return toEl;
  }
  function syncBooleanAttrProp(fromEl, toEl, name) {
    if (fromEl[name] !== toEl[name]) {
      fromEl[name] = toEl[name];
      if (fromEl[name]) {
        fromEl.setAttribute(name, "");
      } else {
        fromEl.removeAttribute(name);
      }
    }
  }
  var specialElHandlers = {
    OPTION: function(fromEl, toEl) {
      var parentNode = fromEl.parentNode;
      if (parentNode) {
        var parentName = parentNode.nodeName.toUpperCase();
        if (parentName === "OPTGROUP") {
          parentNode = parentNode.parentNode;
          parentName = parentNode && parentNode.nodeName.toUpperCase();
        }
        if (parentName === "SELECT" && !parentNode.hasAttribute("multiple")) {
          if (fromEl.hasAttribute("selected") && !toEl.selected) {
            fromEl.setAttribute("selected", "selected");
            fromEl.removeAttribute("selected");
          }
          parentNode.selectedIndex = -1;
        }
      }
      syncBooleanAttrProp(fromEl, toEl, "selected");
    },
    /**
     * The "value" attribute is special for the <input> element since it sets
     * the initial value. Changing the "value" attribute without changing the
     * "value" property will have no effect since it is only used to the set the
     * initial value.  Similar for the "checked" attribute, and "disabled".
     */
    INPUT: function(fromEl, toEl) {
      syncBooleanAttrProp(fromEl, toEl, "checked");
      syncBooleanAttrProp(fromEl, toEl, "disabled");
      if (fromEl.value !== toEl.value) {
        fromEl.value = toEl.value;
      }
      if (!toEl.hasAttribute("value")) {
        fromEl.removeAttribute("value");
      }
    },
    TEXTAREA: function(fromEl, toEl) {
      var newValue = toEl.value;
      if (fromEl.value !== newValue) {
        fromEl.value = newValue;
      }
      var firstChild = fromEl.firstChild;
      if (firstChild) {
        var oldValue = firstChild.nodeValue;
        if (oldValue == newValue || !newValue && oldValue == fromEl.placeholder) {
          return;
        }
        firstChild.nodeValue = newValue;
      }
    },
    SELECT: function(fromEl, toEl) {
      if (!toEl.hasAttribute("multiple")) {
        var selectedIndex = -1;
        var i = 0;
        var curChild = fromEl.firstChild;
        var optgroup;
        var nodeName;
        while (curChild) {
          nodeName = curChild.nodeName && curChild.nodeName.toUpperCase();
          if (nodeName === "OPTGROUP") {
            optgroup = curChild;
            curChild = optgroup.firstChild;
            if (!curChild) {
              curChild = optgroup.nextSibling;
              optgroup = null;
            }
          } else {
            if (nodeName === "OPTION") {
              if (curChild.hasAttribute("selected")) {
                selectedIndex = i;
                break;
              }
              i++;
            }
            curChild = curChild.nextSibling;
            if (!curChild && optgroup) {
              curChild = optgroup.nextSibling;
              optgroup = null;
            }
          }
        }
        fromEl.selectedIndex = selectedIndex;
      }
    }
  };
  var ELEMENT_NODE = 1;
  var DOCUMENT_FRAGMENT_NODE$1 = 11;
  var TEXT_NODE = 3;
  var COMMENT_NODE = 8;
  function noop() {
  }
  function defaultGetNodeKey(node) {
    if (node) {
      return node.getAttribute && node.getAttribute("id") || node.id;
    }
  }
  function morphdomFactory(morphAttrs2) {
    return function morphdom2(fromNode, toNode, options) {
      if (!options) {
        options = {};
      }
      if (typeof toNode === "string") {
        if (fromNode.nodeName === "#document" || fromNode.nodeName === "HTML" || fromNode.nodeName === "BODY") {
          var toNodeHtml = toNode;
          toNode = doc.createElement("html");
          toNode.innerHTML = toNodeHtml;
        } else {
          toNode = toElement(toNode);
        }
      } else if (toNode.nodeType === DOCUMENT_FRAGMENT_NODE$1) {
        toNode = toNode.firstElementChild;
      }
      var getNodeKey = options.getNodeKey || defaultGetNodeKey;
      var onBeforeNodeAdded = options.onBeforeNodeAdded || noop;
      var onNodeAdded = options.onNodeAdded || noop;
      var onBeforeElUpdated = options.onBeforeElUpdated || noop;
      var onElUpdated = options.onElUpdated || noop;
      var onBeforeNodeDiscarded = options.onBeforeNodeDiscarded || noop;
      var onNodeDiscarded = options.onNodeDiscarded || noop;
      var onBeforeElChildrenUpdated = options.onBeforeElChildrenUpdated || noop;
      var skipFromChildren = options.skipFromChildren || noop;
      var addChild = options.addChild || function(parent, child) {
        return parent.appendChild(child);
      };
      var childrenOnly = options.childrenOnly === true;
      var fromNodesLookup = /* @__PURE__ */ Object.create(null);
      var keyedRemovalList = [];
      function addKeyedRemoval(key) {
        keyedRemovalList.push(key);
      }
      function walkDiscardedChildNodes(node, skipKeyedNodes) {
        if (node.nodeType === ELEMENT_NODE) {
          var curChild = node.firstChild;
          while (curChild) {
            var key = void 0;
            if (skipKeyedNodes && (key = getNodeKey(curChild))) {
              addKeyedRemoval(key);
            } else {
              onNodeDiscarded(curChild);
              if (curChild.firstChild) {
                walkDiscardedChildNodes(curChild, skipKeyedNodes);
              }
            }
            curChild = curChild.nextSibling;
          }
        }
      }
      function removeNode(node, parentNode, skipKeyedNodes) {
        if (onBeforeNodeDiscarded(node) === false) {
          return;
        }
        if (parentNode) {
          parentNode.removeChild(node);
        }
        onNodeDiscarded(node);
        walkDiscardedChildNodes(node, skipKeyedNodes);
      }
      function indexTree(node) {
        if (node.nodeType === ELEMENT_NODE || node.nodeType === DOCUMENT_FRAGMENT_NODE$1) {
          var curChild = node.firstChild;
          while (curChild) {
            var key = getNodeKey(curChild);
            if (key) {
              fromNodesLookup[key] = curChild;
            }
            indexTree(curChild);
            curChild = curChild.nextSibling;
          }
        }
      }
      indexTree(fromNode);
      function handleNodeAdded(el) {
        onNodeAdded(el);
        var curChild = el.firstChild;
        while (curChild) {
          var nextSibling = curChild.nextSibling;
          var key = getNodeKey(curChild);
          if (key) {
            var unmatchedFromEl = fromNodesLookup[key];
            if (unmatchedFromEl && compareNodeNames(curChild, unmatchedFromEl)) {
              curChild.parentNode.replaceChild(unmatchedFromEl, curChild);
              morphEl(unmatchedFromEl, curChild);
            } else {
              handleNodeAdded(curChild);
            }
          } else {
            handleNodeAdded(curChild);
          }
          curChild = nextSibling;
        }
      }
      function cleanupFromEl(fromEl, curFromNodeChild, curFromNodeKey) {
        while (curFromNodeChild) {
          var fromNextSibling = curFromNodeChild.nextSibling;
          if (curFromNodeKey = getNodeKey(curFromNodeChild)) {
            addKeyedRemoval(curFromNodeKey);
          } else {
            removeNode(
              curFromNodeChild,
              fromEl,
              true
              /* skip keyed nodes */
            );
          }
          curFromNodeChild = fromNextSibling;
        }
      }
      function morphEl(fromEl, toEl, childrenOnly2) {
        var toElKey = getNodeKey(toEl);
        if (toElKey) {
          delete fromNodesLookup[toElKey];
        }
        if (!childrenOnly2) {
          var beforeUpdateResult = onBeforeElUpdated(fromEl, toEl);
          if (beforeUpdateResult === false) {
            return;
          } else if (beforeUpdateResult instanceof HTMLElement) {
            fromEl = beforeUpdateResult;
            indexTree(fromEl);
          }
          morphAttrs2(fromEl, toEl);
          onElUpdated(fromEl);
          if (onBeforeElChildrenUpdated(fromEl, toEl) === false) {
            return;
          }
        }
        if (fromEl.nodeName !== "TEXTAREA") {
          morphChildren(fromEl, toEl);
        } else {
          specialElHandlers.TEXTAREA(fromEl, toEl);
        }
      }
      function morphChildren(fromEl, toEl) {
        var skipFrom = skipFromChildren(fromEl, toEl);
        var curToNodeChild = toEl.firstChild;
        var curFromNodeChild = fromEl.firstChild;
        var curToNodeKey;
        var curFromNodeKey;
        var fromNextSibling;
        var toNextSibling;
        var matchingFromEl;
        outer: while (curToNodeChild) {
          toNextSibling = curToNodeChild.nextSibling;
          curToNodeKey = getNodeKey(curToNodeChild);
          while (!skipFrom && curFromNodeChild) {
            fromNextSibling = curFromNodeChild.nextSibling;
            if (curToNodeChild.isSameNode && curToNodeChild.isSameNode(curFromNodeChild)) {
              curToNodeChild = toNextSibling;
              curFromNodeChild = fromNextSibling;
              continue outer;
            }
            curFromNodeKey = getNodeKey(curFromNodeChild);
            var curFromNodeType = curFromNodeChild.nodeType;
            var isCompatible = void 0;
            if (curFromNodeType === curToNodeChild.nodeType) {
              if (curFromNodeType === ELEMENT_NODE) {
                if (curToNodeKey) {
                  if (curToNodeKey !== curFromNodeKey) {
                    if (matchingFromEl = fromNodesLookup[curToNodeKey]) {
                      if (fromNextSibling === matchingFromEl) {
                        isCompatible = false;
                      } else {
                        fromEl.insertBefore(matchingFromEl, curFromNodeChild);
                        if (curFromNodeKey) {
                          addKeyedRemoval(curFromNodeKey);
                        } else {
                          removeNode(
                            curFromNodeChild,
                            fromEl,
                            true
                            /* skip keyed nodes */
                          );
                        }
                        curFromNodeChild = matchingFromEl;
                        curFromNodeKey = getNodeKey(curFromNodeChild);
                      }
                    } else {
                      isCompatible = false;
                    }
                  }
                } else if (curFromNodeKey) {
                  isCompatible = false;
                }
                isCompatible = isCompatible !== false && compareNodeNames(curFromNodeChild, curToNodeChild);
                if (isCompatible) {
                  morphEl(curFromNodeChild, curToNodeChild);
                }
              } else if (curFromNodeType === TEXT_NODE || curFromNodeType == COMMENT_NODE) {
                isCompatible = true;
                if (curFromNodeChild.nodeValue !== curToNodeChild.nodeValue) {
                  curFromNodeChild.nodeValue = curToNodeChild.nodeValue;
                }
              }
            }
            if (isCompatible) {
              curToNodeChild = toNextSibling;
              curFromNodeChild = fromNextSibling;
              continue outer;
            }
            if (curFromNodeKey) {
              addKeyedRemoval(curFromNodeKey);
            } else {
              removeNode(
                curFromNodeChild,
                fromEl,
                true
                /* skip keyed nodes */
              );
            }
            curFromNodeChild = fromNextSibling;
          }
          if (curToNodeKey && (matchingFromEl = fromNodesLookup[curToNodeKey]) && compareNodeNames(matchingFromEl, curToNodeChild)) {
            if (!skipFrom) {
              addChild(fromEl, matchingFromEl);
            }
            morphEl(matchingFromEl, curToNodeChild);
          } else {
            var onBeforeNodeAddedResult = onBeforeNodeAdded(curToNodeChild);
            if (onBeforeNodeAddedResult !== false) {
              if (onBeforeNodeAddedResult) {
                curToNodeChild = onBeforeNodeAddedResult;
              }
              if (curToNodeChild.actualize) {
                curToNodeChild = curToNodeChild.actualize(fromEl.ownerDocument || doc);
              }
              addChild(fromEl, curToNodeChild);
              handleNodeAdded(curToNodeChild);
            }
          }
          curToNodeChild = toNextSibling;
          curFromNodeChild = fromNextSibling;
        }
        cleanupFromEl(fromEl, curFromNodeChild, curFromNodeKey);
        var specialElHandler = specialElHandlers[fromEl.nodeName];
        if (specialElHandler) {
          specialElHandler(fromEl, toEl);
        }
      }
      var morphedNode = fromNode;
      var morphedNodeType = morphedNode.nodeType;
      var toNodeType = toNode.nodeType;
      if (!childrenOnly) {
        if (morphedNodeType === ELEMENT_NODE) {
          if (toNodeType === ELEMENT_NODE) {
            if (!compareNodeNames(fromNode, toNode)) {
              onNodeDiscarded(fromNode);
              morphedNode = moveChildren(fromNode, createElementNS(toNode.nodeName, toNode.namespaceURI));
            }
          } else {
            morphedNode = toNode;
          }
        } else if (morphedNodeType === TEXT_NODE || morphedNodeType === COMMENT_NODE) {
          if (toNodeType === morphedNodeType) {
            if (morphedNode.nodeValue !== toNode.nodeValue) {
              morphedNode.nodeValue = toNode.nodeValue;
            }
            return morphedNode;
          } else {
            morphedNode = toNode;
          }
        }
      }
      if (morphedNode === toNode) {
        onNodeDiscarded(fromNode);
      } else {
        if (toNode.isSameNode && toNode.isSameNode(morphedNode)) {
          return;
        }
        morphEl(morphedNode, toNode, childrenOnly);
        if (keyedRemovalList) {
          for (var i = 0, len = keyedRemovalList.length; i < len; i++) {
            var elToRemove = fromNodesLookup[keyedRemovalList[i]];
            if (elToRemove) {
              removeNode(elToRemove, elToRemove.parentNode, false);
            }
          }
        }
      }
      if (!childrenOnly && morphedNode !== fromNode && fromNode.parentNode) {
        if (morphedNode.actualize) {
          morphedNode = morphedNode.actualize(fromNode.ownerDocument || doc);
        }
        fromNode.parentNode.replaceChild(morphedNode, fromNode);
      }
      return morphedNode;
    };
  }
  var morphdom = morphdomFactory(morphAttrs);
  var morphdom_esm_default = morphdom;

  // livetemplate-client.ts
  var LiveTemplateClient = class _LiveTemplateClient {
    constructor(options = {}) {
      this.treeState = {};
      this.rangeState = {};
      // Track range items and statics by field key
      this.lvtId = null;
      // Transport properties
      this.ws = null;
      this.wrapperElement = null;
      this.reconnectTimer = null;
      this.useHTTP = false;
      // True when WebSocket is unavailable
      this.sessionCookie = null;
      // For HTTP mode session tracking
      // Form lifecycle tracking
      this.activeForm = null;
      // The form that submitted the current action
      this.activeButton = null;
      // The button that triggered the action
      this.originalButtonText = null;
      // Original button text for restore
      // Rate limiting: cache of debounced/throttled handlers per element+eventType
      this.rateLimitedHandlers = /* @__PURE__ */ new WeakMap();
      this.options = {
        autoReconnect: false,
        // Disable autoReconnect by default to avoid connection loops
        reconnectDelay: 1e3,
        liveUrl: "/",
        ...options
      };
    }
    /**
     * Auto-initialize when DOM is ready
     * Called automatically when script loads
     */
    static autoInit() {
      const init = () => {
        const wrapper = document.querySelector("[data-lvt-id]");
        if (wrapper) {
          const client = new _LiveTemplateClient();
          client.wrapperElement = wrapper;
          client.connectWebSocket();
          client.setupEventDelegation();
          client.setupWindowEventDelegation();
          client.setupClickAwayDelegation();
          window.liveTemplateClient = client;
        }
      };
      if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
      } else {
        init();
      }
    }
    /**
     * Check if WebSocket is available on the server
     * Makes a HEAD request to probe the endpoint without fetching data
     */
    async checkWebSocketAvailability() {
      try {
        const liveUrl = this.options.liveUrl || "/live";
        const response = await fetch(liveUrl, {
          method: "HEAD"
        });
        const wsHeader = response.headers.get("X-LiveTemplate-WebSocket");
        if (wsHeader) {
          return wsHeader === "enabled";
        }
        return true;
      } catch (error) {
        console.error("Failed to check WebSocket availability:", error);
        return true;
      }
    }
    /**
     * Fetch initial state via HTTP GET
     */
    async fetchInitialState() {
      try {
        const liveUrl = this.options.liveUrl || "/live";
        const response = await fetch(liveUrl, {
          method: "GET",
          credentials: "include",
          // Include cookies for session
          headers: {
            "Accept": "application/json"
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
        console.error("Failed to fetch initial state:", error);
      }
    }
    /**
     * Connect via WebSocket
     */
    connectWebSocket() {
      const wsUrl = this.options.wsUrl || `ws://${window.location.host}${this.options.liveUrl || "/live"}`;
      this.ws = new WebSocket(wsUrl);
      this.ws.onopen = () => {
        console.log("LiveTemplate: WebSocket connected");
        if (this.options.onConnect) {
          this.options.onConnect();
        }
        if (this.wrapperElement) {
          this.wrapperElement.dispatchEvent(new Event("lvt:connected"));
        }
      };
      this.ws.onmessage = (event) => {
        try {
          const response = JSON.parse(event.data);
          if (this.wrapperElement) {
            this.updateDOM(this.wrapperElement, response.tree, response.meta);
          }
        } catch (error) {
          console.error("LiveTemplate error:", error);
        }
      };
      this.ws.onclose = () => {
        console.log("LiveTemplate: WebSocket disconnected");
        if (this.options.onDisconnect) {
          this.options.onDisconnect();
        }
        if (this.wrapperElement) {
          this.wrapperElement.dispatchEvent(new Event("lvt:disconnected"));
        }
        if (this.options.autoReconnect) {
          this.reconnectTimer = window.setTimeout(() => {
            console.log("LiveTemplate: Attempting to reconnect...");
            this.connectWebSocket();
          }, this.options.reconnectDelay);
        }
      };
      this.ws.onerror = (error) => {
        console.error("LiveTemplate WebSocket error:", error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      };
    }
    /**
     * Connect to WebSocket and start receiving updates
     * @param wrapperSelector - CSS selector for the LiveTemplate wrapper (defaults to '[data-lvt-id]')
     */
    async connect(wrapperSelector = "[data-lvt-id]") {
      this.wrapperElement = document.querySelector(wrapperSelector);
      if (!this.wrapperElement) {
        throw new Error(`LiveTemplate wrapper not found with selector: ${wrapperSelector}`);
      }
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      }
      const wsAvailable = await this.checkWebSocketAvailability();
      if (wsAvailable) {
        this.connectWebSocket();
      } else {
        console.log("LiveTemplate: WebSocket not available, using HTTP mode");
        this.useHTTP = true;
        if (this.options.onConnect) {
          this.options.onConnect();
        }
      }
      this.setupEventDelegation();
      this.setupWindowEventDelegation();
      this.setupClickAwayDelegation();
    }
    /**
     * Set up event delegation for elements with lvt-* attributes
     * Uses event delegation to handle dynamically updated elements
     * Supports: lvt-click, lvt-submit, lvt-change, lvt-input, lvt-keydown, lvt-keyup,
     *           lvt-focus, lvt-blur, lvt-mouseenter, lvt-mouseleave
     */
    setupEventDelegation() {
      if (!this.wrapperElement) return;
      const eventTypes = ["click", "submit", "change", "input", "keydown", "keyup", "focus", "blur", "mouseenter", "mouseleave"];
      const wrapperId = this.wrapperElement.getAttribute("data-lvt-id");
      eventTypes.forEach((eventType) => {
        const listenerKey = `__lvt_delegated_${eventType}_${wrapperId}`;
        const existingListener = document[listenerKey];
        if (existingListener) {
          document.removeEventListener(eventType, existingListener, false);
        }
        const listener = (e) => {
          const target = e.target;
          if (!target) return;
          let element = target;
          let inWrapper = false;
          while (element) {
            if (element === this.wrapperElement) {
              inWrapper = true;
              break;
            }
            element = element.parentElement;
          }
          if (!inWrapper) return;
          const attrName = `lvt-${eventType}`;
          element = target;
          while (element && element !== this.wrapperElement.parentElement) {
            let action = element.getAttribute(attrName);
            let actionElement = element;
            if (!action && (eventType === "change" || eventType === "input")) {
              const formElement = element.closest("form");
              if (formElement && formElement.hasAttribute("lvt-change")) {
                action = formElement.getAttribute("lvt-change");
                actionElement = formElement;
              }
            }
            if (action && actionElement) {
              if (eventType === "submit") {
                e.preventDefault();
              }
              if ((eventType === "keydown" || eventType === "keyup") && actionElement.hasAttribute("lvt-key")) {
                const keyFilter = actionElement.getAttribute("lvt-key");
                const keyboardEvent = e;
                if (keyFilter && keyboardEvent.key !== keyFilter) {
                  element = element.parentElement;
                  continue;
                }
              }
              const targetElement = actionElement;
              const handleAction = () => {
                const message = { action, data: {} };
                if (targetElement instanceof HTMLFormElement) {
                  const formData = new FormData(targetElement);
                  formData.forEach((value, key) => {
                    message.data[key] = this.parseValue(value);
                  });
                } else if (eventType === "change" || eventType === "input") {
                  if (targetElement instanceof HTMLInputElement) {
                    const key = targetElement.name || "value";
                    message.data[key] = this.parseValue(targetElement.value);
                  } else if (targetElement instanceof HTMLSelectElement) {
                    const key = targetElement.name || "value";
                    message.data[key] = this.parseValue(targetElement.value);
                  } else if (targetElement instanceof HTMLTextAreaElement) {
                    const key = targetElement.name || "value";
                    message.data[key] = this.parseValue(targetElement.value);
                  }
                }
                Array.from(targetElement.attributes).forEach((attr) => {
                  if (attr.name.startsWith("lvt-data-")) {
                    const key = attr.name.replace("lvt-data-", "");
                    message.data[key] = this.parseValue(attr.value);
                  }
                });
                Array.from(targetElement.attributes).forEach((attr) => {
                  if (attr.name.startsWith("lvt-value-")) {
                    const key = attr.name.replace("lvt-value-", "");
                    message.data[key] = this.parseValue(attr.value);
                  }
                });
                if (eventType === "submit" && targetElement instanceof HTMLFormElement) {
                  this.activeForm = targetElement;
                  const submitEvent = e;
                  const submitButton = submitEvent.submitter;
                  if (submitButton && submitButton.hasAttribute("lvt-disable-with")) {
                    this.activeButton = submitButton;
                    this.originalButtonText = submitButton.textContent;
                    submitButton.disabled = true;
                    submitButton.textContent = submitButton.getAttribute("lvt-disable-with");
                  }
                  targetElement.dispatchEvent(new CustomEvent("lvt:pending", { detail: message }));
                }
                this.send(message);
              };
              const throttleValue = actionElement.getAttribute("lvt-throttle");
              const debounceValue = actionElement.getAttribute("lvt-debounce");
              if (throttleValue || debounceValue) {
                if (!this.rateLimitedHandlers.has(actionElement)) {
                  this.rateLimitedHandlers.set(actionElement, /* @__PURE__ */ new Map());
                }
                const handlerCache = this.rateLimitedHandlers.get(actionElement);
                const cacheKey = `${eventType}:${action}`;
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
              return;
            }
            element = element.parentElement;
          }
        };
        document[listenerKey] = listener;
        document.addEventListener(eventType, listener, false);
      });
    }
    /**
     * Set up window-level event delegation for lvt-window-* attributes
     * Supports: lvt-window-keydown, lvt-window-keyup, lvt-window-scroll,
     *           lvt-window-resize, lvt-window-focus, lvt-window-blur
     */
    setupWindowEventDelegation() {
      if (!this.wrapperElement) return;
      const windowEvents = ["keydown", "keyup", "scroll", "resize", "focus", "blur"];
      const wrapperId = this.wrapperElement.getAttribute("data-lvt-id");
      windowEvents.forEach((eventType) => {
        const listenerKey = `__lvt_window_${eventType}_${wrapperId}`;
        const existingListener = window[listenerKey];
        if (existingListener) {
          window.removeEventListener(eventType, existingListener);
        }
        const listener = (e) => {
          if (!this.wrapperElement) return;
          const attrName = `lvt-window-${eventType}`;
          const elements = this.wrapperElement.querySelectorAll(`[${attrName}]`);
          elements.forEach((element) => {
            const action = element.getAttribute(attrName);
            if (!action) return;
            if ((eventType === "keydown" || eventType === "keyup") && element.hasAttribute("lvt-key")) {
              const keyFilter = element.getAttribute("lvt-key");
              const keyboardEvent = e;
              if (keyFilter && keyboardEvent.key !== keyFilter) {
                return;
              }
            }
            const message = { action, data: {} };
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith("lvt-data-")) {
                const key = attr.name.replace("lvt-data-", "");
                message.data[key] = this.parseValue(attr.value);
              }
            });
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith("lvt-value-")) {
                const key = attr.name.replace("lvt-value-", "");
                message.data[key] = this.parseValue(attr.value);
              }
            });
            const throttleValue = element.getAttribute("lvt-throttle");
            const debounceValue = element.getAttribute("lvt-debounce");
            const handleAction = () => this.send(message);
            if (throttleValue || debounceValue) {
              if (!this.rateLimitedHandlers.has(element)) {
                this.rateLimitedHandlers.set(element, /* @__PURE__ */ new Map());
              }
              const handlerCache = this.rateLimitedHandlers.get(element);
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
        window[listenerKey] = listener;
        window.addEventListener(eventType, listener);
      });
    }
    /**
     * Set up click-away event delegation for lvt-click-away attribute
     * Triggers when clicking outside the element
     */
    setupClickAwayDelegation() {
      if (!this.wrapperElement) return;
      const wrapperId = this.wrapperElement.getAttribute("data-lvt-id");
      const listenerKey = `__lvt_click_away_${wrapperId}`;
      const existingListener = document[listenerKey];
      if (existingListener) {
        document.removeEventListener("click", existingListener);
      }
      const listener = (e) => {
        if (!this.wrapperElement) return;
        const target = e.target;
        const elements = this.wrapperElement.querySelectorAll("[lvt-click-away]");
        elements.forEach((element) => {
          if (!element.contains(target)) {
            const action = element.getAttribute("lvt-click-away");
            if (!action) return;
            const message = { action, data: {} };
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith("lvt-data-")) {
                const key = attr.name.replace("lvt-data-", "");
                message.data[key] = this.parseValue(attr.value);
              }
            });
            Array.from(element.attributes).forEach((attr) => {
              if (attr.name.startsWith("lvt-value-")) {
                const key = attr.name.replace("lvt-value-", "");
                message.data[key] = this.parseValue(attr.value);
              }
            });
            this.send(message);
          }
        });
      };
      document[listenerKey] = listener;
      document.addEventListener("click", listener);
    }
    /**
     * Disconnect from WebSocket
     */
    disconnect() {
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
    send(message) {
      if (this.useHTTP) {
        this.sendHTTP(message);
      } else if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(message));
      } else if (this.ws) {
        console.log("LiveTemplate: WebSocket not ready, using HTTP fallback");
        this.sendHTTP(message);
      } else {
        console.error("LiveTemplate: No transport available");
      }
    }
    /**
     * Send action via HTTP POST
     */
    async sendHTTP(message) {
      try {
        const liveUrl = this.options.liveUrl || "/live";
        const response = await fetch(liveUrl, {
          method: "POST",
          credentials: "include",
          // Include cookies for session
          headers: {
            "Content-Type": "application/json",
            "Accept": "application/json"
          },
          body: JSON.stringify(message)
        });
        if (!response.ok) {
          throw new Error(`HTTP request failed: ${response.status}`);
        }
        const updateResponse = await response.json();
        if (this.wrapperElement) {
          this.updateDOM(this.wrapperElement, updateResponse.tree, updateResponse.meta);
        }
      } catch (error) {
        console.error("Failed to send HTTP request:", error);
      }
    }
    /**
     * Parse a string value into appropriate type (number, boolean, or string)
     * @param value - String value to parse
     * @returns Parsed value with correct type
     */
    parseValue(value) {
      const num = parseFloat(value);
      if (!isNaN(num) && value.trim() === num.toString()) {
        return num;
      }
      if (value === "true") return true;
      if (value === "false") return false;
      return value;
    }
    /**
     * Apply an update to the current state and reconstruct HTML
     * @param update - Tree update object from LiveTemplate server
     * @returns Reconstructed HTML and whether anything changed
     */
    applyUpdate(update) {
      let changed = false;
      for (const [key, value] of Object.entries(update)) {
        const isDifferentialOps = Array.isArray(value) && value.length > 0 && Array.isArray(value[0]) && typeof value[0][0] === "string";
        if (isDifferentialOps) {
          this.treeState[key] = value;
          changed = true;
        } else {
          if (JSON.stringify(this.treeState[key]) !== JSON.stringify(value)) {
            this.treeState[key] = value;
            changed = true;
          }
        }
      }
      const html = this.reconstructFromTree(this.treeState);
      return { html, changed };
    }
    /**
     * Reconstruct HTML from a tree structure
     * This is the core algorithm that matches the Go server implementation
     * Dynamic values are interleaved between static segments: static[0] + dynamic[0] + static[1] + ...
     * Invariant: len(statics) == len(dynamics) + 1
     */
    reconstructFromTree(node) {
      if (node.s && Array.isArray(node.s)) {
        let html = "";
        for (let i = 0; i < node.s.length; i++) {
          const staticSegment = node.s[i];
          html += staticSegment;
          if (i < node.s.length - 1) {
            const dynamicKey = i.toString();
            if (node[dynamicKey] !== void 0) {
              html += this.renderValue(node[dynamicKey], dynamicKey);
            }
          }
        }
        html = html.replace(/<root>/g, "").replace(/<\/root>/g, "");
        return html;
      }
      return this.renderValue(node);
    }
    /**
     * Render a dynamic value (could be string, nested tree, or array)
     */
    renderValue(value, fieldKey) {
      if (value === null || value === void 0) {
        return "";
      }
      if (typeof value === "string" && value.startsWith("{{") && value.endsWith("}}")) {
        return "";
      }
      if (typeof value === "object" && !Array.isArray(value)) {
        if (value.d && Array.isArray(value.d) && value.s && Array.isArray(value.s)) {
          if (fieldKey) {
            this.rangeState[fieldKey] = {
              items: value.d,
              statics: value.s
            };
          }
          return this.renderRangeStructure(value);
        }
        if (value.s) {
          return this.reconstructFromTree(value);
        }
      }
      if (Array.isArray(value)) {
        if (value.length > 0 && Array.isArray(value[0]) && typeof value[0][0] === "string") {
          return this.applyDifferentialOperations(value, fieldKey);
        }
        return value.map((item) => {
          if (typeof item === "object" && item.s) {
            return this.reconstructFromTree(item);
          }
          return this.renderValue(item);
        }).join("");
      }
      return String(value);
    }
    /**
     * Render a range structure with 'd' (dynamics) and 's' (statics) arrays
     */
    renderRangeStructure(rangeNode) {
      const { d: dynamics, s: statics } = rangeNode;
      if (!dynamics || !Array.isArray(dynamics)) {
        return "";
      }
      if (dynamics.length === 0) {
        if (rangeNode["else"]) {
          return this.renderValue(rangeNode["else"]);
        }
        return "";
      }
      if (statics && Array.isArray(statics)) {
        return dynamics.map((item) => {
          let html = "";
          for (let i = 0; i < statics.length; i++) {
            html += statics[i];
            if (i < statics.length - 1) {
              const fieldKey = i.toString();
              if (item[fieldKey] !== void 0) {
                html += this.renderValue(item[fieldKey]);
              }
            }
          }
          return html;
        }).join("");
      }
      return dynamics.map((item) => this.renderValue(item)).join("");
    }
    /**
     * Find the position where the key attribute appears in statics array
     * Priority order: data-lvt-key, data-key, key, id (same as server-side)
     */
    findKeyPositionFromStatics(statics) {
      const keyAttrs = ['data-lvt-key="', 'data-key="', 'key="', 'id="'];
      for (let i = 0; i < statics.length; i++) {
        const staticStr = String(statics[i]);
        for (const keyAttr of keyAttrs) {
          if (staticStr.includes(keyAttr)) {
            return i;
          }
        }
      }
      return 0;
    }
    /**
     * Get item key from item data using statics to find correct position
     */
    getItemKey(item, statics) {
      const keyPos = this.findKeyPositionFromStatics(statics);
      const keyPosStr = keyPos.toString();
      return item[keyPosStr] || null;
    }
    /**
     * Apply differential operations to existing range items
     * Operations: ["r", key] for remove, ["u", key, changes] for update, ["a", items] for append
     */
    applyDifferentialOperations(operations, fieldKey) {
      if (!fieldKey || !this.rangeState[fieldKey]) {
        return "";
      }
      const rangeData = this.rangeState[fieldKey];
      const currentItems = [...rangeData.items];
      const statics = rangeData.statics;
      for (const operation of operations) {
        if (!Array.isArray(operation) || operation.length < 2) {
          continue;
        }
        const opType = operation[0];
        switch (opType) {
          case "r":
            const removeKey = operation[1];
            const removeIndex = currentItems.findIndex(
              (item) => this.getItemKey(item, statics) === removeKey
            );
            if (removeIndex >= 0) {
              currentItems.splice(removeIndex, 1);
            }
            break;
          case "u":
            const updateKey = operation[1];
            const changes = operation[2];
            const updateIndex = currentItems.findIndex(
              (item) => this.getItemKey(item, statics) === updateKey
            );
            if (updateIndex >= 0 && changes) {
              currentItems[updateIndex] = { ...currentItems[updateIndex], ...changes };
            }
            break;
          case "a":
            const itemsToAppend = operation[1];
            if (itemsToAppend) {
              if (Array.isArray(itemsToAppend)) {
                currentItems.push(...itemsToAppend);
              } else {
                currentItems.push(itemsToAppend);
              }
            }
            break;
          case "i":
            const targetKey = operation[1];
            const position = operation[2];
            const insertData = operation[3];
            if (insertData) {
              const itemsToInsert = Array.isArray(insertData) ? insertData : [insertData];
              if (targetKey === null) {
                if (position === "start") {
                  currentItems.unshift(...itemsToInsert);
                } else {
                  currentItems.push(...itemsToInsert);
                }
              } else {
                const targetIndex = currentItems.findIndex(
                  (item) => this.getItemKey(item, statics) === targetKey
                );
                if (targetIndex >= 0) {
                  const insertIndex = position === "before" ? targetIndex : targetIndex + 1;
                  currentItems.splice(insertIndex, 0, ...itemsToInsert);
                }
              }
            }
            break;
          case "o":
            const newOrder = operation[1];
            const reorderedItems = [];
            const itemsByKey = /* @__PURE__ */ new Map();
            for (const item of currentItems) {
              const itemKey = this.getItemKey(item, statics);
              if (itemKey) {
                itemsByKey.set(itemKey, item);
              }
            }
            for (const orderedKey of newOrder) {
              const item = itemsByKey.get(orderedKey);
              if (item) {
                reorderedItems.push(item);
              }
            }
            currentItems.length = 0;
            currentItems.push(...reorderedItems);
            break;
        }
      }
      this.rangeState[fieldKey] = {
        items: currentItems,
        statics
      };
      this.treeState[fieldKey] = {
        d: currentItems,
        s: statics
      };
      const rangeStructure = this.getCurrentRangeStructure(fieldKey);
      if (rangeStructure && rangeStructure.s) {
        return this.renderItemsWithStatics(currentItems, rangeStructure.s);
      }
      return currentItems.map((item) => this.renderValue(item)).join("");
    }
    /**
     * Get the current range structure for a field
     */
    getCurrentRangeStructure(fieldKey) {
      if (this.rangeState[fieldKey]) {
        return {
          d: this.rangeState[fieldKey].items,
          s: this.rangeState[fieldKey].statics
        };
      }
      const fieldValue = this.treeState[fieldKey];
      if (fieldValue && typeof fieldValue === "object" && fieldValue.s) {
        return fieldValue;
      }
      return null;
    }
    /**
     * Render items using static template
     */
    renderItemsWithStatics(items, statics) {
      const result = items.map((item) => {
        let html = "";
        for (let i = 0; i < statics.length; i++) {
          html += statics[i];
          if (i < statics.length - 1) {
            const fieldKey = i.toString();
            if (item[fieldKey] !== void 0) {
              html += this.renderValue(item[fieldKey]);
            }
          }
        }
        return html;
      }).join("");
      console.log("[renderItemsWithStatics] statics:", statics);
      console.log("[renderItemsWithStatics] items count:", items.length);
      console.log("[renderItemsWithStatics] result:", result.substring(0, 200));
      return result;
    }
    /**
     * Apply updates to existing HTML using morphdom for efficient DOM updates
     * @param existingHTML - Current full HTML document
     * @param update - Tree update object from LiveTemplate server
     * @returns Updated HTML content
     */
    applyUpdateToHTML(existingHTML, update) {
      const result = this.applyUpdate(update);
      if (!this.lvtId) {
        const match = existingHTML.match(/data-lvt-id="([^"]+)"/);
        if (match) {
          this.lvtId = match[1];
        }
      }
      const innerContent = result.html;
      const bodyMatch = existingHTML.match(/<body>([\s\S]*?)<\/body>/);
      if (!bodyMatch) {
        return existingHTML;
      }
      const wrapperStart = `<div data-lvt-id="${this.lvtId || "lvt-unknown"}">`;
      const wrapperEnd = "</div>";
      const newBodyContent = wrapperStart + innerContent + wrapperEnd;
      return existingHTML.replace(/<body>[\s\S]*?<\/body>/, `<body>${newBodyContent}</body>`);
    }
    /**
     * Update a live DOM element with new tree data
     * @param element - DOM element containing the LiveTemplate content (the wrapper div)
     * @param update - Tree update object from LiveTemplate server
     * @param meta - Optional metadata about the update (action, success, errors)
     */
    updateDOM(element, update, meta) {
      const result = this.applyUpdate(update);
      if (!result.changed && !update.s) {
        return;
      }
      const tempWrapper = document.createElement(element.tagName);
      console.log("[updateDOM] element.tagName:", element.tagName);
      console.log("[updateDOM] result.html (first 500 chars):", result.html.substring(0, 500));
      console.log("[updateDOM] Has <table> tag:", result.html.includes("<table>"));
      console.log("[updateDOM] Has <tbody> tag:", result.html.includes("<tbody>"));
      console.log("[updateDOM] Has <tr> tag:", result.html.includes("<tr"));
      tempWrapper.innerHTML = result.html;
      console.log("[updateDOM] tempWrapper.innerHTML after setting (first 500 chars):", tempWrapper.innerHTML.substring(0, 500));
      console.log("[updateDOM] tempWrapper has <table>:", tempWrapper.innerHTML.includes("<table>"));
      console.log("[updateDOM] tempWrapper has <tbody>:", tempWrapper.innerHTML.includes("<tbody>"));
      console.log("[updateDOM] tempWrapper has <tr>:", tempWrapper.innerHTML.includes("<tr"));
      morphdom_esm_default(element, tempWrapper, {
        childrenOnly: true,
        // Only update children, preserve the wrapper element itself
        getNodeKey: (node) => {
          if (node.nodeType === 1) {
            return node.getAttribute("data-key") || node.getAttribute("data-lvt-key") || void 0;
          }
        },
        onBeforeElUpdated: (fromEl, toEl) => {
          if (fromEl.isEqualNode(toEl)) {
            return false;
          }
          this.executeLifecycleHook(fromEl, "lvt-updated");
          return true;
        },
        onNodeAdded: (node) => {
          if (node.nodeType === Node.ELEMENT_NODE) {
            this.executeLifecycleHook(node, "lvt-mounted");
          }
        },
        onBeforeNodeDiscarded: (node) => {
          if (node.nodeType === Node.ELEMENT_NODE) {
            this.executeLifecycleHook(node, "lvt-destroyed");
          }
          return true;
        }
      });
      if (meta) {
        this.handleFormLifecycle(meta);
      }
    }
    /**
     * Handle form lifecycle after receiving server response
     * @param meta - Response metadata containing success status and errors
     */
    handleFormLifecycle(meta) {
      if (this.activeForm) {
        this.activeForm.dispatchEvent(new CustomEvent("lvt:done", { detail: meta }));
      }
      if (meta.success) {
        if (this.activeForm) {
          this.activeForm.dispatchEvent(new CustomEvent("lvt:success", { detail: meta }));
          if (!this.activeForm.hasAttribute("lvt-preserve")) {
            this.activeForm.reset();
          }
        }
      } else {
        if (this.activeForm) {
          this.activeForm.dispatchEvent(new CustomEvent("lvt:error", { detail: meta }));
        }
      }
      if (this.activeButton && this.originalButtonText !== null) {
        this.activeButton.disabled = false;
        this.activeButton.textContent = this.originalButtonText;
      }
      this.activeForm = null;
      this.activeButton = null;
      this.originalButtonText = null;
    }
    /**
     * Execute lifecycle hook on an element
     * @param element - Element with lifecycle hook attribute
     * @param hookName - Name of the lifecycle hook attribute (e.g., 'lvt-mounted')
     */
    executeLifecycleHook(element, hookName) {
      const hookValue = element.getAttribute(hookName);
      if (!hookValue) {
        return;
      }
      try {
        const hookFunction = new Function("element", hookValue);
        hookFunction.call(element, element);
      } catch (error) {
        console.error(`Error executing ${hookName} hook:`, error);
      }
    }
    /**
     * Reset client state (useful for testing)
     */
    reset() {
      this.treeState = {};
      this.rangeState = {};
      this.lvtId = null;
    }
    /**
     * Get current tree state (for debugging)
     */
    getTreeState() {
      return { ...this.treeState };
    }
    /**
     * Get the static structure if available
     */
    getStaticStructure() {
      return this.treeState.s || null;
    }
  };
  async function loadAndApplyUpdate(client, updatePath) {
    try {
      if (typeof __require !== "undefined") {
        const fs = __require("fs");
        const updateData2 = JSON.parse(fs.readFileSync(updatePath, "utf8"));
        return client.applyUpdate(updateData2);
      }
      const response = await fetch(updatePath);
      const updateData = await response.json();
      return client.applyUpdate(updateData);
    } catch (error) {
      throw new Error(`Failed to load update from ${updatePath}: ${error}`);
    }
  }
  function compareHTML(expected, actual) {
    const differences = [];
    const normalizeHTML = (html) => {
      return html.replace(/\s+/g, " ").replace(/>\s+</g, "><").trim();
    };
    const normalizedExpected = normalizeHTML(expected);
    const normalizedActual = normalizeHTML(actual);
    if (normalizedExpected === normalizedActual) {
      return { match: true, differences: [] };
    }
    const expectedLines = normalizedExpected.split("\n");
    const actualLines = normalizedActual.split("\n");
    const maxLines = Math.max(expectedLines.length, actualLines.length);
    for (let i = 0; i < maxLines; i++) {
      const expectedLine = expectedLines[i] || "";
      const actualLine = actualLines[i] || "";
      if (expectedLine !== actualLine) {
        differences.push(`Line ${i + 1}:`);
        differences.push(`  Expected: ${expectedLine}`);
        differences.push(`  Actual:   ${actualLine}`);
      }
    }
    return { match: false, differences };
  }
  function debounce(func, wait) {
    let timeout = null;
    return function(...args) {
      const context = this;
      if (timeout !== null) {
        clearTimeout(timeout);
      }
      timeout = window.setTimeout(() => {
        func.apply(context, args);
      }, wait);
    };
  }
  function throttle(func, limit) {
    let inThrottle = false;
    return function(...args) {
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
  if (typeof window !== "undefined") {
    LiveTemplateClient.autoInit();
  }
  return __toCommonJS(livetemplate_client_exports);
})();
//# sourceMappingURL=livetemplate-client.browser.js.map
