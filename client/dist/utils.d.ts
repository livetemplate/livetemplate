import { StateTemplateClient } from './client';
import { RealtimeUpdate, ClientConfig } from './types';
/**
 * Create a new StateTemplate client instance
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
export declare function createClient(config?: ClientConfig): StateTemplateClient;
/**
 * Create and set a global client instance for convenience functions
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
export declare function initializeGlobalClient(config?: ClientConfig): StateTemplateClient;
/**
 * Apply a single update using the global client instance
 * @param update - Update object to apply
 * @throws Error if global client is not initialized
 */
export declare function applyUpdate(update: RealtimeUpdate): Promise<void>;
/**
 * Set initial content using the global client instance
 * @param html - Initial HTML content
 * @param containerId - ID of container element
 * @throws Error if global client is not initialized
 */
export declare function setInitialContent(html: string, containerId?: string): void;
/**
 * Get the global client instance
 * @returns StateTemplateClient instance or null if not initialized
 */
export declare function getGlobalClient(): StateTemplateClient | null;
/**
 * Reset the global client instance (useful for testing)
 */
export declare function resetGlobalClient(): void;
//# sourceMappingURL=utils.d.ts.map