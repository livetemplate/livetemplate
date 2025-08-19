/**
 * LiveTemplate Client-Side Fragment Application Engine
 * 
 * This module provides comprehensive client-side JavaScript for applying 
 * all four LiveTemplate fragment strategies in the browser.
 * 
 * Supports:
 * - Static/Dynamic fragments (Strategy 1)
 * - Marker fragments (Strategy 2) 
 * - Granular operations (Strategy 3)
 * - Fragment replacement (Strategy 4)
 * - Client-side caching
 * - Error handling and recovery
 */

class LiveTemplateClient {
    constructor(options = {}) {
        this.options = {
            debug: options.debug || false,
            cacheEnabled: options.cacheEnabled !== false,
            errorCallback: options.errorCallback || this.defaultErrorHandler,
            maxCacheSize: options.maxCacheSize || 100,
            enableMetrics: options.enableMetrics || false,
            ...options
        };

        // Fragment cache for static data
        this.fragmentCache = new Map();
        
        // Metrics collection
        this.metrics = {
            fragmentsApplied: 0,
            cacheHits: 0,
            cacheMisses: 0,
            errorCount: 0,
            strategyUsage: {
                static_dynamic: 0,
                markers: 0,
                granular: 0,
                replacement: 0
            }
        };

        this.log('LiveTemplate Client initialized', this.options);
    }

    /**
     * Main entry point for applying fragments
     * @param {Object|Array} fragments - Single fragment or array of fragments
     * @returns {Promise<boolean>} Success status
     */
    async applyFragments(fragments) {
        try {
            const fragmentArray = Array.isArray(fragments) ? fragments : [fragments];
            
            for (const fragment of fragmentArray) {
                await this.applyFragment(fragment);
            }
            
            return true;
        } catch (error) {
            this.handleError('Failed to apply fragments', error, fragments);
            return false;
        }
    }

    /**
     * Apply a single fragment based on its strategy
     * @param {Object} fragment - Fragment to apply
     * @returns {Promise<boolean>} Success status
     */
    async applyFragment(fragment) {
        if (!this.validateFragment(fragment)) {
            return false;
        }

        try {
            this.log('Applying fragment:', fragment.strategy, fragment.id);
            
            // Update metrics
            this.metrics.fragmentsApplied++;
            this.metrics.strategyUsage[fragment.strategy]++;

            // Route to appropriate strategy handler
            switch (fragment.strategy) {
                case 'static_dynamic':
                    return await this.applyStaticDynamicFragment(fragment);
                case 'markers':
                    return await this.applyMarkerFragment(fragment);
                case 'granular':
                    return await this.applyGranularFragment(fragment);
                case 'replacement':
                    return await this.applyReplacementFragment(fragment);
                default:
                    throw new Error(`Unknown fragment strategy: ${fragment.strategy}`);
            }
        } catch (error) {
            this.handleError(`Failed to apply ${fragment.strategy} fragment`, error, fragment);
            return false;
        }
    }

    /**
     * Apply Static/Dynamic fragment (Strategy 1)
     * Handles both full fragments with statics and dynamics-only updates
     */
    async applyStaticDynamicFragment(fragment) {
        const { data, action, id } = fragment;
        
        switch (action) {
            case 'update_values':
                return this.applyStaticDynamicValueUpdate(data, id);
            case 'update_conditional':
                return this.applyStaticDynamicConditional(data, id);
            default:
                throw new Error(`Unknown static/dynamic action: ${action}`);
        }
    }

    /**
     * Apply static/dynamic value updates
     */
    applyStaticDynamicValueUpdate(data, fragmentId) {
        try {
            // Check if we have cached statics for this fragment
            const cachedData = this.getCachedStaticData(fragmentId);
            
            if (data.statics && data.statics.length > 0) {
                // Full fragment with statics - cache them
                this.cacheStaticData(fragmentId, {
                    statics: data.statics,
                    fragmentId: fragmentId
                });
                this.metrics.cacheMisses++;
                
                // Apply full reconstruction
                return this.reconstructStaticDynamicContent(data, fragmentId);
            } else if (cachedData && data.dynamics) {
                // Dynamics-only update using cached statics
                this.metrics.cacheHits++;
                this.log('Using cached statics for dynamics-only update', fragmentId);
                
                // Merge cached statics with new dynamics
                const fullData = {
                    statics: cachedData.statics,
                    dynamics: data.dynamics,
                    conditionals: data.conditionals
                };
                
                return this.reconstructStaticDynamicContent(fullData, fragmentId);
            } else {
                throw new Error(`No statics available for fragment ${fragmentId} and no cached data`);
            }
        } catch (error) {
            this.handleError('Static/dynamic value update failed', error, data);
            return false;
        }
    }

