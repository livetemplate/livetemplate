import { RealtimeUpdate, ClientConfig, UpdateResult } from './types';
/**
 * StateTemplate client for applying real-time HTML updates using morphdom
 */
export declare class StateTemplateClient {
    private config;
    constructor(config?: ClientConfig);
    /**
     * Apply a real-time update to the DOM
     * @param update - The update object from StateTemplate server
     * @returns Promise resolving to update result
     */
    applyUpdate(update: RealtimeUpdate): Promise<UpdateResult>;
    /**
     * Apply multiple updates in sequence
     * @param updates - Array of update objects
     * @returns Promise resolving to array of update results
     */
    applyUpdates(updates: RealtimeUpdate[]): Promise<UpdateResult[]>;
    /**
     * Set initial HTML content for the page
     * @param html - Initial HTML content
     * @param containerId - ID of container element (default: 'app')
     */
    setInitialContent(html: string, containerId?: string): void;
    /**
     * Check if an element exists in the DOM
     * @param fragmentId - Fragment ID to check
     * @returns boolean indicating if element exists
     */
    hasElement(fragmentId: string): boolean;
    private validateUpdate;
    private findElement;
    private replaceElement;
    private appendToElement;
    private prependToElement;
    private removeElement;
    private debugLog;
}
//# sourceMappingURL=client.d.ts.map