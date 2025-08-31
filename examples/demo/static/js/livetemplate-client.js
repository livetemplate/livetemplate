/**
 * LiveTemplate Client - Handles WebSocket/Ajax communication and fragment updates
 */
class LiveTemplateClient {
    constructor(options = {}) {
        this.pageToken = options.pageToken;
        this.wsUrl = options.wsUrl;
        this.ajaxUrl = options.ajaxUrl || '/api';
        this.debug = options.debug || false;
        
        // Connection state
        this.ws = null;
        this.isConnected = false;
        this.connectionMode = 'websocket'; // 'websocket' or 'ajax'
        
        // Fragment cache for static/dynamic separation
        this.fragmentCache = new Map();
        this.cacheInitialized = false;
        
        // Event callbacks
        this.onConnect = null;
        this.onDisconnect = null;
        this.onFragmentUpdate = null;
        
        // Connection status UI
        this.statusElement = null;
        
        this.log('LiveTemplate client initialized', { 
            pageToken: this.pageToken,
            wsUrl: this.wsUrl,
            ajaxUrl: this.ajaxUrl 
        });
        
        this.initStatusUI();
        this.connect();
    }
    
    log(...args) {
        if (this.debug) {
            console.log('[LiveTemplate Client]', ...args);
        }
    }
    
    initStatusUI() {
        // Create connection status indicator
        this.statusElement = document.createElement('div');
        this.statusElement.className = 'connection-status connecting';
        this.statusElement.textContent = 'Connecting...';
        document.body.appendChild(this.statusElement);
    }
    
    updateStatus(status, text) {
        if (this.statusElement) {
            this.statusElement.className = `connection-status ${status}`;
            this.statusElement.textContent = text;
            
            // Auto-hide after 3 seconds if connected
            if (status === 'connected' || status === 'ajax') {
                setTimeout(() => {
                    if (this.statusElement) {
                        this.statusElement.style.opacity = '0.3';
                    }
                }, 3000);
            }
        }
    }
    
    connect() {
        this.log('Attempting WebSocket connection...');
        this.updateStatus('connecting', 'Connecting...');
        
        try {
            this.ws = new WebSocket(this.wsUrl);
            
            this.ws.onopen = () => {
                this.log('WebSocket connected');
                this.isConnected = true;
                this.connectionMode = 'websocket';
                this.updateStatus('connected', 'WebSocket Connected');
                
                // Send connection message with page token
                this.sendWebSocketMessage({
                    action: 'connect',
                    token: this.pageToken
                });
                
                if (this.onConnect) {
                    this.onConnect('websocket');
                }
            };
            
            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleWebSocketMessage(message);
                } catch (err) {
                    this.log('Error parsing WebSocket message:', err);
                }
            };
            
            this.ws.onclose = () => {
                this.log('WebSocket disconnected, falling back to Ajax');
                this.isConnected = false;
                this.ws = null;
                
                // Fallback to Ajax mode
                this.fallbackToAjax();
            };
            
