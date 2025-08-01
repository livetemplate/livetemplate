/**
 * RealtimeUpdate represents an update message from the StateTemplate server
 */
export interface RealtimeUpdate {
    fragment_id: string;
    html: string;
    action: string;
}
/**
 * Configuration options for the StateTemplate client
 */
export interface ClientConfig {
    /**
     * Whether to log debug information to console
     * @default false
     */
    debug?: boolean;
    /**
     * Custom morphdom options
     */
    morphOptions?: {
        /**
         * Called before a node is discarded
         */
        onBeforeNodeDiscarded?(node: Node): boolean;
        /**
         * Called before the children of a node are updated
         */
        onBeforeElUpdated?(fromEl: Element, toEl: Element): boolean;
        /**
         * Called when a node is added
         */
        onNodeAdded?(node: Node): Node;
        /**
         * Called when a node is discarded
         */
        onNodeDiscarded?(node: Node): void;
        /**
         * Called before an element's children are compared
         */
        onBeforeElChildrenUpdated?(fromEl: Element, toEl: Element): boolean;
    };
}
/**
 * Error thrown when an update operation fails
 */
export declare class UpdateError extends Error {
    readonly fragmentId: string;
    readonly action: string;
    readonly cause?: Error | undefined;
    constructor(message: string, fragmentId: string, action: string, cause?: Error | undefined);
}
/**
 * Result of an update operation
 */
export interface UpdateResult {
    success: boolean;
    fragmentId: string;
    action: string;
    error?: Error;
    element?: Element;
}
//# sourceMappingURL=types.d.ts.map