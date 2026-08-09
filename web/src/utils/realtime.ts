// WebSocket 实时推送封装：首帧 token 认证 + 指数退避重连 + 设备订阅 + requestAnimationFrame 批量派发
type Handler = (msg: any) => void

class RealtimeClient {
  private ws: WebSocket | null = null
  private handlers = new Set<Handler>()
  private subscribed = new Set<number>()
  private timer: number | null = null
  private retryDelay = 3000 // 重连退避：3s → 6s → 12s → … → 30s 封顶，连接成功后复位
  // 批量派发缓冲：同一帧内收到的多条消息合并到下一帧统一派发，
  // 避免高频推送时同步遍历 handler 阻塞主线程、放大成请求风暴。
  private pending: any[] = []
  private rafId: number | null = null

  connect() {
    const token = localStorage.getItem('token')
    if (!token || this.ws) return
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    // token 不放进 URL（会泄露进 nginx/代理 access log）：连接后首帧发送 {type:'auth',token}
    this.ws = new WebSocket(`${proto}://${location.host}/api/v1/ws`)
    this.ws.onopen = () => {
      this.retryDelay = 3000 // 连接成功，退避复位
      this.send({ type: 'auth', token })
      // 重连后恢复订阅
      this.subscribed.forEach((id) => this.send({ type: 'subscribe', deviceId: id }))
    }
    this.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'auth_failed') {
          // token 失效：停止重连并跳转登录
          this.stop()
          localStorage.removeItem('token')
          location.href = '/login'
          return
        }
        this.pending.push(msg)
        this.scheduleFlush()
      } catch { /* ignore */ }
    }
    this.ws.onclose = () => {
      this.ws = null
      if (localStorage.getItem('token')) {
        this.timer = window.setTimeout(() => this.connect(), this.retryDelay)
        this.retryDelay = Math.min(this.retryDelay * 2, 30000)
      }
    }
  }

  private stop() {
    if (this.timer) window.clearTimeout(this.timer)
    if (this.rafId != null) cancelAnimationFrame(this.rafId)
    this.rafId = null
    this.pending = []
    this.ws?.close()
    this.ws = null
  }

  // 下一帧统一派发：高频推送时一帧内只触发一轮 handler 遍历。
  // device_status 按 deviceId 做 last-wins 合并（只关心最终在线态）；
  // alarm / telemetry 等不合并，全部派发。
  private scheduleFlush() {
    if (this.rafId != null) return
    this.rafId = requestAnimationFrame(() => {
      this.rafId = null
      const batch = this.pending
      this.pending = []
      const merged: any[] = []
      const statusByDevice = new Map<number, any>()
      for (const m of batch) {
        if (m && m.type === 'device_status' && m.deviceId != null) {
          statusByDevice.set(m.deviceId, m)
        } else {
          merged.push(m)
        }
      }
      statusByDevice.forEach((m) => merged.push(m))
      for (const h of this.handlers) {
        for (const m of merged) h(m)
      }
    })
  }

  close() {
    this.stop()
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