    /**
     * Reconstruct content from static/dynamic data
     */
    reconstructStaticDynamicContent(data, fragmentId) {
        try {
            const { statics, dynamics, conditionals } = data;
            
            // Find target element (assume fragment ID maps to element ID)
            const targetId = fragmentId.replace('frag_static_dynamic_', '').replace('frag_', '');
            let target = document.getElementById(targetId);
            
            // If direct ID lookup fails, try to find by fragment data attributes
            if (!target) {
                target = document.querySelector(`[data-fragment-id="${fragmentId}"]`);
            }
            
            // If still not found, look for any element with data-fragment attribute
            if (!target) {
                target = document.querySelector('[data-fragment]');
            }
            
            if (!target) {
                this.log('Warning: Target element not found for fragment', fragmentId, 'using document.body');
                target = document.body;
            }

            // Reconstruct HTML from statics and dynamics
            let reconstructedHTML = '';
            
            for (let i = 0; i < statics.length; i++) {
                reconstructedHTML += statics[i];
                
                // Add dynamic content at this position
                if (dynamics && dynamics[i] !== undefined) {
                    reconstructedHTML += dynamics[i];
                }
                
                // Handle conditional content
                if (conditionals && conditionals[i]) {
                    const conditional = conditionals[i];
                    const conditionMet = this.evaluateConditional(conditional);
                    
                    if (conditionMet && conditional.truthy_value) {
                        reconstructedHTML += conditional.truthy_value;
                    } else if (!conditionMet && conditional.falsy_value) {
                        reconstructedHTML += conditional.falsy_value;
                    }
                }
            }

            // Apply the reconstructed content
            if (reconstructedHTML.trim()) {
                target.innerHTML = reconstructedHTML;
                this.log('Static/dynamic content reconstructed successfully', fragmentId);
                return true;
            } else {
                this.log('Warning: Reconstructed content is empty', fragmentId);
                return false;
            }
            
        } catch (error) {
            this.handleError('Content reconstruction failed', error, data);
            return false;
        }
    }

    /**
     * Apply static/dynamic conditional updates
     */
    applyStaticDynamicConditional(data, fragmentId) {
        try {
            if (!data.conditionals) {
                return false;
            }

            for (const conditional of data.conditionals) {
                const elementId = conditional.element_id || `conditional-${fragmentId}-${conditional.position}`;
                const element = document.getElementById(elementId);
                
                if (!element) {
                    this.log('Warning: Conditional element not found', elementId);
                    continue;
                }

                const conditionMet = this.evaluateConditional(conditional);
                
                if (conditional.is_full_element) {
                    // Control entire element visibility
                    element.style.display = conditionMet ? 'block' : 'none';
                }
                
                if (conditionMet && conditional.truthy_value) {
                    element.textContent = conditional.truthy_value;
                } else if (!conditionMet && conditional.falsy_value) {
                    element.textContent = conditional.falsy_value;
                }
            }
            
            return true;
        } catch (error) {
            this.handleError('Conditional update failed', error, data);
            return false;
        }
    }

    /**
     * Apply Marker fragment (Strategy 2)
     * Updates elements with data-marker attributes
     */
    async applyMarkerFragment(fragment) {
        const { data, action } = fragment;
        
        if (action !== 'apply_patches') {
            throw new Error(`Unknown marker action: ${action}`);
        }

        try {
            const { value_updates, position_map } = data;
            
            if (!value_updates) {
                return false;
            }

            for (const [markerId, value] of Object.entries(value_updates)) {
                // Find element by data-marker attribute
                const marker = document.querySelector(`[data-marker="${markerId}"]`);
                
                if (marker) {
                    // Apply value update
                    if (marker.tagName === 'INPUT' || marker.tagName === 'TEXTAREA') {
                        marker.value = value;
                    } else {
                        marker.textContent = value;
                    }
                    
                    this.log('Marker updated:', markerId, '→', value);
                } else {
                    this.log('Warning: Marker element not found:', markerId);
                }
            }
            
            return true;
        } catch (error) {
            this.handleError('Marker fragment application failed', error, data);
            return false;
        }
    }

