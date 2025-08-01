/**
 * Error thrown when an update operation fails
 */
class UpdateError extends Error {
    constructor(message, fragmentId, action, cause) {
        super(message);
        this.fragmentId = fragmentId;
        this.action = action;
        this.cause = cause;
        this.name = 'UpdateError';
    }
}

// @ts-ignore - morphdom doesn't have proper type definitions
const morphdom = require('morphdom');
/**
 * StateTemplate client for applying real-time HTML updates using morphdom
 */
class StateTemplateClient {
    constructor(config = {}) {
        this.config = {
            debug: false,
            morphOptions: {},
            ...config
        };
    }
    /**
     * Apply a real-time update to the DOM
     * @param update - The update object from StateTemplate server
     * @returns Promise resolving to update result
     */
    async applyUpdate(update) {
        try {
            this.validateUpdate(update);
            this.debugLog(`Applying update for fragment: ${update.fragment_id}`, update);
            const element = this.findElement(update.fragment_id);
            if (!element) {
                throw new UpdateError(`Element with ID '${update.fragment_id}' not found`, update.fragment_id, update.action);
            }
            switch (update.action.toLowerCase()) {
                case 'replace':
                    return this.replaceElement(element, update);
                case 'append':
                    return this.appendToElement(element, update);
                case 'prepend':
                    return this.prependToElement(element, update);
                case 'remove':
                    return this.removeElement(element, update);
                default:
                    throw new UpdateError(`Unsupported action: ${update.action}`, update.fragment_id, update.action);
            }
        }
        catch (error) {
            const updateError = error instanceof UpdateError
                ? error
                : new UpdateError(`Failed to apply update: ${error instanceof Error ? error.message : String(error)}`, update.fragment_id, update.action, error instanceof Error ? error : undefined);
            this.debugLog(`Update failed:`, updateError);
            return {
                success: false,
                fragmentId: update.fragment_id,
                action: update.action,
                error: updateError
            };
        }
    }
    /**
     * Apply multiple updates in sequence
     * @param updates - Array of update objects
     * @returns Promise resolving to array of update results
     */
    async applyUpdates(updates) {
        const results = [];
        for (const update of updates) {
            const result = await this.applyUpdate(update);
            results.push(result);
            // Stop processing if an update fails and debug mode is off
            if (!result.success && !this.config.debug) {
                this.debugLog(`Stopping batch updates due to failure in fragment: ${update.fragment_id}`);
                break;
            }
        }
        return results;
    }
    /**
     * Set initial HTML content for the page
     * @param html - Initial HTML content
     * @param containerId - ID of container element (default: 'app')
     */
    setInitialContent(html, containerId = 'app') {
        const container = document.getElementById(containerId);
        if (!container) {
            throw new Error(`Container element with ID '${containerId}' not found`);
        }
        container.innerHTML = html;
        this.debugLog(`Set initial content for container: ${containerId}`);
    }
    /**
     * Check if an element exists in the DOM
     * @param fragmentId - Fragment ID to check
     * @returns boolean indicating if element exists
     */
    hasElement(fragmentId) {
        return this.findElement(fragmentId) !== null;
    }
    validateUpdate(update) {
        if (!update.fragment_id) {
            throw new UpdateError('fragment_id is required', '', update.action || 'unknown');
        }
        if (!update.action) {
            throw new UpdateError('action is required', update.fragment_id, '');
        }
        // HTML is required for most actions except 'remove'
        if (update.action.toLowerCase() !== 'remove' && !update.html) {
            throw new UpdateError('html is required for this action', update.fragment_id, update.action);
        }
    }
    findElement(fragmentId) {
        return document.getElementById(fragmentId) || document.querySelector(`[data-fragment-id="${fragmentId}"]`);
    }
    replaceElement(element, update) {
        try {
            // Create a temporary container to parse the new HTML
            const tempContainer = document.createElement('div');
            tempContainer.innerHTML = update.html;
            const newElement = tempContainer.firstElementChild;
            if (!newElement) {
                throw new Error('Invalid HTML: no element found');
            }
            // Use morphdom to efficiently update the existing element
            const morphedElement = morphdom(element, newElement, {
                ...this.config.morphOptions,
                onBeforeElUpdated: (fromEl, toEl) => {
                    // Preserve the fragment ID
                    if (fromEl.id === update.fragment_id) {
                        toEl.id = update.fragment_id;
                    }
                    // Call user's onBeforeElUpdated if provided
                    if (this.config.morphOptions.onBeforeElUpdated) {
                        return this.config.morphOptions.onBeforeElUpdated(fromEl, toEl);
                    }
                    return true;
                }
            });
            this.debugLog(`Successfully replaced element: ${update.fragment_id}`);
            return {
                success: true,
                fragmentId: update.fragment_id,
                action: update.action,
                element: morphedElement
            };
        }
        catch (error) {
            throw new UpdateError(`Failed to replace element: ${error instanceof Error ? error.message : String(error)}`, update.fragment_id, update.action, error instanceof Error ? error : undefined);
        }
    }
    appendToElement(element, update) {
        try {
            const tempContainer = document.createElement('div');
            tempContainer.innerHTML = update.html;
            // Append all child nodes from the temporary container
            while (tempContainer.firstChild) {
                element.appendChild(tempContainer.firstChild);
            }
            this.debugLog(`Successfully appended to element: ${update.fragment_id}`);
            return {
                success: true,
                fragmentId: update.fragment_id,
                action: update.action,
                element
            };
        }
        catch (error) {
            throw new UpdateError(`Failed to append to element: ${error instanceof Error ? error.message : String(error)}`, update.fragment_id, update.action, error instanceof Error ? error : undefined);
        }
    }
    prependToElement(element, update) {
        try {
            const tempContainer = document.createElement('div');
            tempContainer.innerHTML = update.html;
            // Prepend all child nodes from the temporary container
            const firstChild = element.firstChild;
            while (tempContainer.lastChild) {
                element.insertBefore(tempContainer.lastChild, firstChild);
            }
            this.debugLog(`Successfully prepended to element: ${update.fragment_id}`);
            return {
                success: true,
                fragmentId: update.fragment_id,
                action: update.action,
                element
            };
        }
        catch (error) {
            throw new UpdateError(`Failed to prepend to element: ${error instanceof Error ? error.message : String(error)}`, update.fragment_id, update.action, error instanceof Error ? error : undefined);
        }
    }
    removeElement(element, update) {
        try {
            const parent = element.parentNode;
            if (parent) {
                parent.removeChild(element);
            }
            this.debugLog(`Successfully removed element: ${update.fragment_id}`);
            return {
                success: true,
                fragmentId: update.fragment_id,
                action: update.action
            };
        }
        catch (error) {
            throw new UpdateError(`Failed to remove element: ${error instanceof Error ? error.message : String(error)}`, update.fragment_id, update.action, error instanceof Error ? error : undefined);
        }
    }
    debugLog(message, data) {
        if (this.config.debug) {
            console.log(`[StateTemplateClient] ${message}`, data || '');
        }
    }
}

