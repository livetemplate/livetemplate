import type { LiveTemplateClientOptions, UpdateResponse } from "../types";
import type { Logger } from "../utils/logger";

export interface WebSocketTransportOptions {
  url: string;
  autoReconnect?: boolean;
  reconnectDelay?: number;
  onOpen?: (socket: WebSocket) => void;
  onMessage?: (event: MessageEvent<string>) => void;
  onClose?: (event: CloseEvent) => void;
  onReconnectAttempt?: () => void;
  onError?: (event: Event) => void;
}

/**
 * Lightweight wrapper around browser WebSocket with optional auto-reconnect support.
 */
export class WebSocketTransport {
  private socket: WebSocket | null = null;
  private reconnectTimer: number | null = null;
  private manuallyClosed = false;

  constructor(private readonly options: WebSocketTransportOptions) {}

  connect(): void {
    this.manuallyClosed = false;
    this.clearReconnectTimer();

    this.socket = new WebSocket(this.options.url);
    const socket = this.socket;

    socket.onopen = () => {
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
    const delay = this.options.reconnectDelay ?? 1000;
    this.reconnectTimer = window.setTimeout(() => {
      this.options.onReconnectAttempt?.();
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
  onReconnectAttempt?: () => void;
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
      onReconnectAttempt: () => {
        this.config.onReconnectAttempt?.();
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