    /**
     * Apply Granular fragment (Strategy 3)
     * Executes precise DOM operations
     */
    async applyGranularFragment(fragment) {
        const { data, action } = fragment;
        
        if (action !== 'apply_operations') {
            throw new Error(`Unknown granular action: ${action}`);
        }

        try {
            const { operations } = data;
            
            if (!operations || !Array.isArray(operations)) {
                return false;
            }

            for (const operation of operations) {
                await this.executeGranularOperation(operation);
            }
            
            return true;
        } catch (error) {
            this.handleError('Granular fragment application failed', error, data);
            return false;
        }
    }

    /**
     * Execute a single granular operation
     */
    async executeGranularOperation(operation) {
        const { type, target_id, content, position, selector } = operation;
        
        const target = document.getElementById(target_id);
        if (!target) {
            this.log('Warning: Granular operation target not found:', target_id);
            return false;
        }

        switch (type) {
            case 'insert':
                return this.executeInsertOperation(target, content, position);
            case 'remove':
                return this.executeRemoveOperation(target, selector);
            case 'update':
                return this.executeUpdateOperation(target, content);
            case 'replace':
                return this.executeReplaceOperation(target, content);
            default:
                throw new Error(`Unknown granular operation type: ${type}`);
        }
    }

    /**
     * Execute insert operation
     */
    executeInsertOperation(target, content, position = 'beforeend') {
        try {
            target.insertAdjacentHTML(position, content);
            this.log('Insert operation completed', target.id, position);
            return true;
        } catch (error) {
            this.handleError('Insert operation failed', error, { target, content, position });
            return false;
        }
    }

    /**
     * Execute remove operation
     */
    executeRemoveOperation(target, selector) {
        try {
            if (selector) {
                const elements = target.querySelectorAll(selector);
                elements.forEach(el => el.remove());
                this.log('Remove operation completed', target.id, selector, `${elements.length} elements`);
            } else {
                target.remove();
                this.log('Remove operation completed - entire target removed', target.id);
            }
            return true;
        } catch (error) {
            this.handleError('Remove operation failed', error, { target, selector });
            return false;
        }
    }

    /**
     * Execute update operation
     */
    executeUpdateOperation(target, content) {
        try {
            target.innerHTML = content;
            this.log('Update operation completed', target.id);
            return true;
        } catch (error) {
            this.handleError('Update operation failed', error, { target, content });
            return false;
        }
    }

    /**
     * Execute replace operation
     */
    executeReplaceOperation(target, content) {
        try {
            target.outerHTML = content;
            this.log('Replace operation completed', target.id);
            return true;
        } catch (error) {
            this.handleError('Replace operation failed', error, { target, content });
            return false;
        }
    }

    /**
     * Apply Replacement fragment (Strategy 4)
     * Complete content replacement
     */
    async applyReplacementFragment(fragment) {
        const { data, action, id } = fragment;
        
        if (action !== 'replace_content') {
            throw new Error(`Unknown replacement action: ${action}`);
        }

        try {
            const { content, target_id } = data;
            
            // Determine target element
            const targetId = target_id || id.replace('frag_replacement_', '').replace('frag_', '');
            let target = document.getElementById(targetId);
            
            // Fallback to fragment-specific selector
            if (!target) {
                target = document.querySelector(`[data-fragment-id="${id}"]`);
            }
            
            if (!target) {
                this.log('Warning: Replacement target not found, using body', targetId);
                target = document.body;
            }

            if (data.is_empty) {
                // Empty state - remove content
                target.innerHTML = '';
                this.log('Replacement fragment applied - content cleared', targetId);
            } else {
                // Replace with new content
                target.outerHTML = content;
                this.log('Replacement fragment applied', targetId);
            }
            
            return true;
        } catch (error) {
            this.handleError('Replacement fragment application failed', error, data);
            return false;
        }
    }

