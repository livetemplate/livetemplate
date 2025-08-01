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
export class UpdateError extends Error {
  constructor(
    message: string,
    public readonly fragmentId: string,
    public readonly action: string,
    public readonly cause?: Error
  ) {
    super(message);
    this.name = 'UpdateError';
  }
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
