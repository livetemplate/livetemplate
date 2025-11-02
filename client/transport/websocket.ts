import type { LiveTemplateClientOptions, UpdateResponse } from "../types";
import type { Logger } from "../utils/logger";

export interface WebSocketTransportOptions {
  url: string;
  autoReconnect?: boolean;
  reconnectDelay?: number;
  maxReconnectDelay?: number; // Maximum delay between reconnect attempts (default: 16000ms)
  maxReconnectAttempts?: number; // Maximum number of reconnect attempts (default: 10, 0 = unlimited)
  onOpen?: (socket: WebSocket) => void;
  onMessage?: (event: MessageEvent<string>) => void;
  onClose?: (event: CloseEvent) => void;
  onReconnectAttempt?: (attempt: number, delay: number) => void;
  onReconnectFailed?: () => void; // Called when max reconnect attempts reached
  onError?: (event: Event) => void;
}

/**
 * Lightweight wrapper around browser WebSocket with optional auto-reconnect support.
 * Implements exponential backoff with jitter to prevent thundering herd.
 */
export class WebSocketTransport {
  private socket: WebSocket | null = null;
  private reconnectTimer: number | null = null;
  private manuallyClosed = false;
  private reconnectAttempts = 0;

  constructor(private readonly options: WebSocketTransportOptions) {}

  connect(): void {
    this.manuallyClosed = false;
    this.clearReconnectTimer();

    this.socket = new WebSocket(this.options.url);
    const socket = this.socket;

    socket.onopen = () => {
      // Reset reconnect attempts on successful connection
      this.reconnectAttempts = 0;
      this.options.onOpen?.(socket);
    };

    socket.onmessage = (event: MessageEvent<string>) => {
      this.options.onMessage?.(event);
    };

    socket.onclose = (event: CloseEvent) => {
      this.options.onClose?.(event);
      if (!this.manuallyClosed && this.options.autoReconnect) {
        this.scheduleReconnect();
      }
    };

    socket.onerror = (event: Event) => {
      this.options.onError?.(event);
    };
  }

  send(data: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(data);
    }
  }

  disconnect(): void {
    this.manuallyClosed = true;
    this.clearReconnectTimer();
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }

  getSocket(): WebSocket | null {
    return this.socket;
  }

  private scheduleReconnect(): void {
    this.clearReconnectTimer();

    // Check if max reconnect attempts reached
    const maxAttempts = this.options.maxReconnectAttempts ?? 10;
    if (maxAttempts > 0 && this.reconnectAttempts >= maxAttempts) {
      this.options.onReconnectFailed?.();
      return;
    }

    this.reconnectAttempts++;

    // Calculate exponential backoff: baseDelay * 2^attempt
    const baseDelay = this.options.reconnectDelay ?? 1000;
    const maxDelay = this.options.maxReconnectDelay ?? 16000;
    const exponentialDelay = baseDelay * Math.pow(2, this.reconnectAttempts - 1);

    // Add jitter: random value between 0 and 1000ms to prevent thundering herd
    const jitter = Math.random() * 1000;

    // Calculate final delay with maximum cap
    const delay = Math.min(exponentialDelay + jitter, maxDelay);

    this.reconnectTimer = window.setTimeout(() => {
      this.options.onReconnectAttempt?.(this.reconnectAttempts, delay);
      this.connect();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

export interface WebSocketManagerConfig {
  options: LiveTemplateClientOptions;
  onConnected: () => void;
  onDisconnected: () => void;
  onMessage: (response: UpdateResponse, event: MessageEvent<string>) => void;
  onReconnectAttempt?: (attempt: number, delay: number) => void;
  onReconnectFailed?: () => void;
  onError?: (event: Event) => void;
  logger: Logger;
}

export interface WebSocketConnectResult {
  usingWebSocket: boolean;
  initialState?: UpdateResponse | null;
}

export class WebSocketManager {
  private transport: WebSocketTransport | null = null;

  constructor(private readonly config: WebSocketManagerConfig) {}

  async connect(): Promise<WebSocketConnectResult> {
    const liveUrl = this.getLiveUrl();

    const wsAvailable = await checkWebSocketAvailability(
      liveUrl,
      this.config.logger
    );
    if (!wsAvailable) {
      const initialState = await fetchInitialState(liveUrl, this.config.logger);
      return { usingWebSocket: false, initialState };
    }

    this.transport = new WebSocketTransport({
      url: this.getWebSocketUrl(),
      autoReconnect: this.config.options.autoReconnect,
      reconnectDelay: this.config.options.reconnectDelay,
      maxReconnectDelay: 16000, // 16 seconds maximum
      maxReconnectAttempts: 10, // 10 attempts before giving up
      onOpen: () => {
        this.config.onConnected();
      },
      onMessage: (event) => {
        try {
          const payload: UpdateResponse = JSON.parse(event.data);
          this.config.onMessage(payload, event);
        } catch (error) {
          this.config.logger.error("Failed to parse WebSocket message:", error);
        }
      },
      onClose: () => {
        this.config.onDisconnected();
      },
      onReconnectAttempt: (attempt, delay) => {
        this.config.onReconnectAttempt?.(attempt, delay);
      },
      onReconnectFailed: () => {
        this.config.onReconnectFailed?.();
      },
      onError: (event) => {
        this.config.onError?.(event);
      },
    });

    this.transport.connect();
    return { usingWebSocket: true };
  }

  disconnect(): void {
    this.transport?.disconnect();
    this.transport = null;
  }

  send(data: string): void {
    this.transport?.send(data);
  }

  getReadyState(): number | undefined {
    return this.transport?.getSocket()?.readyState;
  }

  getSocket(): WebSocket | null {
    return this.transport?.getSocket() ?? null;
  }

  private getWebSocketUrl(): string {
    const liveUrl = this.config.options.liveUrl || "/live";
    const baseUrl = this.config.options.wsUrl;
    if (baseUrl) {
      return baseUrl;
    }
    return `ws://${window.location.host}${liveUrl}`;
  }

  private getLiveUrl(): string {
    return this.config.options.liveUrl || window.location.pathname;
  }
}

export async function checkWebSocketAvailability(
  liveUrl: string,
  logger?: Logger
): Promise<boolean> {
  try {
    const response = await fetch(liveUrl, {
      method: "HEAD",
    });

    const wsHeader = response.headers.get("X-LiveTemplate-WebSocket");
    if (wsHeader) {
      return wsHeader === "enabled";
    }

    return true;
  } catch (error) {
    logger?.warn("Failed to check WebSocket availability:", error);
    return true;
  }
}

export async function fetchInitialState(
  liveUrl: string,
  logger?: Logger
): Promise<UpdateResponse | null> {
  try {
    const response = await fetch(liveUrl, {
      method: "GET",
      credentials: "include",
      headers: {
        Accept: "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch initial state: ${response.status}`);
    }

    return (await response.json()) as UpdateResponse;
  } catch (error) {
    logger?.warn("Failed to fetch initial state:", error);
    return null;
  }
}
