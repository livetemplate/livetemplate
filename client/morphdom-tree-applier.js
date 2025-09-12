/**
 * Morphdom Tree Applier - Hybrid approach client-side implementation
 * 
 * This client applies tree updates using morphdom for efficient DOM updating.
 * Server sends full HTML (generated via DOM diffing), client applies with morphdom.
 * 
 * Benefits:
 * - Server: No HTML intrinsics knowledge required  
 * - Client: No DOM operation knowledge required
 * - Uses proven morphdom library for efficient updates
 * - Compatible with any HTML structure (future-proof)
 */

class MorphdomTreeApplier {
    constructor(options = {}) {
        this.debug = options.debug || false;
        this.morphdomOptions = {
            // morphdom options - can be customized
            childrenOnly: false,
            getNodeKey: options.getNodeKey || null,
            onBeforeNodeAdded: options.onBeforeNodeAdded || null,
            onNodeAdded: options.onNodeAdded || null,
            onBeforeElUpdated: options.onBeforeElUpdated || null,
            onElUpdated: options.onElUpdated || null,
            onBeforeNodeDiscarded: options.onBeforeNodeDiscarded || null,
            onNodeDiscarded: options.onNodeDiscarded || null,
            onBeforeElChildrenUpdated: options.onBeforeElChildrenUpdated || null
        };
        
        this.containerElement = options.container || document.body;
        this.loadMorphdom();
    }

    /**
     * Load morphdom library if not already available
     */
    loadMorphdom() {
        if (typeof morphdom !== 'undefined') {
            if (this.debug) {
                console.log('morphdom already available');
            }
            return;
        }

        // For demo purposes, we'll create a simple morphdom-like implementation
        // In production, you'd load the actual morphdom library
        this.createSimpleMorphdom();
    }

    /**
     * Simple morphdom-like implementation for demo
     * In production, use the real morphdom library: https://github.com/patrick-steele-idem/morphdom
     */
    createSimpleMorphdom() {
        window.morphdom = (targetElement, newHTML, options = {}) => {
            if (this.debug) {
                console.log('Applying morphdom update:', {
                    target: targetElement,
                    newHTML: newHTML.substring(0, 100) + '...',
                    options
                });
            }

            // Simple implementation: replace innerHTML
            // Real morphdom would do sophisticated DOM diffing and minimal updates
            if (typeof newHTML === 'string') {
                // Create temporary element to parse HTML
                const temp = document.createElement('div');
                temp.innerHTML = newHTML;
                
                // If single element, replace with that element
                if (temp.children.length === 1) {
                    const newElement = temp.firstElementChild;
                    if (targetElement.parentNode) {
                        targetElement.parentNode.replaceChild(newElement, targetElement);
                        return newElement;
                    }
                } else {
                    // Multiple elements or text content
                    targetElement.innerHTML = newHTML;
                    return targetElement;
                }
            }
            
            return targetElement;
        };

        if (this.debug) {
            console.log('Simple morphdom implementation created');
        }
    }

    /**
     * Apply a tree update received from the server
     * @param {Object} update - The TreeUpdate object
     */
    applyTreeUpdate(update) {
        if (!update || !update.fragment_id) {
            console.warn('Invalid tree update received:', update);
            return;
        }

        if (this.debug) {
            console.log(`Applying tree update for fragment: ${update.fragment_id}`, update);
        }

        // Handle empty updates (no changes)
        if (this.isEmptyUpdate(update)) {
            if (this.debug) {
                console.log('Empty update - no changes needed');
            }
            return;
        }

        // Apply the update using morphdom
        if (update.html) {
            this.applyHTMLUpdate(update);
        } else if (update.statics && update.dynamics) {
            // If we have static/dynamic breakdown, we could optimize further
            // For now, fall back to HTML update
            console.warn('Static/dynamic optimization not implemented, using HTML fallback');
            if (update.html) {
                this.applyHTMLUpdate(update);
            }
        }
    }

    /**
     * Check if update is empty (no changes needed)
     */
    isEmptyUpdate(update) {
        return !update.html && 
               (!update.statics || update.statics.length === 0) && 
               (!update.dynamics || Object.keys(update.dynamics).length === 0);
    }

    /**
     * Apply HTML update using morphdom
     */
    applyHTMLUpdate(update) {
        try {
            const targetElement = this.findTargetElement(update.fragment_id);
            if (!targetElement) {
                console.error(`Target element not found for fragment: ${update.fragment_id}`);
                return;
            }

            if (this.debug) {
                console.log('Applying HTML update to:', targetElement);
                console.log('New HTML:', update.html.substring(0, 200) + '...');
            }

            // Use morphdom to efficiently update the DOM
            const result = morphdom(targetElement, update.html, this.morphdomOptions);
            
            if (this.debug) {
                console.log('morphdom update completed:', result);
            }

            // Trigger custom event for application to respond to updates
            this.dispatchUpdateEvent(update.fragment_id, 'html_update');

        } catch (error) {
            console.error('Failed to apply HTML update:', error, update);
        }
    }

    /**
     * Find the target element for a fragment ID
     * This could be enhanced with more sophisticated targeting
     */
    findTargetElement(fragmentId) {
        // Look for element with data-fragment-id attribute
        let element = document.querySelector(`[data-fragment-id="${fragmentId}"]`);
        
        if (!element) {
            // Fallback: look for element with id
            element = document.getElementById(fragmentId);
        }
        
        if (!element) {
            // Final fallback: use container element for first update
            element = this.containerElement;
        }

        return element;
    }

    /**
     * Dispatch custom event when update is applied
     */
    dispatchUpdateEvent(fragmentId, updateType) {
        const event = new CustomEvent('livetemplate:update', {
            detail: {
                fragmentId,
                updateType,
                timestamp: Date.now()
            }
        });
        
        document.dispatchEvent(event);
        
        if (this.debug) {
            console.log('Dispatched update event:', event.detail);
        }
    }

    /**
     * Enable debug logging
     */
    enableDebug() {
        this.debug = true;
        console.log('Morphdom Tree Applier debug enabled');
    }

    /**
     * Disable debug logging
     */
    disableDebug() {
        this.debug = false;
    }

    /**
     * Set container element for fallback targeting
     */
    setContainer(element) {
        this.containerElement = element;
        if (this.debug) {
            console.log('Container element set:', element);
        }
    }

    /**
     * Update morphdom options
     */
    updateMorphdomOptions(options) {
        this.morphdomOptions = { ...this.morphdomOptions, ...options };
        if (this.debug) {
            console.log('Updated morphdom options:', this.morphdomOptions);
        }
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = MorphdomTreeApplier;
}

// Global access
window.MorphdomTreeApplier = MorphdomTreeApplier;

// Auto-initialize if LiveTemplate is present
if (window.LiveTemplate) {
    window.LiveTemplate.morphdomTreeApplier = new MorphdomTreeApplier({ debug: false });
    console.log('Morphdom Tree Applier initialized with LiveTemplate');
}