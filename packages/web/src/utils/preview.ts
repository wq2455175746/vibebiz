export const PREVIEW_SANDBOX = 'allow-scripts allow-forms allow-same-origin'

const imageExtensions = new Set(['png', 'jpg', 'jpeg', 'webp', 'gif', 'svg'])
const pptExtensions = new Set(['ppt', 'pptx'])

export type PreviewKind = 'html' | 'pdf' | 'markdown' | 'image' | 'ppt' | 'other'

export function buildPreviewUrl(projectId: string, previewPath = ''): string {
  const base = (import.meta.env.VITE_DP_BASE || '/dp/api/v1').replace(/\/$/, '')
  const relative = previewPath.trim().replace(/^\/+/, '')
  const suffix = relative ? relative.split('/').map(encodeURIComponent).join('/') : ''
  return `${base}/projects/${encodeURIComponent(projectId)}/preview/${suffix}`
}

export function previewKind(previewPath: string): PreviewKind {
  const clean = previewPath.split(/[?#]/, 1)[0] ?? ''
  const name = clean.toLowerCase()
  if (name.endsWith('.html') || name.endsWith('.htm')) {
    return 'html'
  }
  if (name.endsWith('.pdf')) {
    return 'pdf'
  }
  if (name.endsWith('.md') || name.endsWith('.markdown')) {
    return 'markdown'
  }
  const extension = name.split('.').pop() ?? ''
  if (imageExtensions.has(extension)) {
    return 'image'
  }
  if (pptExtensions.has(extension)) {
    return 'ppt'
  }
  return 'other'
}
