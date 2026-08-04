// 统一时间格式化工具：全站展示格式保持一致（YYYY-MM-DD HH:mm:ss，24小时制）

const pad = (n: number) => String(n).padStart(2, '0')

/** 完整日期时间：YYYY-MM-DD HH:mm:ss，空值返回 '-' */
export function fmtDateTime(s: string | null | undefined): string {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return '-'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 仅日期：YYYY-MM-DD，空值返回 '-' */
export function fmtDate(s: string | null | undefined): string {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return '-'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 仅时间：HH:mm:ss */
export function fmtTime(s: string | number | Date | null | undefined): string {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return '-'
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