    /**
     * Cache static data for future use
     */
    cacheStaticData(fragmentId, staticData) {
        if (!this.options.cacheEnabled) {
            return;
        }

        // Implement LRU cache with size limit
        if (this.fragmentCache.size >= this.options.maxCacheSize) {
            const firstKey = this.fragmentCache.keys().next().value;
            this.fragmentCache.delete(firstKey);
        }

        this.fragmentCache.set(fragmentId, {
            ...staticData,
            cachedAt: Date.now()
        });
        
        this.log('Static data cached', fragmentId);
    }

    /**
     * Retrieve cached static data
     */
    getCachedStaticData(fragmentId) {
        if (!this.options.cacheEnabled) {
            return null;
        }

        const cached = this.fragmentCache.get(fragmentId);
        if (cached) {
            // Move to end (LRU)
            this.fragmentCache.delete(fragmentId);
            this.fragmentCache.set(fragmentId, cached);
            return cached;
        }
        
        return null;
    }

    /**
     * Clear fragment cache
     */
    clearCache() {
        this.fragmentCache.clear();
        this.log('Fragment cache cleared');
    }

    /**
     * Evaluate conditional logic
     */
    evaluateConditional(conditional) {
        switch (conditional.condition_type) {
            case 'boolean':
                return conditional.condition === true || conditional.condition === 'true';
            case 'nil-notnil':
                return conditional.condition != null && conditional.condition !== undefined;
            case 'show-hide':
                return conditional.condition !== false && conditional.condition !== 'false';
            default:
                return Boolean(conditional.condition);
        }
    }

    /**
     * Validate fragment structure
     */
    validateFragment(fragment) {
        if (!fragment || typeof fragment !== 'object') {
            this.handleError('Invalid fragment: not an object', null, fragment);
            return false;
        }

        const required = ['id', 'strategy', 'action', 'data'];
        for (const field of required) {
            if (!fragment[field]) {
                this.handleError(`Invalid fragment: missing ${field}`, null, fragment);
                return false;
            }
        }

        const validStrategies = ['static_dynamic', 'markers', 'granular', 'replacement'];
        if (!validStrategies.includes(fragment.strategy)) {
            this.handleError(`Invalid fragment: unknown strategy ${fragment.strategy}`, null, fragment);
            return false;
        }

        return true;
    }

    /**
     * Error handling
     */
    handleError(message, error, context) {
        this.metrics.errorCount++;
        
        const errorInfo = {
            message,
            error: error?.message || error,
            context,
            timestamp: new Date().toISOString()
        };

        console.error('LiveTemplate Client Error:', errorInfo);
        
        if (this.options.errorCallback) {
            try {
                this.options.errorCallback(errorInfo);
            } catch (callbackError) {
                console.error('Error callback failed:', callbackError);
            }
        }
    }

    /**
     * Default error handler
     */
    defaultErrorHandler(errorInfo) {
        // Default: just log to console (already done in handleError)
    }

    /**
     * Debug logging
     */
    log(...args) {
        if (this.options.debug) {
            console.log('[LiveTemplate Client]', ...args);
        }
    }

    /**
     * Get performance metrics
     */
    getMetrics() {
        return {
            ...this.metrics,
            cacheSize: this.fragmentCache.size,
            cacheHitRate: this.metrics.cacheHits / (this.metrics.cacheHits + this.metrics.cacheMisses) || 0
        };
    }

    /**
     * Reset metrics
     */
    resetMetrics() {
        this.metrics = {
            fragmentsApplied: 0,
            cacheHits: 0,
            cacheMisses: 0,
            errorCount: 0,
            strategyUsage: {
                static_dynamic: 0,
                markers: 0,
                granular: 0,
                replacement: 0
            }
        };
    }
}

// Export for both Node.js and browser environments
if (typeof module !== 'undefined' && module.exports) {
    module.exports = LiveTemplateClient;
} else if (typeof window !== 'undefined') {
    window.LiveTemplateClient = LiveTemplateClient;
}