            this.ws.onerror = (error) => {
                this.log('WebSocket error:', error);
                this.isConnected = false;
                this.ws = null;
                
                // Fallback to Ajax mode
                this.fallbackToAjax();
            };
            
        } catch (err) {
            this.log('WebSocket connection failed:', err);
            this.fallbackToAjax();
        }
    }
    
    fallbackToAjax() {
        this.log('Using Ajax fallback mode');
        this.connectionMode = 'ajax';
        this.updateStatus('ajax', 'Ajax Mode');
        
        // Initialize fragments cache if needed
        if (!this.cacheInitialized) {
            this.initializeFragmentCache();
        }
        
        if (this.onConnect) {
            this.onConnect('ajax');
        }
    }
    
    async initializeFragmentCache() {
        if (this.cacheInitialized) {
            return;
        }
        
        this.log('Initializing fragment cache via Ajax...');
        
        try {
            const response = await fetch(`${this.ajaxUrl}/fragments`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Page-Token': this.pageToken,
                    'X-Cache-Empty': 'true'
                }
            });
            
            if (!response.ok) {
                throw new Error(`Ajax request failed: ${response.status}`);
            }
            
            const fragments = await response.json();
            this.handleFragments(fragments, true);
            this.cacheInitialized = true;
            
            this.log('Fragment cache initialized via Ajax', fragments.length, 'fragments');
            
        } catch (err) {
            this.log('Failed to initialize fragment cache:', err);
        }
    }
    
    handleWebSocketMessage(message) {
        this.log('WebSocket message received:', message.type);
        
        switch (message.type) {
            case 'initial_fragments':
                this.handleFragments(message.data, true);
                this.cacheInitialized = true;
                break;
                
            case 'fragments':
                this.handleFragments(message.data, false);
                break;
                
            case 'error':
                this.log('Server error:', message.data);
                break;
        }
    }
    
    handleFragments(fragments, isInitial) {
        if (!fragments || !Array.isArray(fragments)) {
            this.log('Invalid fragments received:', fragments);
            return;
        }
        
        this.log(`Processing ${fragments.length} fragments (initial: ${isInitial})`);
        
        fragments.forEach(fragment => {
            if (isInitial) {
                // Cache static/dynamic structure for first call
                this.fragmentCache.set(fragment.id, fragment);
                this.log(`Cached fragment ${fragment.id}:`, fragment);
            } else {
                // Update dynamic parts only for subsequent calls
                this.updateFragment(fragment);
            }
        });
        
        if (this.onFragmentUpdate) {
            this.onFragmentUpdate(fragments, isInitial);
        }
    }
    
    updateFragment(fragment) {
        this.log(`Updating fragment ${fragment.id}:`, fragment);
        
        // Find all elements with this fragment ID
        const elements = document.querySelectorAll(`[data-fragment-id="${fragment.id}"]`);
        
        if (elements.length === 0) {
            this.log(`No elements found for fragment ID: ${fragment.id}`);
            return;
        }
        
        elements.forEach(element => {
            try {
                this.applyFragmentUpdate(element, fragment);
            } catch (err) {
                this.log(`Error updating element for fragment ${fragment.id}:`, err);
            }
        });
    }
    
    applyFragmentUpdate(element, fragment) {
        // Handle different fragment strategies
        switch (fragment.strategy) {
            case 'tree_based':
            case 'static_dynamic':
                this.applyTreeBasedUpdate(element, fragment);
                break;
                
            case 'replacement':
                this.applyReplacementUpdate(element, fragment);
                break;
                
            default:
                this.log(`Unknown fragment strategy: ${fragment.strategy}`);
        }
    }
    
    applyTreeBasedUpdate(element, fragment) {
        const data = fragment.data;
        
        if (!data || typeof data !== 'object') {
            this.log('Invalid tree-based fragment data:', data);
            return;
        }
        
        // Get cached static structure
        const cachedFragment = this.fragmentCache.get(fragment.id);
        let staticParts = [];
        
        if (cachedFragment && cachedFragment.data && cachedFragment.data.s) {
            staticParts = cachedFragment.data.s;
        } else if (data.s) {
            staticParts = data.s;
        }
        
        // Reconstruct content with static parts and dynamic values
        let content = '';
        
        if (staticParts.length > 0) {
            // Interleave static parts with dynamic values
            for (let i = 0; i < staticParts.length; i++) {
                content += staticParts[i];
                
                // Add dynamic value if exists
                const dynamicKey = i.toString();
                if (data.hasOwnProperty(dynamicKey)) {
                    const dynamicValue = data[dynamicKey];
                    
                    if (Array.isArray(dynamicValue)) {
                        // Handle array of sub-fragments
                        dynamicValue.forEach(subItem => {
                            if (subItem && typeof subItem === 'object' && subItem.s) {
                                // Reconstruct sub-fragment
                                let subContent = '';
                                for (let j = 0; j < subItem.s.length; j++) {
                                    subContent += subItem.s[j];
                                    const subKey = j.toString();
                                    if (subItem.hasOwnProperty(subKey)) {
                                        subContent += this.formatDynamicValue(subItem[subKey]);
                                    }
                                }
                                content += subContent;
                            } else {
                                content += this.formatDynamicValue(subItem);
                            }
                        });
                    } else if (dynamicValue && typeof dynamicValue === 'object' && dynamicValue.s) {
                        // Handle nested fragment structure
                        let nestedContent = '';
                        for (let j = 0; j < dynamicValue.s.length; j++) {
                            nestedContent += dynamicValue.s[j];
                            const nestedKey = j.toString();
                            if (dynamicValue.hasOwnProperty(nestedKey)) {
                                nestedContent += this.formatDynamicValue(dynamicValue[nestedKey]);
                            }
                        }
                        content += nestedContent;
                    } else {
                        content += this.formatDynamicValue(dynamicValue);
                    }
                }
            }
        } else {
            // Simple dynamic value
            content = this.formatDynamicValue(data);
        }
        
        // Update element content
        element.innerHTML = content;
        
        this.log(`Applied tree-based update to element:`, element, 'Content:', content);
    }
    
    applyReplacementUpdate(element, fragment) {
        if (fragment.data && typeof fragment.data === 'string') {
            element.innerHTML = fragment.data;
            this.log(`Applied replacement update to element:`, element);
        }
    }
    
    formatDynamicValue(value) {
        if (value === null || value === undefined) {
            return '';
        }
        if (typeof value === 'object') {
            return JSON.stringify(value);
        }
        return String(value);
    }
    
    sendWebSocketMessage(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
            this.log('Sent WebSocket message:', message);
        } else {
            this.log('Cannot send WebSocket message: connection not open');
        }
    }
    
    async sendAction(action, payload = {}) {
        const message = {
            action: action,
            payload: payload,
            token: this.pageToken
        };
        
        if (this.connectionMode === 'websocket' && this.isConnected) {
            // Send via WebSocket
            this.sendWebSocketMessage(message);
        } else {
            // Send via Ajax
            try {
                this.log('Sending Ajax action:', action);
                
                const response = await fetch(`${this.ajaxUrl}/action`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(message)
                });
                
                if (!response.ok) {
                    throw new Error(`Ajax action failed: ${response.status}`);
                }
                
                const fragments = await response.json();
                this.handleFragments(fragments, false);
                
                this.log('Ajax action completed:', action, fragments.length, 'fragments');
                
            } catch (err) {
                this.log('Ajax action failed:', err);
            }
        }
    }
    
    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.isConnected = false;
        this.updateStatus('disconnected', 'Disconnected');
        
        if (this.onDisconnect) {
            this.onDisconnect();
        }
    }
}