export class WSClient {
  private socket: WebSocket | null = null;
  private url: string;

  constructor() {
    this.url = getWebSocketUrl();
  }

  connect(
    onOpen: () => void,
    onMessage: (data: any) => void,
    onClose: () => void,
    onError: (error: Event) => void
  ) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      return;
    }

    this.socket = new WebSocket(this.url);
    this.socket.onopen = onOpen;
    this.socket.onmessage = (event) => onMessage(event.data);
    this.socket.onclose = onClose;
    this.socket.onerror = onError;
  }

  disconnect() {
    this.socket?.close();
    this.socket = null;
  }

  sendMessage<T>(data: T) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(data));
    }
  }

  isConnected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }
}

function getWebSocketUrl(): string {
  const apiBase = import.meta.env.VITE_API_BASE_URL
  let baseUrl: URL

  try {
    // Try parsing as absolute URL
    baseUrl = new URL(apiBase)
  } catch {
    // If parsing fails, treat as relative URL
    baseUrl = new URL(apiBase, window.location.origin)
  }

  const protocol = baseUrl.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${baseUrl.host}/api/v1/ws`
}