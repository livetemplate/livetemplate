export interface TreeNode {
  [key: string]: any;
  s?: string[];
}

export interface UpdateResult {
  html: string;
  changed: boolean;
  dom?: Element;
}

export interface ResponseMetadata {
  success: boolean;
  errors: { [key: string]: string };
  action?: string;
}

export interface UpdateResponse {
  tree: TreeNode;
  meta?: ResponseMetadata;
}

export interface LiveTemplateClientOptions {
  wsUrl?: string;
  liveUrl?: string;
  autoReconnect?: boolean;
  reconnectDelay?: number;
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Event) => void;
}
