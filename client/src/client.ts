// @ts-ignore - morphdom doesn't have proper type definitions
const morphdom = require('morphdom');
import { RealtimeUpdate, ClientConfig, UpdateError, UpdateResult } from './types';

/**
 * StateTemplate client for applying real-time HTML updates using morphdom
 */
export class StateTemplateClient {
  private config: Required<ClientConfig>;

  constructor(config: ClientConfig = {}) {
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
  public async applyUpdate(update: RealtimeUpdate): Promise<UpdateResult> {
    try {
      this.validateUpdate(update);
      this.debugLog(`Applying update for fragment: ${update.fragment_id}`, update);

      const element = this.findElement(update.fragment_id);
      if (!element) {
        throw new UpdateError(
          `Element with ID '${update.fragment_id}' not found`,
          update.fragment_id,
          update.action
        );
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
          throw new UpdateError(
            `Unsupported action: ${update.action}`,
            update.fragment_id,
            update.action
          );
      }
    } catch (error) {
      const updateError = error instanceof UpdateError 
        ? error 
        : new UpdateError(
            `Failed to apply update: ${error instanceof Error ? error.message : String(error)}`,
            update.fragment_id,
            update.action,
            error instanceof Error ? error : undefined
          );

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
  public async applyUpdates(updates: RealtimeUpdate[]): Promise<UpdateResult[]> {
    const results: UpdateResult[] = [];
    
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
  public setInitialContent(html: string, containerId: string = 'app'): void {
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
  public hasElement(fragmentId: string): boolean {
    return this.findElement(fragmentId) !== null;
  }

  private validateUpdate(update: RealtimeUpdate): void {
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

  private findElement(fragmentId: string): Element | null {
    return document.getElementById(fragmentId) || document.querySelector(`[data-fragment-id="${fragmentId}"]`);
  }

  private replaceElement(element: Element, update: RealtimeUpdate): UpdateResult {
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
        onBeforeElUpdated: (fromEl: Element, toEl: Element) => {
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
      }) as Element;

      this.debugLog(`Successfully replaced element: ${update.fragment_id}`);
      
      return {
        success: true,
        fragmentId: update.fragment_id,
        action: update.action,
        element: morphedElement
      };
    } catch (error) {
      throw new UpdateError(
        `Failed to replace element: ${error instanceof Error ? error.message : String(error)}`,
        update.fragment_id,
        update.action,
        error instanceof Error ? error : undefined
      );
    }
  }

  private appendToElement(element: Element, update: RealtimeUpdate): UpdateResult {
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
    } catch (error) {
      throw new UpdateError(
        `Failed to append to element: ${error instanceof Error ? error.message : String(error)}`,
        update.fragment_id,
        update.action,
        error instanceof Error ? error : undefined
      );
    }
  }

  private prependToElement(element: Element, update: RealtimeUpdate): UpdateResult {
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
    } catch (error) {
      throw new UpdateError(
        `Failed to prepend to element: ${error instanceof Error ? error.message : String(error)}`,
        update.fragment_id,
        update.action,
        error instanceof Error ? error : undefined
      );
    }
  }

  private removeElement(element: Element, update: RealtimeUpdate): UpdateResult {
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
    } catch (error) {
      throw new UpdateError(
        `Failed to remove element: ${error instanceof Error ? error.message : String(error)}`,
        update.fragment_id,
        update.action,
        error instanceof Error ? error : undefined
      );
    }
  }

  private debugLog(message: string, data?: any): void {
    if (this.config.debug) {
      console.log(`[StateTemplateClient] ${message}`, data || '');
    }
  }
}
