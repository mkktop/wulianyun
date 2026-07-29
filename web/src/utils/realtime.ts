// WebSocket 实时推送封装：自动重连 + 设备订阅
type Handler = (msg: any) => void

class RealtimeClient {
  private ws: WebSocket | null = null
  private handlers = new Set<Handler>()
  private subscribed = new Set<number>()
  private timer: number | null = null

  connect() {
    const token = localStorage.getItem('token')
    if (!token || this.ws) return
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    this.ws = new WebSocket(`${proto}://${location.host}/api/v1/ws?token=${token}`)
    this.ws.onopen = () => {
      // 重连后恢复订阅
      this.subscribed.forEach((id) => this.send({ type: 'subscribe', deviceId: id }))
    }
    this.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        this.handlers.forEach((h) => h(msg))
      } catch { /* ignore */ }
    }
    this.ws.onclose = () => {
      this.ws = null
      if (localStorage.getItem('token')) {
        this.timer = window.setTimeout(() => this.connect(), 3000)
      }
    }
  }

  close() {
    if (this.timer) window.clearTimeout(this.timer)
    this.ws?.close()
    this.ws = null
    this.subscribed.clear()
  }

  on(h: Handler) { this.handlers.add(h); this.connect() }
  off(h: Handler) { this.handlers.delete(h) }

  subscribe(deviceId: number) {
    this.subscribed.add(deviceId)
    this.send({ type: 'subscribe', deviceId })
  }
  unsubscribe(deviceId: number) {
    this.subscribed.delete(deviceId)
    this.send({ type: 'unsubscribe', deviceId })
  }

  private send(obj: any) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(obj))
  }
}

export const realtime = new RealtimeClient()
