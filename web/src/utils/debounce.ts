// 轻量去抖：短时间内多次触发合并为最后一次执行。
// 用于 WS 消息触发的列表刷新 / 通知，避免高频推送放大成请求风暴或通知堆叠。
export function debounce<T extends (...args: any[]) => any>(fn: T, ms = 400) {
  let t: number | undefined
  const wrapped = (...args: Parameters<T>) => {
    if (t !== undefined) clearTimeout(t)
    t = window.setTimeout(() => {
      t = undefined
      fn(...args)
    }, ms)
  }
  wrapped.flush = () => {
    if (t !== undefined) {
      clearTimeout(t)
      t = undefined
      fn()
    }
  }
  return wrapped
}
