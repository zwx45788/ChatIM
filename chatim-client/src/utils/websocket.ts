import { useChatStore } from '@/stores/chat'

export class WebSocketManager {
  private ws: WebSocket | null = null
  private url: string
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectTimer: any = null
  private token: string | null = null

  constructor() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    // In development, vite proxy handles /ws, so we can use relative path or same host
    // But for WebSocket constructor we need absolute URL.
    // If we use proxy, we connect to current host /ws
    this.url = `${protocol}//${host}/ws`
  }

  connect(token: string) {
    this.token = token
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
    }

    this.ws = new WebSocket(`${this.url}?token=${token}`)

    this.ws.onopen = () => {
      console.log('WebSocket connected')
      this.reconnectAttempts = 0
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
    }

    this.ws.onmessage = (event) => {
      try {
        console.log('WS Received:', event.data)
        const message = JSON.parse(event.data)
        const chatStore = useChatStore()
        chatStore.handleNewMessage(message)
      } catch (e) {
        console.error('Failed to parse websocket message', e)
      }
    }

    this.ws.onclose = () => {
      console.log('WebSocket disconnected')
      this.reconnect()
    }

    this.ws.onerror = (error) => {
      console.error('WebSocket error', error)
      this.ws?.close()
    }
  }

  disconnect() {
    this.token = null
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
  }

  private reconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++
      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
      console.log(`Reconnecting in ${delay}ms...`)
      this.reconnectTimer = setTimeout(() => {
        if (this.token) {
          this.connect(this.token)
        }
      }, delay)
    } else {
      console.error('Max reconnect attempts reached')
    }
  }
}

export const wsManager = new WebSocketManager()
