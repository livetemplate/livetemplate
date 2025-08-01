// Main exports
export { StateTemplateClient } from './client';
export type { RealtimeUpdate, ClientConfig, UpdateResult } from './types';
export { UpdateError } from './types';

// Convenience functions
export { createClient, initializeGlobalClient, applyUpdate, setInitialContent, getGlobalClient, resetGlobalClient } from './utils.js';
