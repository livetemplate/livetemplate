import { StateTemplateClient } from './client';
import { RealtimeUpdate, ClientConfig } from './types';

// Global client instance for convenience functions
let globalClient: StateTemplateClient | null = null;

/**
 * Create a new StateTemplate client instance
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
export function createClient(config?: ClientConfig): StateTemplateClient {
  return new StateTemplateClient(config);
}

/**
 * Create and set a global client instance for convenience functions
 * @param config - Configuration options
 * @returns StateTemplateClient instance
 */
export function initializeGlobalClient(config?: ClientConfig): StateTemplateClient {
  globalClient = createClient(config);
  return globalClient;
}

/**
 * Apply a single update using the global client instance
 * @param update - Update object to apply
 * @throws Error if global client is not initialized
 */
export async function applyUpdate(update: RealtimeUpdate): Promise<void> {
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
export function setInitialContent(html: string, containerId?: string): void {
  if (!globalClient) {
    throw new Error('Global client not initialized. Call initializeGlobalClient() first or use StateTemplateClient directly.');
  }
  
  globalClient.setInitialContent(html, containerId);
}

/**
 * Get the global client instance
 * @returns StateTemplateClient instance or null if not initialized
 */
export function getGlobalClient(): StateTemplateClient | null {
  return globalClient;
}

/**
 * Reset the global client instance (useful for testing)
 */
export function resetGlobalClient(): void {
  globalClient = null;
}
