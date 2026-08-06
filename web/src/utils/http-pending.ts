// HTTP 请求 pending 管理：GET 请求去重 + 路由切换取消。
// 单例 Map 由 api 拦截器写入、router beforeEach 取消，两者共享此模块（避免 api↔router 循环依赖）。
export const pending = new Map<string, AbortController>()

export function reqKey(method: string, url: string | undefined, params: any): string {
  return [method.toLowerCase(), url || '', JSON.stringify(params || {})].join('&')
}

// 取消所有进行中的请求（路由切换时调用，消除旧页面请求结果覆盖新页面数据的竞态）
export function cancelAllPending() {
  pending.forEach((c) => c.abort())
  pending.clear()
}
