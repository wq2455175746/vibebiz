export const PREVIEW_SANDBOX = 'allow-scripts allow-forms allow-same-origin'

export function buildPreviewUrl(projectId: string): string {
  const base = (import.meta.env.VITE_DP_BASE || '/dp/api/v1').replace(/\/$/, '')
  return `${base}/projects/${encodeURIComponent(projectId)}/preview/`
}
