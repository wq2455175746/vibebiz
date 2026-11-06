import type { QuotaExceededInfo } from '@/api/client'

export function quotaDescription(info: QuotaExceededInfo | null): string {
  switch (info?.quotaType) {
    case 'project':
      return '今日创建项目数已超出限额。'
    case 'token':
      return '今日 Token 使用量已超出限额。'
    case 'upload_count':
      return '今日上传文件数已超出限额。'
    case 'upload_bytes':
      return '今日上传文件大小已超出限额。'
    default:
      return '今日使用额度已超出限额。'
  }
}

export function quotaUsage(info: QuotaExceededInfo | null): string {
  if (info?.used === undefined || info?.limit === undefined) {
    return ''
  }
  return `当前：${info.used} / ${info.limit}`
}
