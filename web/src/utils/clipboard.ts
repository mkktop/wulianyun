import { ElMessage } from 'element-plus'

// 复制文本到剪贴板：
// 优先 Clipboard API（仅 HTTPS 安全上下文可用），HTTP 下回退 execCommand textarea 方案
export function copyText(text: string, silent = false) {
  const done = () => {
    if (!silent) ElMessage.success('已复制')
  }
  const fail = () => {
    if (!silent) ElMessage.error('复制失败，请手动复制')
  }
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(done).catch(() => fallbackCopy(text, done, fail))
  } else {
    fallbackCopy(text, done, fail)
  }
}

function fallbackCopy(text: string, done: () => void, fail: () => void) {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    if (document.execCommand('copy')) {
      done()
    } else {
      fail()
    }
  } catch {
    fail()
  } finally {
    document.body.removeChild(ta)
  }
}
