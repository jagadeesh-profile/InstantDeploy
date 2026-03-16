const API_BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8082";

type WebSocketEventType = "deployment_status" | "deployment_log";

export type WebSocketMessage = {
  type: WebSocketEventType;
  payload: Record<string, unknown>;
  timestamp?: string;
};

type Listener = (message: WebSocketMessage) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private readonly maxReconnectAttempts = 5;
  private readonly reconnectDelayMs = 3000;
  private readonly listeners = new Map<WebSocketEventType, Set<Listener>>();
  private currentUserID = "";

  connect(userID: string) {
    if (!userID) return;

    this.currentUserID = userID;
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const wsURL = this.buildWebSocketURL(userID);
    this.ws = new WebSocket(wsURL);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
    };

    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WebSocketMessage;
        this.emit(message);
      } catch {
        // Ignore malformed events.
      }
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.attemptReconnect();
    };

    this.ws.onerror = () => {
      // close event triggers retry flow
    };
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.reconnectAttempts = 0;
    this.currentUserID = "";
  }

  on(eventType: WebSocketEventType, listener: Listener): () => void {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set());
    }
    this.listeners.get(eventType)!.add(listener);

    return () => {
      this.listeners.get(eventType)?.delete(listener);
    };
  }

  private emit(message: WebSocketMessage) {
    const eventListeners = this.listeners.get(message.type);
    if (!eventListeners) return;
    eventListeners.forEach((listener) => listener(message));
  }

  private attemptReconnect() {
    if (!this.currentUserID) return;
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;

    this.reconnectAttempts += 1;
    window.setTimeout(() => {
      this.connect(this.currentUserID);
    }, this.reconnectDelayMs);
  }

  private buildWebSocketURL(userID: string): string {
    const parsed = new URL(API_BASE_URL);
    parsed.protocol = parsed.protocol === "https:" ? "wss:" : "ws:";
    parsed.pathname = "/ws";
    parsed.searchParams.set("user_id", userID);
    return parsed.toString();
  }
}

export const websocketService = new WebSocketService();
