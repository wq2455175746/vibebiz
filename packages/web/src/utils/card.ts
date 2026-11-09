import type { BusinessCard, CardContentType, CardFeatured, CardProjectItem } from '@/types'

export const CARD_MAX_ITEMS = 5
export const CARD_MAX_TAGS = 5
export const CARD_MAX_TAG_LENGTH = 12
export const CARD_MAX_UPLOAD_BYTES = 2 * 1024 * 1024

export const CARD_COVER_PLACEHOLDER =
  'data:image/svg+xml;charset=utf-8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#f3f4f6"/><text x="320" y="190" text-anchor="middle" fill="#6b7280" font-size="28" font-family="sans-serif">Vibebiz</text></svg>',
  )

export const CARD_TYPE_LABELS: Record<CardContentType, string> = {
  html: 'HTML',
  image: '图片',
  pdf: 'PDF',
  video: '视频',
  document: '文档',
}

const imageExtensions = new Set(['png', 'jpg', 'jpeg', 'webp', 'gif', 'svg'])
const videoExtensions = new Set(['mp4', 'webm', 'mov', 'm4v'])
const documentExtensions = new Set(['md', 'markdown', 'txt', 'doc', 'docx'])

export function cardContentType(previewPath: string): CardContentType {
  const clean = previewPath.split(/[?#]/, 1)[0] ?? ''
  const name = clean.toLowerCase()
  const extension = name.includes('.') ? (name.split('.').pop() ?? '') : ''
  if (name.endsWith('.html') || name.endsWith('.htm')) return 'html'
  if (name.endsWith('.pdf')) return 'pdf'
  if (imageExtensions.has(extension)) return 'image'
  if (videoExtensions.has(extension)) return 'video'
  if (documentExtensions.has(extension)) return 'document'
  return 'html'
}

export function addCardTag(tags: string[], raw: string): { tags: string[]; error: string } {
  const value = raw.trim()
  if (!value) return { tags, error: '' }
  if (tags.includes(value)) return { tags, error: '标签已存在' }
  if (value.length > CARD_MAX_TAG_LENGTH) {
    return { tags, error: `单个标签不能超过 ${CARD_MAX_TAG_LENGTH} 个字` }
  }
  if (tags.length >= CARD_MAX_TAGS) return { tags, error: `最多 ${CARD_MAX_TAGS} 个标签` }
  return { tags: [...tags, value], error: '' }
}

export function canAddCardItem(items: CardProjectItem[], projectId: string): boolean {
  return items.length < CARD_MAX_ITEMS && !items.some((item) => item.projectId === projectId)
}

export function pageCards<T>(items: T[], page: number, pageSize: number): T[] {
  const size = Math.max(1, Math.floor(pageSize))
  const safePage = Math.max(1, Math.floor(page))
  return items.slice((safePage - 1) * size, safePage * size)
}

export function emptyCard(now = new Date()): BusinessCard {
  const timestamp = now.toISOString()
  return {
    id: 'me',
    ownerUserId: null,
    ownerName: '',
    avatarUrl: null,
    headline: '',
    tags: [],
    items: [],
    featured: null,
    published: false,
    commentCount: 0,
    createdAt: timestamp,
    updatedAt: timestamp,
  }
}

export function normalizeCard(card: BusinessCard): BusinessCard {
  const items = Array.isArray(card.items)
    ? card.items
        .filter((item) => item && typeof item.projectId === 'string')
        .slice(0, CARD_MAX_ITEMS)
    : []
  const tags = Array.isArray(card.tags)
    ? Array.from(
        new Set(
          card.tags
            .filter((tag) => typeof tag === 'string' && tag.trim().length > 0)
            .map((tag) => tag.trim()),
        ),
      ).slice(0, CARD_MAX_TAGS)
    : []

  let featured: CardFeatured = null
  if (card.featured?.kind === 'project') {
    const projectId = card.featured.projectId
    if (items.some((item) => item.projectId === projectId)) {
      featured = { kind: 'project', projectId }
    }
  } else if (card.featured?.kind === 'file') {
    featured = card.featured
  }

  return {
    id: typeof card.id === 'string' && card.id ? card.id : 'me',
    ownerUserId: typeof card.ownerUserId === 'number' ? card.ownerUserId : null,
    ownerName: typeof card.ownerName === 'string' ? card.ownerName : '',
    avatarUrl: typeof card.avatarUrl === 'string' ? card.avatarUrl : null,
    headline: typeof card.headline === 'string' ? card.headline : '',
    tags,
    items,
    featured,
    published: Boolean(card.published),
    commentCount: Number.isFinite(card.commentCount) ? card.commentCount : 0,
    createdAt: typeof card.createdAt === 'string' ? card.createdAt : new Date().toISOString(),
    updatedAt: typeof card.updatedAt === 'string' ? card.updatedAt : new Date().toISOString(),
  }
}

export function decodeCard(raw: string | null): BusinessCard | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<BusinessCard>
    if (!parsed || typeof parsed !== 'object' || typeof parsed.id !== 'string') return null
    return normalizeCard(parsed as BusinessCard)
  } catch {
    return null
  }
}

export function decodeCards(raw: string | null): BusinessCard[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter((item): item is BusinessCard => Boolean(item) && typeof item === 'object')
      .map((item) => normalizeCard(item))
      .filter((item) => item.id !== 'me')
  } catch {
    return []
  }
}

export function sortDiscoverCards(cards: BusinessCard[], mineId = 'me'): BusinessCard[] {
  return [...cards].sort((a, b) => {
    if (a.id === mineId) return -1
    if (b.id === mineId) return 1
    return b.updatedAt.localeCompare(a.updatedAt) || b.id.localeCompare(a.id)
  })
}

export function isPdfFile(file: { name: string; type: string }): boolean {
  return file.type === 'application/pdf'
}

export function validateCardUpload(file: { name: string; size: number; type: string }): string {
  const isPdf = isPdfFile(file)
  const isImage = file.type.startsWith('image/')
  if (!isPdf && !isImage) return '只支持 PDF 或图片'
  if (file.size <= 0 || file.size > CARD_MAX_UPLOAD_BYTES) return '文件为空或超过 2MB'
  return ''
}

export function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(new Error('文件读取失败'))
    reader.readAsDataURL(file)
  })
}