// Global client instance for convenience functions
let globalClient = null;
/**
 * Create a new StateTemplate client instance
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
function createClient(config) {
    return new StateTemplateClient(config);
}
/**
 * Create and set a global client instance for convenience functions
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
function initializeGlobalClient(config) {
    globalClient = createClient(config);
    return globalClient;
}
/**
 * Apply a single update using the global client instance
 * @param update - Update object to apply
 * @throws Error if global client is not initialized
 */
async function applyUpdate(update) {
    if (!globalClient) {
        throw new Error('Global client not initialized. Call initializeGlobalClient() first or use StateTemplateClient directly.');
    }
    const result = await globalClient.applyUpdate(update);
    if (!result.success) {
        throw result.error || new Error(`Update failed for fragment: ${update.fragment_id}`);
    }
}
/**
 * Set initial content using the global client instance
 * @param html - Initial HTML content
 * @param containerId - ID of container element
 * @throws Error if global client is not initialized
 */
function setInitialContent(html, containerId) {
    if (!globalClient) {
        throw new Error('Global client not initialized. Call initializeGlobalClient() first or use StateTemplateClient directly.');
    }
    globalClient.setInitialContent(html, containerId);
}
/**
 * Get the global client instance
 * @returns StateTemplateClient instance or null if not initialized
 */
function getGlobalClient() {
    return globalClient;
}
/**
 * Reset the global client instance (useful for testing)
 */
function resetGlobalClient() {
    globalClient = null;
}

export { StateTemplateClient, UpdateError, applyUpdate, createClient, getGlobalClient, initializeGlobalClient, resetGlobalClient, setInitialContent };
//# sourceMappingURL=index.esm.js.map
