# 个人名片与编码社交发现页 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 web 与 ops-web 中实现个人名片编辑、公开名片/项目分享页、发现页与名片管理页的前端效果。

**Architecture:** 名片数据通过 Pinia store + localStorage 持久化，添加项目复用现有已发布项目与项目详情 API。公开分享页使用独立路由；发现页把自有已发布名片与 mock 名片合并后分页展示。ops-web 使用独立 localStorage mock 仓储。

**Tech Stack:** Vue 3 Composition API + `<script setup lang="ts">` + Pinia + Vue Router + PrimeVue + Vitest；ops-web 复用 DataTable / TablePagination / useAdminPage。

**Spec:** `docs/superpowers/specs/2026-11-08-business-card-social-design.md`

## Global Constraints

- 本期只改 `packages/web` 与 `packages/ops-web`，不改 backend / sandbox / 数据库 / 现有 CP `/p/:slug` 分享页。
- 颜色只允许黑 / 白 / 灰 / 蓝，新增样式必须使用 `global.css` 已有 CSS 变量。
- web 不得访问 admin；ops-web 不得访问 DP / sandbox / web。
- 名片最多 5 个项目、5 个标签；单标签 ≤12 字；上传仅 PDF/图片且 ≤2MB。
- 自有名片固定 `id = 'me'`；mock 名片 `demo-1` ~ `demo-12`。
- 评论 / 私信按钮必须可见但禁用，点击只提示“即将上线”。
- 所有新代码先写失败测试再实现；任务结束跑对应包的 `npm test -- --run`。
- 最终验证：`cd packages/web && npm test -- --run && npm run build && npm run lint`，`cd packages/ops-web && npm test -- --run && npm run build && npm run lint`，根目录 `make validate`。
- 计划中的 `git commit` 步骤只在用户明确要求提交时执行；否则跳过提交，仅保留工作区改动。

---

## 文件结构

```
packages/web/src/
├── types/index.ts                     # 新增名片类型
├── utils/card.ts                      # 纯函数：类型映射/校验/分页/编解码
├── mocks/cards.ts                     # 12 张 mock 名片 + 4 类演示资源
├── stores/card.ts                     # 名片状态、localStorage、发布、发现分页
├── components/card/
│   ├── MyCardPanel.vue                # 我的名片编辑页
│   ├── DiscoverPanel.vue              # 发现页
│   ├── AddProjectDialog.vue           # 添加已发布项目
│   ├── CardProjectItem.vue            # 内容模块项目项
│   ├── CardProfilePanel.vue           # 个人信息模块
│   ├── CardPreview.vue                # 主预览
│   └── CardTile.vue                   # 发现页卡片
├── views/CardShareView.vue            # 名片分享页
├── views/ProjectShareView.vue         # 项目分享页（4 类内容）
├── styles/card.css                    # 名片模块样式
├── components/home/HomeSidebar.vue    # 菜单入口
├── views/HomeView.vue                 # my-card / discover 分支
└── router/index.ts                    # 新路由

packages/ops-web/src/
├── types/index.ts                     # AdminCardSummary
├── api/cards.ts                       # localStorage mock 仓储
├── mocks/cards.ts                     # ops mock 种子
├── views/CardsView.vue                # 名片管理
├── router/navigation.ts               # 新增菜单
└── router/routes.ts                   # 新增路由
```

---

### Task 1: 名片类型与纯函数

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Create: `packages/web/src/utils/card.ts`
- Test: `packages/web/src/utils/card.test.ts`

**Interfaces:**
- Produces:
  - `CardContentType = 'html' | 'image' | 'pdf' | 'video' | 'document'`
  - `CardProjectItem`、`CardFeatured`、`BusinessCard`
  - `cardContentType(previewPath: string): CardContentType`
  - `addCardTag(tags: string[], raw: string): { tags: string[]; error: string }`
  - `canAddCardItem(items: CardProjectItem[], projectId: string): boolean`
  - `pageCards<T>(items: T[], page: number, pageSize: number): T[]`
  - `emptyCard(now?: Date): BusinessCard`
  - `normalizeCard(card: BusinessCard): BusinessCard`
  - `decodeCard(raw: string | null): BusinessCard | null`
  - `decodeCards(raw: string | null): BusinessCard[]`
  - `sortDiscoverCards(cards: BusinessCard[], mineId?: string): BusinessCard[]`
  - `validateCardUpload(file: { name: string; size: number; type: string }): string`
  - `readFileAsDataUrl(file: File): Promise<string>`
  - 常量 `CARD_MAX_ITEMS`、`CARD_MAX_TAGS`、`CARD_MAX_TAG_LENGTH`、`CARD_MAX_UPLOAD_BYTES`、`CARD_TYPE_LABELS`

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/utils/card.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import {
  CARD_MAX_ITEMS,
  CARD_MAX_TAG_LENGTH,
  addCardTag,
  canAddCardItem,
  cardContentType,
  decodeCard,
  decodeCards,
  emptyCard,
  normalizeCard,
  pageCards,
  sortDiscoverCards,
  validateCardUpload,
} from './card'
import type { BusinessCard, CardProjectItem } from '@/types'

function item(projectId: string): CardProjectItem {
  return {
    projectId,
    name: projectId,
    description: '',
    coverUrl: null,
    type: 'html',
    shareUrl: `/p/${projectId}`,
    addedAt: '2026-11-08T00:00:00.000Z',
  }
}

function card(id: string, overrides: Partial<BusinessCard> = {}): BusinessCard {
  return { ...emptyCard(), id, ...overrides }
}

describe('cardContentType', () => {
  it('maps preview paths to card content types', () => {
    expect(cardContentType('index.html')).toBe('html')
    expect(cardContentType('app.htm')).toBe('html')
    expect(cardContentType('docs/a.pdf')).toBe('pdf')
    expect(cardContentType('assets/cover.JPG')).toBe('image')
    expect(cardContentType('media/demo.mp4')).toBe('video')
    expect(cardContentType('README.md')).toBe('document')
    expect(cardContentType('unknown.bin')).toBe('html')
  })

  it('ignores query and hash suffixes', () => {
    expect(cardContentType('demo.pdf?v=2#page=1')).toBe('pdf')
  })
})

describe('addCardTag', () => {
  it('adds a trimmed tag', () => {
    expect(addCardTag([], '  Vue  ')).toEqual({ tags: ['Vue'], error: '' })
  })

  it('rejects duplicates and overlong tags', () => {
    expect(addCardTag(['Vue'], 'Vue').error).toBe('标签已存在')
    expect(addCardTag([], 'a'.repeat(CARD_MAX_TAG_LENGTH + 1)).error).toContain('不能超过')
  })

  it('rejects a sixth tag', () => {
    expect(addCardTag(['a', 'b', 'c', 'd', 'e'], 'f').error).toContain('最多')
  })
})

describe('canAddCardItem', () => {
  it('rejects duplicates and more than five items', () => {
    const items = [item('p1')]
    expect(canAddCardItem(items, 'p2')).toBe(true)
    expect(canAddCardItem(items, 'p1')).toBe(false)
    const full = Array.from({ length: CARD_MAX_ITEMS }, (_, index) => item(`p${index}`))
    expect(canAddCardItem(full, 'p9')).toBe(false)
  })
})

describe('pageCards', () => {
  it('returns the requested slice', () => {
    expect(pageCards([1, 2, 3, 4, 5, 6, 7], 2, 3)).toEqual([4, 5, 6])
    expect(pageCards([1, 2], 0, 3)).toEqual([1, 2])
  })
})

describe('normalizeCard', () => {
  it('keeps a featured project only when it exists in items', () => {
    expect(
      normalizeCard(
        card('demo-1', {
          items: [item('p1')],
          featured: { kind: 'project', projectId: 'p2' },
        }),
      ).featured,
    ).toBeNull()
    expect(
      normalizeCard(
        card('demo-1', {
          items: [item('p1')],
          featured: { kind: 'project', projectId: 'p1' },
        }),
      ).featured,
    ).toEqual({ kind: 'project', projectId: 'p1' })
  })

  it('trims and deduplicates tags', () => {
    expect(normalizeCard(card('demo-1', { tags: [' Vue ', 'Vue', '', 'Go'] })).tags).toEqual([
      'Vue',
      'Go',
    ])
  })
})

describe('decode', () => {
  it('round-trips a card and rejects broken json', () => {
    const source = card('demo-1', { headline: 'hello' })
    expect(decodeCard(JSON.stringify(source))?.headline).toBe('hello')
    expect(decodeCard('{oops')).toBeNull()
    expect(decodeCard(null)).toBeNull()
  })

  it('decodes a card list and skips invalid entries', () => {
    const raw = JSON.stringify([card('demo-1'), card('demo-2'), null, 'x'])
    expect(decodeCards(raw).map((entry) => entry.id)).toEqual(['demo-1', 'demo-2'])
    expect(decodeCards('[]')).toEqual([])
  })
})

describe('sortDiscoverCards', () => {
  it('puts the own card first', () => {
    const cards = [card('demo-1'), card('me'), card('demo-2')]
    expect(sortDiscoverCards(cards).map((entry) => entry.id)).toEqual(['me', 'demo-2', 'demo-1'])
  })
})

describe('validateCardUpload', () => {
  it('accepts pdf and images under 2MB', () => {
    expect(validateCardUpload({ name: 'a.pdf', size: 10, type: 'application/pdf' })).toBe('')
    expect(validateCardUpload({ name: 'a.png', size: 10, type: 'image/png' })).toBe('')
  })

  it('rejects other types and oversized files', () => {
    expect(validateCardUpload({ name: 'a.zip', size: 10, type: 'application/zip' })).toContain('PDF')
    expect(
      validateCardUpload({ name: 'a.png', size: 3 * 1024 * 1024, type: 'image/png' }),
    ).toContain('2MB')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/utils/card.test.ts`
Expected: FAIL，提示无法解析 `./card`.

- [ ] **Step 3: Add types**

在 `packages/web/src/types/index.ts` 追加：

```ts
export type CardContentType = 'html' | 'image' | 'pdf' | 'video' | 'document'

export interface CardProjectItem {
  projectId: string
  name: string
  description: string
  coverUrl: string | null
  type: CardContentType
  shareUrl: string
  previewUrl?: string
  addedAt: string
}

export type CardFeatured =
  | { kind: 'project'; projectId: string }
  | {
      kind: 'file'
      fileType: 'pdf' | 'image'
      name: string
      size: number
      dataUrl: string
    }
  | null

export interface BusinessCard {
  id: string
  ownerUserId: number | null
  ownerName: string
  avatarUrl: string | null
  headline: string
  tags: string[]
  items: CardProjectItem[]
  featured: CardFeatured
  published: boolean
  commentCount: number
  createdAt: string
  updatedAt: string
}
```

- [ ] **Step 4: Implement `src/utils/card.ts`**

```ts
import type { BusinessCard, CardContentType, CardFeatured, CardProjectItem } from '@/types'

export const CARD_MAX_ITEMS = 5
export const CARD_MAX_TAGS = 5
export const CARD_MAX_TAG_LENGTH = 12
export const CARD_MAX_UPLOAD_BYTES = 2 * 1024 * 1024

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
    ? card.items.filter((item) => item && typeof item.projectId === 'string').slice(0, CARD_MAX_ITEMS)
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
    return b.updatedAt.localeCompare(a.updatedAt)
  })
}

export function validateCardUpload(file: { name: string; size: number; type: string }): string {
  const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  const isImage = file.type.startsWith('image/')
  if (!isPdf && !isImage) return '只支持 PDF 或图片'
  if (file.size <= 0 || file.size > CARD_MAX_UPLOAD_BYTES) return '文件不能超过 2MB'
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/utils/card.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/types/index.ts packages/web/src/utils/card.ts packages/web/src/utils/card.test.ts
git commit -m "feat(web): add business card types and pure utilities"
```

---

### Task 2: Mock 名片与名片 Store

**Files:**
- Create: `packages/web/src/mocks/cards.ts`
- Create: `packages/web/src/stores/card.ts`
- Test: `packages/web/src/stores/card.test.ts`

**Interfaces:**
- Consumes: Task 1 类型与工具；`getProject(id)`（`@/api/projects`）；`useAuthStore`。
- Produces:
  - `mockCards: BusinessCard[]`
  - `DEMO_HTML`、`DEMO_IMAGE`、`DEMO_PDF`、`DEMO_VIDEO`、`DEMO_DOCUMENT`
  - `useCardStore()` 返回 `mine`、`discoverMock`、`loading`、`saving`、`error`、`items`、`hasShareContent`、`discoverCards`、`load()`、`setHeadline(value)`、`addTag(raw): string`、`removeTag(tag)`、`removeItem(projectId)`、`addPublishedProject(published): Promise<CardProjectItem>`、`setFeatured(featured)`、`setFeaturedFile(file): Promise<void>`、`clearFeatured()`、`publish(): BusinessCard`、`getCardById(id)`、`findProject(projectId)`、`listDiscover(page, pageSize?)`

- [ ] **Step 1: Write the failing store test**

Create `packages/web/src/stores/card.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { mockCards } from '@/mocks/cards'
import { useCardStore } from './card'
import type { CardProjectItem } from '@/types'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 7, username: 'alice', avatarUrl: null } }),
}))

vi.mock('@/api/projects', () => ({
  getProject: vi.fn().mockResolvedValue({ previewPath: 'docs/demo.pdf' }),
}))

vi.mock('@/api/publish', () => ({
  listPublishedProjects: vi.fn().mockResolvedValue([]),
}))

function seedItem(store: ReturnType<typeof useCardStore>, projectId: string) {
  const item: CardProjectItem = {
    projectId,
    name: projectId,
    description: '',
    coverUrl: null,
    type: 'html',
    shareUrl: `/p/${projectId}`,
    addedAt: '2026-11-08T00:00:00.000Z',
  }
  store.$patch({ mine: { ...store.mine, items: [...store.mine.items, item] } })
}

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})

describe('card store', () => {
  it('initializes an empty own card and 12 mock cards', () => {
    const store = useCardStore()
    store.load()
    expect(store.mine.id).toBe('me')
    expect(store.mine.ownerName).toBe('alice')
    expect(store.mine.items).toEqual([])
    expect(store.discoverCards.filter((card) => card.id.startsWith('demo-'))).toHaveLength(
      mockCards.length,
    )
  })

  it('adds, deduplicates and limits tags', () => {
    const store = useCardStore()
    store.load()
    expect(store.addTag('Vue')).toBe('')
    expect(store.addTag('Vue')).toBe('标签已存在')
    for (const tag of ['Go', 'Rust', 'SQL', 'AI']) store.addTag(tag)
    expect(store.addTag('Next')).toContain('最多')
    expect(store.mine.tags).toEqual(['Vue', 'Go', 'Rust', 'SQL', 'AI'])
  })

  it('adds a published project with preview type from project detail', async () => {
    const store = useCardStore()
    store.load()
    const item = await store.addPublishedProject({
      projectId: 'p1',
      name: '作品一',
      description: '介绍',
      tags: [],
      coverUrl: null,
      publishedAt: '2026-11-08T00:00:00.000Z',
      url: '/p/p1',
    })
    expect(item.type).toBe('pdf')
    expect(store.mine.items).toHaveLength(1)
  })

  it('clears featured when its project is removed', () => {
    const store = useCardStore()
    store.load()
    seedItem(store, 'p1')
    store.setFeatured({ kind: 'project', projectId: 'p1' })
    store.removeItem('p1')
    expect(store.mine.featured).toBeNull()
    expect(store.mine.items).toEqual([])
  })

  it('requires items before publishing and persists the card', () => {
    const store = useCardStore()
    store.load()
    expect(() => store.publish()).toThrow('请先添加项目')
    seedItem(store, 'p1')
    store.publish()
    expect(store.mine.published).toBe(true)
    expect(localStorage.getItem('vibebiz:cards:mine:v1')).toContain('"published":true')
  })

  it('finds a project from the own card and paginates discover cards', () => {
    const store = useCardStore()
    store.load()
    seedItem(store, 'p1')
    expect(store.findProject('p1')?.item.projectId).toBe('p1')
    const first = store.listDiscover(1, 6)
    expect(first.items).toHaveLength(6)
    expect(first.total).toBe(mockCards.length)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/stores/card.test.ts`
Expected: FAIL，找不到 `@/mocks/cards` / `./card`.

- [ ] **Step 3: Implement mock cards**

Create `packages/web/src/mocks/cards.ts`:

```ts
import type { BusinessCard, CardContentType, CardProjectItem } from '@/types'

const TIMESTAMP = '2026-11-08T10:00:00.000Z'

export const DEMO_IMAGE =
  'data:image/svg+xml;charset=utf-8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#111111"/><stop offset="1" stop-color="#2563eb"/></linearGradient></defs><rect width="640" height="360" fill="url(#g)"/><text x="320" y="190" text-anchor="middle" fill="#ffffff" font-size="34" font-family="sans-serif">Vibebiz Card</text></svg>',
  )

export const DEMO_PDF =
  'data:application/pdf;base64,JVBERi0xLjAKMSAwIG9iajw8L1R5cGUvQ2F0YWxvZy9QYWdlcyAyIDAgUj4+ZW5kb2JqCjIgMCBvYmo8PC9UeXBlL1BhZ2VzL0tpZHNbMyAwIFJdL0NvdW50IDE+PmVuZG9iagozIDAgb2JqPDwvVHlwZS9QYWdlL1BhcmVudCAyIDAgUi9NZWRpYUJveFswIDAgMjAwIDIwMF0+PmVuZG9iagp4cmVmCjAgNAowMDAwMDAwMDAwIDY1NTM1IGYgCjAwMDAwMDAwMDkgMDAwMDAgbiAKMDAwMDAwMDA1MiAwMDAwMCBuIAowMDAwMDAwMTAxIDAwMDAwIG4gCnRyYWlsZXI8PC9Sb290IDEgMCBSPj4Kc3RhcnR4cmVmCjE2MAolJUVPRgo='

export const DEMO_VIDEO =
  'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4'

export const DEMO_DOCUMENT = [
  '# 演示文档',
  '',
  '这是一个名片项目的 Markdown 文档预览。',
  '',
  '## 能力',
  '',
  '- 图片与 PDF 预览',
  '- 视频播放',
  '- HTML 游戏',
  '- 文档阅读',
].join('\n')

export const DEMO_HTML = [
  '<!doctype html><html><head><meta charset="utf-8">',
  '<style>body{margin:0;height:100vh;display:grid;place-items:center;background:#111;color:#fff;font-family:sans-serif}',
  'button{padding:12px 24px;border:0;border-radius:999px;background:#2563eb;color:#fff;font-size:16px;cursor:pointer}</style>',
  '</head><body><div><h1>点击小游戏</h1>',
  '<button id="b">点我 +1</button><p id="s">得分：0</p></div>',
  '<script>let n=0;const s=document.getElementById("s");document.getElementById("b").onclick=function(){n+=1;s.textContent="得分："+n}</script>',
  '</body></html>',
].join('')

const PEOPLE = [
  { name: '林一', headline: '前端工程师 · 喜欢把想法做成小游戏', tags: ['Vue', '游戏', '开源'] },
  { name: '周舟', headline: '独立开发者 · 正在做 AI 工具', tags: ['AI', 'Go', '产品'] },
  { name: '陈默', headline: '设计工程师 · 关注像素与动效', tags: ['设计', 'CSS', '动效'] },
  { name: '苏白', headline: '全栈开发 · 数据可视化爱好者', tags: ['全栈', '可视化', 'D3'] },
  { name: '顾言', headline: '后端工程师 · 分布式系统', tags: ['Go', '架构', 'MySQL'] },
  { name: '叶青', headline: '产品经理 · 会写一点代码', tags: ['产品', '增长', 'SQL'] },
  { name: '沈知', headline: '算法工程师 · NLP 方向', tags: ['NLP', 'Python', '论文'] },
  { name: '许川', headline: '游戏开发者 · WebGL', tags: ['WebGL', '游戏', 'Three'] },
  { name: '陆离', headline: '开源维护者 · 工具链', tags: ['开源', 'Rust', 'CLI'] },
  { name: '唐糖', headline: '前端新人 · 记录学习', tags: ['前端', '学习', '笔记'] },
  { name: '韩野', headline: 'SRE · 可观测性', tags: ['SRE', '监控', 'Go'] },
  { name: '白露', headline: '插画师 · 也会写代码', tags: ['插画', 'SVG', '创意'] },
]

const TYPE_ROTATION: CardContentType[] = ['html', 'image', 'pdf', 'video', 'document']

function previewFor(type: CardContentType): string | undefined {
  if (type === 'image') return DEMO_IMAGE
  if (type === 'pdf') return DEMO_PDF
  if (type === 'video') return DEMO_VIDEO
  if (type === 'document') return DEMO_DOCUMENT
  return undefined
}

function demoItem(
  projectId: string,
  name: string,
  type: CardContentType,
  description: string,
): CardProjectItem {
  return {
    projectId,
    name,
    description,
    coverUrl: DEMO_IMAGE,
    type,
    shareUrl: `/share/project/${projectId}`,
    previewUrl: previewFor(type),
    addedAt: TIMESTAMP,
  }
}

export const mockCards: BusinessCard[] = PEOPLE.map((person, index) => {
  const id = `demo-${index + 1}`
  const count = 2 + (index % 4)
  const items = Array.from({ length: count }, (_, itemIndex) => {
    const type = TYPE_ROTATION[(index + itemIndex) % TYPE_ROTATION.length] ?? 'html'
    return demoItem(
      `${id}-p${itemIndex + 1}`,
      `${person.name}的作品 ${itemIndex + 1}`,
      type,
      `${person.name} 的${type === 'html' ? '互动小游戏' : type === 'document' ? '技术文档' : '创作项目'}`,
    )
  })
  return {
    id,
    ownerUserId: null,
    ownerName: person.name,
    avatarUrl: null,
    headline: person.headline,
    tags: person.tags,
    items,
    featured: items[0] ? { kind: 'project', projectId: items[0].projectId } : null,
    published: true,
    commentCount: 12 + index * 7,
    createdAt: TIMESTAMP,
    updatedAt: new Date(Date.parse(TIMESTAMP) - index * 3_600_000).toISOString(),
  }
})
```

- [ ] **Step 4: Implement the store**

Create `packages/web/src/stores/card.ts`:

```ts
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getProject } from '@/api/projects'
import { mockCards } from '@/mocks/cards'
import { useAuthStore } from '@/stores/auth'
import type {
  BusinessCard,
  CardContentType,
  CardFeatured,
  CardProjectItem,
  PublishedProject,
} from '@/types'
import {
  addCardTag,
  canAddCardItem,
  cardContentType,
  decodeCard,
  decodeCards,
  emptyCard,
  normalizeCard,
  pageCards,
  readFileAsDataUrl,
  sortDiscoverCards,
  validateCardUpload,
} from '@/utils/card'

const MINE_KEY = 'vibebiz:cards:mine:v1'
const DISCOVER_KEY = 'vibebiz:cards:discover:v1'

export const useCardStore = defineStore('card', () => {
  const mine = ref<BusinessCard>(emptyCard())
  const discoverMock = ref<BusinessCard[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  let saveTimer: number | undefined

  const items = computed(() => mine.value.items)
  const hasShareContent = computed(() => mine.value.items.length > 0)
  const discoverCards = computed(() =>
    sortDiscoverCards([...(mine.value.published ? [mine.value] : []), ...discoverMock.value]),
  )

  function persistMine() {
    try {
      localStorage.setItem(MINE_KEY, JSON.stringify(mine.value))
      error.value = ''
    } catch {
      error.value = '本地存储空间不足，名片未能保存'
    }
  }

  function persistDiscover() {
    try {
      localStorage.setItem(DISCOVER_KEY, JSON.stringify(discoverMock.value))
    } catch {
      error.value = '本地存储空间不足，发现页数据未能保存'
    }
  }

  function saveNow() {
    saving.value = true
    mine.value = { ...mine.value, updatedAt: new Date().toISOString() }
    persistMine()
    saving.value = false
  }

  function scheduleSave() {
    if (saveTimer) window.clearTimeout(saveTimer)
    saveTimer = window.setTimeout(saveNow, 300)
  }

  function load() {
    loading.value = true
    const auth = useAuthStore()
    const stored = decodeCard(localStorage.getItem(MINE_KEY)) ?? emptyCard()
    mine.value = normalizeCard({
      ...stored,
      ownerUserId: auth.user?.id ?? stored.ownerUserId,
      ownerName: auth.user?.username ?? stored.ownerName,
      avatarUrl: auth.user?.avatarUrl ?? stored.avatarUrl,
    })
    const storedMock = decodeCards(localStorage.getItem(DISCOVER_KEY))
    discoverMock.value = storedMock.length
      ? storedMock
      : mockCards.map((card) => normalizeCard(card))
    if (!storedMock.length) persistDiscover()
    loading.value = false
  }

  function setHeadline(value: string) {
    mine.value = { ...mine.value, headline: value.slice(0, 80) }
    scheduleSave()
  }

  function addTag(raw: string): string {
    const result = addCardTag(mine.value.tags, raw)
    if (!result.error) {
      mine.value = { ...mine.value, tags: result.tags }
      saveNow()
    }
    return result.error
  }

  function removeTag(tag: string) {
    mine.value = { ...mine.value, tags: mine.value.tags.filter((item) => item !== tag) }
    saveNow()
  }

  function removeItem(projectId: string) {
    const nextItems = mine.value.items.filter((item) => item.projectId !== projectId)
    const featured =
      mine.value.featured?.kind === 'project' && mine.value.featured.projectId === projectId
        ? null
        : mine.value.featured
    mine.value = { ...mine.value, items: nextItems, featured }
    saveNow()
  }

  async function addPublishedProject(published: PublishedProject): Promise<CardProjectItem> {
    if (!canAddCardItem(mine.value.items, published.projectId)) {
      throw new Error('项目已添加或已达到 5 个上限')
    }
    let type: CardContentType = 'html'
    try {
      const project = await getProject(published.projectId)
      type = cardContentType(project.previewPath)
    } catch {
      type = 'html'
    }
    const item: CardProjectItem = {
      projectId: published.projectId,
      name: published.name,
      description: published.description,
      coverUrl: published.coverUrl,
      type,
      shareUrl: published.url,
      addedAt: new Date().toISOString(),
    }
    mine.value = { ...mine.value, items: [...mine.value.items, item] }
    saveNow()
    return item
  }

  function setFeatured(featured: CardFeatured) {
    if (featured?.kind === 'project') {
      const exists = mine.value.items.some((item) => item.projectId === featured.projectId)
      if (!exists) throw new Error('主预览项目必须来自已添加项目')
    }
    mine.value = { ...mine.value, featured }
    saveNow()
  }

  async function setFeaturedFile(file: File): Promise<void> {
    const message = validateCardUpload(file)
    if (message) throw new Error(message)
    const dataUrl = await readFileAsDataUrl(file)
    const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
    setFeatured({
      kind: 'file',
      fileType: isPdf ? 'pdf' : 'image',
      name: file.name,
      size: file.size,
      dataUrl,
    })
  }

  function clearFeatured() {
    mine.value = { ...mine.value, featured: null }
    saveNow()
  }

  function publish(): BusinessCard {
    if (!hasShareContent.value) throw new Error('请先添加项目')
    mine.value = { ...mine.value, published: true }
    saveNow()
    return mine.value
  }

  function getCardById(id: string): BusinessCard | null {
    if (id === 'me') return mine.value
    return discoverMock.value.find((card) => card.id === id) ?? null
  }

  function findProject(projectId: string): { card: BusinessCard; item: CardProjectItem } | null {
    for (const card of [mine.value, ...discoverMock.value]) {
      const item = card.items.find((entry) => entry.projectId === projectId)
      if (item) return { card, item }
    }
    return null
  }

  function listDiscover(page: number, pageSize = 6): { items: BusinessCard[]; total: number } {
    const all = discoverCards.value
    return { items: pageCards(all, page, pageSize), total: all.length }
  }

  return {
    mine,
    discoverMock,
    loading,
    saving,
    error,
    items,
    hasShareContent,
    discoverCards,
    load,
    setHeadline,
    addTag,
    removeTag,
    removeItem,
    addPublishedProject,
    setFeatured,
    setFeaturedFile,
    clearFeatured,
    publish,
    getCardById,
    findProject,
    listDiscover,
  }
})
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/stores/card.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/mocks/cards.ts packages/web/src/stores/card.ts packages/web/src/stores/card.test.ts
git commit -m "feat(web): add card mock data and store"
```

---

### Task 3: 内容模块组件与添加项目弹窗

**Files:**
- Create: `packages/web/src/components/card/CardProjectItem.vue`
- Create: `packages/web/src/components/card/CardPreview.vue`
- Create: `packages/web/src/components/card/AddProjectDialog.vue`
- Test: `packages/web/src/components/card/AddProjectDialog.test.ts`

**Interfaces:**
- Consumes: Task 1 `CARD_TYPE_LABELS`、Task 2 `DEMO_IMAGE`、`listPublishedProjects()`。
- Produces:
  - `CardProjectItem` props `{ item: CardProjectItem }`；emits `open(item)`、`remove(projectId)`。
  - `CardPreview` props `{ featured: CardFeatured; items: CardProjectItem[]; large?: boolean }`。
  - `AddProjectDialog` props `{ existingIds: string[]; full: boolean }`；emits `close`、`add(published: PublishedProject)`。

- [ ] **Step 1: Write the failing dialog test**

Create `packages/web/src/components/card/AddProjectDialog.test.ts`:

```ts
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { listPublishedProjects } from '@/api/publish'
import AddProjectDialog from './AddProjectDialog.vue'

vi.mock('@/api/publish', () => ({ listPublishedProjects: vi.fn() }))

const listMock = vi.mocked(listPublishedProjects)
let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount(props: Record<string, unknown>) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(AddProjectDialog, props)
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

const published = (id: string, name: string) => ({
  projectId: id,
  name,
  description: '介绍',
  tags: [],
  coverUrl: null,
  publishedAt: '2026-11-08T00:00:00.000Z',
  url: `/p/${id}`,
})

describe('AddProjectDialog', () => {
  it('lists published projects and disables already added rows', async () => {
    listMock.mockResolvedValue([published('p1', '作品一'), published('p2', '作品二')])
    mount({ existingIds: ['p1'], full: false })
    await flush()
    expect(host.textContent).toContain('作品一')
    expect(host.textContent).toContain('作品二')
    const buttons = host.querySelectorAll<HTMLButtonElement>('.add-project-action')
    expect(buttons[0]?.disabled).toBe(true)
    expect(buttons[1]?.disabled).toBe(false)
  })

  it('disables every row when the card is full', async () => {
    listMock.mockResolvedValue([published('p3', '作品三')])
    mount({ existingIds: [], full: true })
    await flush()
    expect(host.querySelector<HTMLButtonElement>('.add-project-action')?.disabled).toBe(true)
  })

  it('shows the empty state', async () => {
    listMock.mockResolvedValue([])
    mount({ existingIds: [], full: false })
    await flush()
    expect(host.textContent).toContain('暂无已发布项目')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/components/card/AddProjectDialog.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 3: Implement `CardProjectItem.vue`**

```vue
<script setup lang="ts">
import { CARD_TYPE_LABELS } from '@/utils/card'
import type { CardProjectItem } from '@/types'

const props = defineProps<{ item: CardProjectItem }>()
const emit = defineEmits<{
  open: [item: CardProjectItem]
  remove: [projectId: string]
}>()
</script>

<template>
  <article class="card-project-item" @click="emit('open', props.item)">
    <div class="card-project-cover">
      <img v-if="item.coverUrl" :src="item.coverUrl" :alt="item.name" />
      <span v-else class="card-project-placeholder"><i class="pi pi-image" /></span>
    </div>
    <div class="card-project-body">
      <div class="card-project-title">
        <strong>{{ item.name }}</strong>
        <span class="tag">{{ CARD_TYPE_LABELS[item.type] }}</span>
      </div>
      <p>{{ item.description || '暂无简介' }}</p>
    </div>
    <button
      class="card-project-remove"
      type="button"
      aria-label="移除项目"
      @click.stop="emit('remove', item.projectId)"
    >
      <i class="pi pi-times" />
    </button>
  </article>
</template>
```

- [ ] **Step 4: Implement `CardPreview.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'

import { DEMO_IMAGE } from '@/mocks/cards'
import type { CardFeatured, CardProjectItem } from '@/types'

const props = withDefaults(
  defineProps<{ featured: CardFeatured; items: CardProjectItem[]; large?: boolean }>(),
  { large: false },
)

const resolved = computed(() => {
  const featured = props.featured
  if (featured?.kind === 'file') {
    return { kind: featured.fileType, src: featured.dataUrl, name: featured.name }
  }
  if (featured?.kind === 'project') {
    const projectId = featured.projectId
    const item = props.items.find((entry) => entry.projectId === projectId)
    if (!item) return null
    return { kind: 'image' as const, src: item.coverUrl || item.previewUrl || DEMO_IMAGE, name: item.name }
  }
  return null
})
</script>

<template>
  <div class="card-preview" :class="{ large }">
    <template v-if="resolved">
      <div v-if="large && resolved.kind === 'pdf'" class="card-preview-pdf">
        <iframe :src="resolved.src" title="PDF 预览" />
        <span>{{ resolved.name }}</span>
      </div>
      <img v-else-if="resolved.kind === 'image'" :src="resolved.src" :alt="resolved.name" />
      <div v-else class="card-preview-file">
        <i class="pi pi-file-pdf" />
        <span>{{ resolved.name }}</span>
      </div>
    </template>
    <div v-else class="card-preview-empty">
      <i class="pi pi-image" />
      <span>还没有主预览</span>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Implement `AddProjectDialog.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { listPublishedProjects } from '@/api/publish'
import type { PublishedProject } from '@/types'

const props = defineProps<{ existingIds: string[]; full: boolean }>()
const emit = defineEmits<{ close: []; add: [project: PublishedProject] }>()

const projects = ref<PublishedProject[]>([])
const loading = ref(true)
const error = ref('')
const existing = computed(() => new Set(props.existingIds))

function disabled(project: PublishedProject): boolean {
  return props.full || existing.value.has(project.projectId)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    projects.value = await listPublishedProjects()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载已发布项目失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <div class="dialog-card add-project-dialog">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">内容模块</p>
          <h2>添加已发布项目</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <p class="add-project-note">只能添加自己已经发布的项目，最多 5 个。</p>
      <p v-if="full" class="add-project-note">已达到 5 个上限，请先移除一个项目。</p>
      <div v-if="loading" class="dialog-state">加载中...</div>
      <div v-else-if="error" class="dialog-state">
        <span class="error-text">{{ error }}</span>
        <button class="dialog-retry" type="button" @click="load">重试</button>
      </div>
      <p v-else-if="projects.length === 0" class="dialog-state">暂无已发布项目</p>
      <ul v-else class="add-project-list">
        <li v-for="project in projects" :key="project.projectId" class="add-project-row">
          <img v-if="project.coverUrl" class="add-project-cover" :src="project.coverUrl" :alt="project.name" />
          <span v-else class="add-project-cover placeholder"><i class="pi pi-image" /></span>
          <div class="add-project-info">
            <strong>{{ project.name }}</strong>
            <p>{{ project.description || '暂无简介' }}</p>
          </div>
          <button
            class="add-project-action"
            type="button"
            :disabled="disabled(project)"
            @click="emit('add', project)"
          >
            {{ existing.has(project.projectId) ? '已添加' : '添加' }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/components/card/AddProjectDialog.test.ts`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add packages/web/src/components/card/CardProjectItem.vue packages/web/src/components/card/CardPreview.vue packages/web/src/components/card/AddProjectDialog.vue packages/web/src/components/card/AddProjectDialog.test.ts
git commit -m "feat(web): add card project list components"
```

---

### Task 4: 个人信息模块

**Files:**
- Create: `packages/web/src/components/card/CardProfilePanel.vue`
- Test: `packages/web/src/components/card/CardProfilePanel.test.ts`

**Interfaces:**
- Consumes: Task 1 `CARD_MAX_TAGS`、`validateCardUpload`、`readFileAsDataUrl`；Task 3 `CardPreview`。
- Produces: props `{ card: BusinessCard }`；emits `update:headline(string)`、`add-tag(string)`、`remove-tag(string)`、`set-featured(CardFeatured)`、`clear-featured`。

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/components/card/CardProfilePanel.test.ts`:

```ts
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import CardProfilePanel from './CardProfilePanel.vue'
import { emptyCard } from '@/utils/card'
import type { BusinessCard, CardProjectItem } from '@/types'

let app: App | null = null
let host: HTMLElement

const item: CardProjectItem = {
  projectId: 'p1',
  name: '作品一',
  description: '',
  coverUrl: null,
  type: 'html',
  shareUrl: '/p/p1',
  addedAt: '2026-11-08T00:00:00.000Z',
}

function mount(card: BusinessCard) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardProfilePanel, { card })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardProfilePanel', () => {
  it('renders the owner, headline and tags', () => {
    mount({ ...emptyCard(), ownerName: 'alice', headline: '前端工程师', tags: ['Vue', 'Go'] })
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('Go')
  })

  it('shows the project selector only when items exist', () => {
    mount({ ...emptyCard(), items: [item] })
    expect(host.querySelector('select')?.textContent).toContain('作品一')
    app?.unmount()
    host.remove()
    mount({ ...emptyCard(), items: [] })
    expect(host.querySelector('select')).toBeNull()
  })

  it('renders an uploaded pdf as a file preview', () => {
    mount({
      ...emptyCard(),
      featured: {
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 1024,
        dataUrl: 'data:application/pdf;base64,AA==',
      },
    })
    expect(host.textContent).toContain('resume.pdf')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/components/card/CardProfilePanel.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 3: Implement `CardProfilePanel.vue`**

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'

import CardPreview from './CardPreview.vue'
import type { BusinessCard, CardFeatured } from '@/types'
import { CARD_MAX_TAGS, readFileAsDataUrl, validateCardUpload } from '@/utils/card'

const props = defineProps<{ card: BusinessCard }>()
const emit = defineEmits<{
  'update:headline': [value: string]
  'add-tag': [raw: string]
  'remove-tag': [tag: string]
  'set-featured': [featured: CardFeatured]
  'clear-featured': []
}>()

const tagInput = ref('')
const tagError = ref('')
const uploadError = ref('')
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const initial = computed(() => (props.card.ownerName || '?').charAt(0).toUpperCase())
const tagsFull = computed(() => props.card.tags.length >= CARD_MAX_TAGS)
const selectedProjectId = computed(() =>
  props.card.featured?.kind === 'project' ? props.card.featured.projectId : '',
)

function submitTag() {
  const value = tagInput.value.trim()
  if (!value) return
  if (props.card.tags.includes(value)) {
    tagError.value = '标签已存在'
    return
  }
  if (props.card.tags.length >= CARD_MAX_TAGS) {
    tagError.value = '最多 5 个标签'
    return
  }
  tagError.value = ''
  emit('add-tag', value)
  tagInput.value = ''
}

function onTagKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',' || event.key === '，') {
    event.preventDefault()
    submitTag()
  }
}

function selectProject(event: Event) {
  const projectId = (event.target as HTMLSelectElement).value
  if (projectId) emit('set-featured', { kind: 'project', projectId })
}

function openUpload() {
  uploadError.value = ''
  fileInput.value?.click()
}

async function onFileChange(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  const message = validateCardUpload(file)
  if (message) {
    uploadError.value = message
    return
  }
  uploading.value = true
  uploadError.value = ''
  try {
    const dataUrl = await readFileAsDataUrl(file)
    const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
    emit('set-featured', {
      kind: 'file',
      fileType: isPdf ? 'pdf' : 'image',
      name: file.name,
      size: file.size,
      dataUrl,
    })
  } catch (err) {
    uploadError.value = err instanceof Error ? err.message : '文件读取失败'
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}
</script>

<template>
  <section class="card-profile">
    <header class="card-profile-head">
      <img v-if="card.avatarUrl" class="card-profile-avatar" :src="card.avatarUrl" :alt="card.ownerName" />
      <span v-else class="card-profile-avatar">{{ initial }}</span>
      <div>
        <strong>{{ card.ownerName || '未命名开发者' }}</strong>
        <p>个人信息</p>
      </div>
    </header>

    <label class="card-field">
      <span>一句话简介</span>
      <input
        :value="card.headline"
        maxlength="80"
        placeholder="例如：前端工程师，喜欢做小游戏"
        @input="emit('update:headline', ($event.target as HTMLInputElement).value)"
      />
    </label>

    <div class="card-field">
      <span>主预览</span>
      <CardPreview :featured="card.featured" :items="card.items" large />
      <div class="card-featured-actions">
        <select
          v-if="card.items.length > 0"
          class="card-select"
          :value="selectedProjectId"
          @change="selectProject"
        >
          <option value="">从已添加项目选择</option>
          <option v-for="item in card.items" :key="item.projectId" :value="item.projectId">
            {{ item.name }}
          </option>
        </select>
        <button type="button" class="card-secondary" :disabled="uploading" @click="openUpload">
          <i :class="uploading ? 'pi pi-spin pi-spinner' : 'pi pi-upload'" />
          {{ uploading ? '读取中…' : '上传 PDF / 图片' }}
        </button>
        <button v-if="card.featured" type="button" class="card-text-btn" @click="emit('clear-featured')">
          移除主预览
        </button>
      </div>
      <input ref="fileInput" class="card-file-input" type="file" accept="application/pdf,image/*" @change="onFileChange" />
      <p v-if="uploadError" class="error-text" role="alert">{{ uploadError }}</p>
      <small>只能从已添加项目中选择，或上传 2MB 以内的 PDF / 图片。</small>
    </div>

    <div class="card-field">
      <span>标签</span>
      <div class="card-tags">
        <span v-for="tag in card.tags" :key="tag" class="card-tag">
          {{ tag }}
          <button type="button" :aria-label="`删除标签 ${tag}`" @click="emit('remove-tag', tag)">
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <input
        v-model="tagInput"
        :disabled="tagsFull"
        maxlength="12"
        :placeholder="tagsFull ? '最多 5 个标签' : '输入标签后回车'"
        @keydown="onTagKeydown"
        @blur="submitTag"
      />
      <p v-if="tagError" class="error-text" role="alert">{{ tagError }}</p>
      <small>最多 5 个标签，每个不超过 12 个字。</small>
    </div>
  </section>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/components/card/CardProfilePanel.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/web/src/components/card/CardProfilePanel.vue packages/web/src/components/card/CardProfilePanel.test.ts
git commit -m "feat(web): add card profile panel"
```

---

### Task 5: 我的名片编辑页

**Files:**
- Create: `packages/web/src/components/card/MyCardPanel.vue`
- Test: `packages/web/src/components/card/MyCardPanel.test.ts`

**Interfaces:**
- Consumes: Task 2 store；Task 3 `AddProjectDialog`、`CardProjectItem`；Task 4 `CardProfilePanel`；`useAuthStore`。
- Produces: `MyCardPanel.vue` 无 props，包含 3:1 布局、添加项目、分享弹窗。

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/components/card/MyCardPanel.test.ts`:

```ts
import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import MyCardPanel from './MyCardPanel.vue'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 7, username: 'alice', avatarUrl: null },
    ensure: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/api/publish', () => ({ listPublishedProjects: vi.fn().mockResolvedValue([]) }))

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(MyCardPanel)
  app.use(createPinia())
  app.mount(host)
}

beforeEach(() => localStorage.clear())
afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('MyCardPanel', () => {
  it('renders the content and profile modules', async () => {
    mount()
    await flush()
    expect(host.textContent).toContain('我的项目')
    expect(host.textContent).toContain('个人信息')
    expect(host.querySelector('.card-content-panel')).not.toBeNull()
    expect(host.querySelector('.card-profile')).not.toBeNull()
  })

  it('disables sharing until a project exists', async () => {
    mount()
    await flush()
    expect(host.querySelector<HTMLButtonElement>('.card-share-btn')?.disabled).toBe(true)
    expect(host.textContent).toContain('请先添加项目')
  })

  it('opens the add project dialog', async () => {
    mount()
    await flush()
    host.querySelector<HTMLButtonElement>('.card-add-btn')?.click()
    await flush()
    expect(host.textContent).toContain('添加已发布项目')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/components/card/MyCardPanel.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 3: Implement `MyCardPanel.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import AddProjectDialog from './AddProjectDialog.vue'
import CardProfilePanel from './CardProfilePanel.vue'
import CardProjectItem from './CardProjectItem.vue'
import { useAuthStore } from '@/stores/auth'
import { useCardStore } from '@/stores/card'
import type { CardProjectItem as CardProjectItemType, PublishedProject } from '@/types'
import '@/styles/card.css'

const auth = useAuthStore()
const store = useCardStore()
const addOpen = ref(false)
const shareOpen = ref(false)
const copied = ref(false)
const actionError = ref('')
const shareUrl = computed(() => `${window.location.origin}/share/card/me`)

onMounted(async () => {
  await auth.ensure().catch(() => undefined)
  store.load()
})

function openProject(item: CardProjectItemType) {
  window.open(`/share/project/${encodeURIComponent(item.projectId)}`, '_blank', 'noopener,noreferrer')
}

async function addProject(published: PublishedProject) {
  actionError.value = ''
  try {
    await store.addPublishedProject(published)
    addOpen.value = false
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '添加项目失败'
  }
}

function share() {
  actionError.value = ''
  try {
    store.publish()
    copied.value = false
    shareOpen.value = true
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '发布失败'
  }
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(shareUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    actionError.value = '复制失败，请手动复制链接'
  }
}
</script>

<template>
  <div class="card-page">
    <header class="card-page-head">
      <div>
        <h1>我的名片</h1>
        <p>{{ store.saving ? '保存中…' : '已保存' }}</p>
      </div>
      <div class="card-page-actions">
        <span v-if="store.error" class="error-text">{{ store.error }}</span>
        <span v-if="!store.hasShareContent" class="card-share-hint">请先添加项目</span>
        <button
          class="card-share-btn"
          type="button"
          :disabled="!store.hasShareContent"
          :title="store.hasShareContent ? '分享名片' : '请先添加项目'"
          @click="share"
        >
          <i class="pi pi-share-alt" />
          分享
        </button>
      </div>
    </header>

    <p v-if="actionError" class="error-text" role="alert">{{ actionError }}</p>

    <div class="card-page-grid">
      <section class="card-content-panel">
        <header class="card-panel-head">
          <div>
            <h2>我的项目</h2>
            <p>从自己已发布的项目中挑选最多 5 个。</p>
          </div>
          <button class="card-add-btn" type="button" :disabled="store.items.length >= 5" @click="addOpen = true">
            <i class="pi pi-plus" />
            添加项目
          </button>
        </header>

        <p v-if="store.items.length === 0" class="card-empty">还没有项目，添加自己已发布的作品吧。</p>
        <div v-else class="card-project-list">
          <CardProjectItem
            v-for="item in store.items"
            :key="item.projectId"
            :item="item"
            @open="openProject"
            @remove="store.removeItem"
          />
        </div>
      </section>

      <CardProfilePanel
        :card="store.mine"
        @update:headline="store.setHeadline"
        @add-tag="store.addTag"
        @remove-tag="store.removeTag"
        @set-featured="store.setFeatured"
        @clear-featured="store.clearFeatured"
      />
    </div>

    <AddProjectDialog
      v-if="addOpen"
      :existing-ids="store.items.map((item) => item.projectId)"
      :full="store.items.length >= 5"
      @close="addOpen = false"
      @add="addProject"
    />

    <div v-if="shareOpen" class="dialog-backdrop" @click.self="shareOpen = false">
      <div class="dialog-card card-share-dialog">
        <header class="dialog-head">
          <div>
            <p class="dialog-eyebrow">分享名片</p>
            <h2>名片分享页已生成</h2>
          </div>
          <button class="dialog-close" type="button" aria-label="关闭" @click="shareOpen = false">
            <i class="pi pi-times" />
          </button>
        </header>
        <p class="card-share-note">把这个链接发给朋友，就能看到你的项目和个人信息。</p>
        <div class="card-share-link">
          <span>{{ shareUrl }}</span>
          <button type="button" @click="copyLink">
            <i class="pi pi-copy" />
            {{ copied ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="card-share-actions">
          <button type="button" class="card-secondary" @click="shareOpen = false">关闭</button>
          <a class="card-primary" :href="shareUrl" target="_blank" rel="noopener noreferrer">打开分享页</a>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/components/card/MyCardPanel.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/web/src/components/card/MyCardPanel.vue packages/web/src/components/card/MyCardPanel.test.ts
git commit -m "feat(web): add my card editor page"
```

---

### Task 6: 发现页

**Files:**
- Create: `packages/web/src/components/card/CardTile.vue`
- Create: `packages/web/src/components/card/DiscoverPanel.vue`
- Test: `packages/web/src/components/card/DiscoverPanel.test.ts`

**Interfaces:**
- Consumes: Task 2 store `load()`、`listDiscover(page, pageSize)`；Task 3 `CardPreview`。
- Produces: `CardTile` props `{ card: BusinessCard }`；emits `open(id)`。`DiscoverPanel` 无 props。

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/components/card/DiscoverPanel.test.ts`:

```ts
import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import DiscoverPanel from './DiscoverPanel.vue'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 7, username: 'alice', avatarUrl: null } }),
}))

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 350))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(DiscoverPanel)
  app.use(createPinia())
  app.mount(host)
}

beforeEach(() => localStorage.clear())
afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('DiscoverPanel', () => {
  it('renders the first page of mock cards', async () => {
    mount()
    await flush()
    expect(host.querySelectorAll('.card-tile')).toHaveLength(6)
    expect(host.textContent).toContain('林一')
  })

  it('loads more cards on click', async () => {
    mount()
    await flush()
    host.querySelector<HTMLButtonElement>('.discover-more')?.click()
    await flush()
    expect(host.querySelectorAll('.card-tile')).toHaveLength(12)
    expect(host.textContent).toContain('已到底')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/components/card/DiscoverPanel.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 3: Implement `CardTile.vue`**

```vue
<script setup lang="ts">
import CardPreview from './CardPreview.vue'
import type { BusinessCard } from '@/types'

const props = defineProps<{ card: BusinessCard }>()
const emit = defineEmits<{ open: [id: string] }>()
const initial = (props.card.ownerName || '?').charAt(0).toUpperCase()
</script>

<template>
  <article class="card-tile" @click="emit('open', card.id)">
    <div class="card-tile-preview">
      <CardPreview :featured="card.featured" :items="card.items" />
    </div>
    <div class="card-tile-body">
      <div class="card-tile-person">
        <img v-if="card.avatarUrl" class="card-tile-avatar" :src="card.avatarUrl" :alt="card.ownerName" />
        <span v-else class="card-tile-avatar">{{ initial }}</span>
        <div>
          <strong>{{ card.ownerName }}</strong>
          <p>{{ card.headline || '这个人还没有写简介' }}</p>
        </div>
      </div>
      <div class="card-tile-tags">
        <span v-for="tag in card.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
        <span v-if="card.tags.length > 3" class="tag">+{{ card.tags.length - 3 }}</span>
      </div>
      <footer class="card-tile-foot">
        <span>{{ card.items.length }} 个项目</span>
        <span><i class="pi pi-comment" /> {{ card.commentCount }}</span>
      </footer>
    </div>
  </article>
</template>
```

- [ ] **Step 4: Implement `DiscoverPanel.vue`**

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import CardTile from './CardTile.vue'
import { useCardStore } from '@/stores/card'
import type { BusinessCard } from '@/types'
import '@/styles/card.css'

const store = useCardStore()
const cards = ref<BusinessCard[]>([])
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const error = ref('')

async function loadMore() {
  if (loading.value || finished.value) return
  loading.value = true
  error.value = ''
  try {
    await new Promise((resolve) => setTimeout(resolve, 300))
    const result = store.listDiscover(page.value, 6)
    cards.value = [...cards.value, ...result.items]
    finished.value = cards.value.length >= result.total
    page.value += 1
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载名片失败'
  } finally {
    loading.value = false
  }
}

function open(id: string) {
  window.open(`/share/card/${encodeURIComponent(id)}`, '_blank', 'noopener,noreferrer')
}

const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  store.load()
  void loadMore()
  if (typeof IntersectionObserver !== 'undefined' && sentinel.value) {
    observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) void loadMore()
    })
    observer.observe(sentinel.value)
  }
})

onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div class="discover-page">
    <header class="discover-head">
      <div>
        <h1>发现 · 编码伙伴</h1>
        <p>看看其他开发者正在做什么，找到同频的人。</p>
      </div>
      <span class="tag">{{ cards.length }} 张名片</span>
    </header>

    <p v-if="error" class="error-text">{{ error }}</p>
    <p v-if="!loading && cards.length === 0" class="card-empty">还没有人发布名片。</p>

    <div class="discover-grid">
      <CardTile v-for="card in cards" :key="card.id" :card="card" @open="open" />
    </div>

    <div ref="sentinel" class="discover-sentinel" />
    <div class="discover-more-wrap">
      <button v-if="!finished" class="discover-more" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? '加载中…' : '加载更多' }}
      </button>
      <span v-else class="discover-end">已到底</span>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/web && npm test -- --run src/components/card/DiscoverPanel.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/components/card/CardTile.vue packages/web/src/components/card/DiscoverPanel.vue packages/web/src/components/card/DiscoverPanel.test.ts
git commit -m "feat(web): add card discovery page"
```

---

### Task 7: 侧栏入口、路由与 HomeView 集成

**Files:**
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/views/HomeView.vue`
- Create: `packages/web/src/router/cardRoutes.test.ts`

**Interfaces:**
- Consumes: Task 5 `MyCardPanel`；Task 6 `DiscoverPanel`。
- Produces: `/my-card`、`/discover`、`/share/card/:id`、`/share/project/:projectId` 路由与侧栏入口。

- [ ] **Step 1: Write the failing route test**

Create `packages/web/src/router/cardRoutes.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import router from './index'

describe('card routes', () => {
  it('registers my card and discover routes behind auth', () => {
    expect(router.resolve('/my-card').name).toBe('my-card')
    expect(router.resolve('/discover').name).toBe('discover')
  })

  it('marks share pages public', () => {
    const card = router.resolve('/share/card/me')
    const project = router.resolve('/share/project/demo-1-p1')
    expect(card.name).toBe('card-share')
    expect(card.meta.public).toBe(true)
    expect(project.name).toBe('project-share')
    expect(project.meta.public).toBe(true)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- --run src/router/cardRoutes.test.ts`
Expected: FAIL，路由不存在。

- [ ] **Step 3: Add routes**

修改 `packages/web/src/router/index.ts` 顶部导入：

```ts
import CardShareView from '@/views/CardShareView.vue'
import ProjectShareView from '@/views/ProjectShareView.vue'
```

在 `routes` 数组中追加：

```ts
{ path: '/my-card', name: 'my-card', component: HomeView },
{ path: '/discover', name: 'discover', component: HomeView },
{ path: '/share/card/:id', name: 'card-share', component: CardShareView, meta: { public: true } },
{
  path: '/share/project/:projectId',
  name: 'project-share',
  component: ProjectShareView,
  meta: { public: true },
},
```

`afterEach` 标题分支追加：

```ts
name === 'my-card'
  ? 'Vibebiz · 我的名片'
  : name === 'discover'
    ? 'Vibebiz · 发现'
    : name === 'card-share' || name === 'project-share'
      ? 'Vibebiz · 名片分享'
      : /* 原有分支 */
```

- [ ] **Step 4: Add sidebar entries**

在 `HomeSidebar.vue` 的 `navigation` 数组“首页”后加入：

```ts
{ label: '发现', icon: 'pi pi-compass', route: 'discover', visible: true },
```

在 `<div class="sidebar-footer">` 内、`sidebar-tools` 之前加入：

```vue
<button
  class="sidebar-card-entry"
  type="button"
  aria-label="我的名片"
  :title="collapsed ? '我的名片' : undefined"
  @click="navigate('my-card')"
>
  <i class="pi pi-id-card" />
  <span>我的名片</span>
</button>
```

- [ ] **Step 5: Add HomeView branches**

在 `HomeView.vue` 顶部导入：

```ts
import DiscoverPanel from '@/components/card/DiscoverPanel.vue'
import MyCardPanel from '@/components/card/MyCardPanel.vue'
```

新增计算：

```ts
const isMyCardPage = computed(() => route.name === 'my-card')
const isDiscoverPage = computed(() => route.name === 'discover')
```

在模板 `LeaderboardPanel` 分支后追加：

```vue
<MyCardPanel v-else-if="isMyCardPage" />
<DiscoverPanel v-else-if="isDiscoverPage" />
```

- [ ] **Step 6: Run route test**

Run: `cd packages/web && npm test -- --run src/router/cardRoutes.test.ts`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add packages/web/src/router/index.ts packages/web/src/router/cardRoutes.test.ts packages/web/src/components/home/HomeSidebar.vue packages/web/src/views/HomeView.vue
git commit -m "feat(web): add my card and discover navigation"
```

---

### Task 8: 名片分享页与项目分享页

**Files:**
- Create: `packages/web/src/views/CardShareView.vue`
- Create: `packages/web/src/views/ProjectShareView.vue`
- Test: `packages/web/src/views/CardShareView.test.ts`
- Test: `packages/web/src/views/ProjectShareView.test.ts`

**Interfaces:**
- Consumes: Task 2 store `load()`、`getCardById(id)`、`findProject(projectId)`；Task 2 `DEMO_*`；`renderMarkdown`。
- Produces: 公开分享页组件。

- [ ] **Step 1: Write the failing tests**

Create `packages/web/src/views/CardShareView.test.ts`:

```ts
import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import CardShareView from './CardShareView.vue'

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'demo-1' } }) }))

vi.mock('@/stores/card', () => ({
  useCardStore: () => ({
    load: vi.fn(),
    getCardById: () => ({
      id: 'demo-1',
      ownerUserId: null,
      ownerName: '林一',
      avatarUrl: null,
      headline: '前端工程师',
      tags: ['Vue', '游戏'],
      items: [
        {
          projectId: 'demo-1-p1',
          name: '作品一',
          description: '',
          coverUrl: null,
          type: 'html',
          shareUrl: '/share/project/demo-1-p1',
          addedAt: '2026-11-08T00:00:00.000Z',
        },
      ],
      featured: { kind: 'project', projectId: 'demo-1-p1' },
      published: true,
      commentCount: 3,
      createdAt: '2026-11-08T00:00:00.000Z',
      updatedAt: '2026-11-08T00:00:00.000Z',
    }),
  }),
}))

let app: App | null = null
let host: HTMLElement

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardShareView)
  app.use(createPinia())
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardShareView', () => {
  it('renders the card profile and projects', () => {
    mount()
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('作品一')
  })

  it('keeps comment and message entries disabled', () => {
    mount()
    expect(host.querySelector<HTMLButtonElement>('.card-social-comment')?.disabled).toBe(true)
    expect(host.querySelector<HTMLButtonElement>('.card-social-message')?.disabled).toBe(true)
  })
})
```

Create `packages/web/src/views/ProjectShareView.test.ts`:

```ts
import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import ProjectShareView from './ProjectShareView.vue'
import type { CardContentType } from '@/types'

let currentType: CardContentType = 'html'

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { projectId: 'demo-1-p1' } }) }))

vi.mock('@/stores/card', () => ({
  useCardStore: () => ({
    load: vi.fn(),
    findProject: () => ({
      card: { id: 'demo-1', ownerName: '林一' },
      item: {
        projectId: 'demo-1-p1',
        name: '作品一',
        description: '',
        coverUrl: null,
        type: currentType,
        shareUrl: '/share/project/demo-1-p1',
        previewUrl: 'https://example.com/demo',
        addedAt: '2026-11-08T00:00:00.000Z',
      },
    }),
  }),
}))

let app: App | null = null
let host: HTMLElement

function mount(type: CardContentType) {
  currentType = type
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(ProjectShareView)
  app.use(createPinia())
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('ProjectShareView', () => {
  it('renders html in an iframe', () => {
    mount('html')
    expect(host.querySelector('iframe')).not.toBeNull()
  })

  it('renders an image', () => {
    mount('image')
    expect(host.querySelector('img')).not.toBeNull()
  })

  it('renders a pdf iframe', () => {
    mount('pdf')
    expect(host.querySelector('iframe')).not.toBeNull()
  })

  it('renders a video', () => {
    mount('video')
    expect(host.querySelector('video')).not.toBeNull()
  })

  it('renders a document', () => {
    mount('document')
    expect(host.querySelector('.share-document')).not.toBeNull()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd packages/web && npm test -- --run src/views/CardShareView.test.ts src/views/ProjectShareView.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 3: Implement `ProjectShareView.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { DEMO_DOCUMENT, DEMO_HTML, DEMO_IMAGE, DEMO_PDF, DEMO_VIDEO } from '@/mocks/cards'
import { useCardStore } from '@/stores/card'
import type { CardContentType } from '@/types'
import { CARD_TYPE_LABELS } from '@/utils/card'
import { renderMarkdown } from '@/utils/markdown'
import '@/styles/card.css'

const route = useRoute()
const store = useCardStore()
const ready = ref(false)

onMounted(() => {
  store.load()
  ready.value = true
})

const found = computed(() => (ready.value ? store.findProject(String(route.params.projectId ?? '')) : null))
const item = computed(() => found.value?.item ?? null)
const card = computed(() => found.value?.card ?? null)
const type = computed<CardContentType>(() => item.value?.type ?? 'html')

const htmlExternal = computed(() => {
  const url = item.value?.shareUrl ?? ''
  return url.startsWith('http') || url.startsWith('/p/')
})

const htmlSource = computed(() => (htmlExternal.value ? (item.value?.shareUrl ?? '') : DEMO_HTML))

const mediaSource = computed(() => {
  if (!item.value) return ''
  if (item.value.previewUrl) return item.value.previewUrl
  if (type.value === 'image') return DEMO_IMAGE
  if (type.value === 'pdf') return DEMO_PDF
  if (type.value === 'video') return DEMO_VIDEO
  return ''
})

const documentHTML = computed(() => renderMarkdown(item.value?.previewUrl || DEMO_DOCUMENT))
</script>

<template>
  <main class="share-page">
    <div v-if="!ready" class="card-empty">加载中…</div>
    <div v-else-if="!item" class="share-empty">
      <h1>项目不存在或已下线</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-head">
        <a class="share-back" href="/">← 返回</a>
        <div>
          <h1>{{ item.name }}</h1>
          <p>
            <span class="tag">{{ CARD_TYPE_LABELS[type] }}</span>
            <span v-if="card">来自 {{ card.ownerName }} 的名片</span>
          </p>
        </div>
      </header>

      <section class="share-stage">
        <iframe
          v-if="type === 'html' && htmlExternal"
          :src="htmlSource"
          title="HTML 游戏预览"
          sandbox="allow-scripts allow-forms allow-same-origin"
        />
        <iframe
          v-else-if="type === 'html'"
          :srcdoc="htmlSource"
          title="HTML 演示预览"
          sandbox="allow-scripts allow-forms"
        />
        <img v-else-if="type === 'image'" :src="mediaSource" :alt="item.name" />
        <iframe v-else-if="type === 'pdf'" :src="mediaSource" title="PDF 预览" />
        <video v-else-if="type === 'video'" :src="mediaSource" controls playsinline />
        <article v-else class="share-document" v-html="documentHTML" />
      </section>

      <footer class="share-foot">
        <a v-if="item.shareUrl" class="card-secondary" :href="item.shareUrl" target="_blank" rel="noopener noreferrer">
          打开原项目
        </a>
        <a v-if="card" class="card-primary" :href="`/share/card/${card.id}`">查看名片</a>
      </footer>
    </template>
  </main>
</template>
```

- [ ] **Step 4: Implement `CardShareView.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import CardProjectItem from '@/components/card/CardProjectItem.vue'
import CardPreview from '@/components/card/CardPreview.vue'
import { useCardStore } from '@/stores/card'
import '@/styles/card.css'

const route = useRoute()
const store = useCardStore()
const ready = ref(false)
const notice = ref('')
const copied = ref(false)

onMounted(() => {
  store.load()
  ready.value = true
})

const card = computed(() => (ready.value ? store.getCardById(String(route.params.id ?? '')) : null))
const shareUrl = computed(() => window.location.href)

function openProject(projectId: string) {
  window.open(`/share/project/${encodeURIComponent(projectId)}`, '_blank', 'noopener,noreferrer')
}

function comingSoon(label: string) {
  notice.value = `${label}即将上线`
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(shareUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    notice.value = '复制失败，请手动复制链接'
  }
}
</script>

<template>
  <main class="share-page">
    <div v-if="!ready" class="card-empty">加载中…</div>
    <div v-else-if="!card" class="share-empty">
      <h1>名片不存在或已下线</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-card-head">
        <div class="share-person">
          <img v-if="card.avatarUrl" class="share-avatar" :src="card.avatarUrl" :alt="card.ownerName" />
          <span v-else class="share-avatar">{{ (card.ownerName || '?').charAt(0).toUpperCase() }}</span>
          <div>
            <h1>{{ card.ownerName || '未命名开发者' }}</h1>
            <p>{{ card.headline || '这个人还没有写简介' }}</p>
            <div class="share-tags">
              <span v-for="tag in card.tags" :key="tag" class="tag">{{ tag }}</span>
            </div>
          </div>
        </div>
        <button class="card-secondary" type="button" @click="copyLink">
          <i class="pi pi-copy" />
          {{ copied ? '已复制' : '复制分享链接' }}
        </button>
      </header>

      <p v-if="notice" class="share-notice">{{ notice }}</p>

      <section class="share-featured">
        <CardPreview :featured="card.featured" :items="card.items" large />
      </section>

      <section class="share-projects">
        <h2>项目</h2>
        <p v-if="card.items.length === 0" class="card-empty">还没有添加项目。</p>
        <div v-else class="card-project-list">
          <CardProjectItem
            v-for="item in card.items"
            :key="item.projectId"
            :item="item"
            @open="openProject(item.projectId)"
            @remove="() => undefined"
          />
        </div>
      </section>

      <footer class="share-social">
        <button class="card-social-comment" type="button" disabled @click="comingSoon('评论')">
          <i class="pi pi-comment" />
          评论 · {{ card.commentCount }}
        </button>
        <button class="card-social-message" type="button" disabled @click="comingSoon('私信')">
          <i class="pi pi-send" />
          私信
        </button>
        <span class="tag">即将上线</span>
      </footer>
    </template>
  </main>
</template>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd packages/web && npm test -- --run src/views/CardShareView.test.ts src/views/ProjectShareView.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/views/CardShareView.vue packages/web/src/views/ProjectShareView.vue packages/web/src/views/CardShareView.test.ts packages/web/src/views/ProjectShareView.test.ts
git commit -m "feat(web): add card and project share pages"
```

---

### Task 9: 名片模块样式与响应式

**Files:**
- Create: `packages/web/src/styles/card.css`
- Modify: `packages/web/src/styles/home.css`

**Interfaces:**
- Consumes: 全局 CSS 变量 `--ink`、`--text`、`--muted`、`--line`、`--line-strong`、`--surface`、`--canvas`、`--soft`、`--blue`、`--blue-soft`。
- Produces: 名片模块全部类名样式 + 侧栏“我的名片”入口样式。

- [ ] **Step 1: Create `card.css`**

```css
.card-page,
.discover-page {
  display: flex;
  height: 100%;
  flex-direction: column;
  gap: 16px;
  padding: 24px;
}

.card-page {
  overflow: hidden;
}

.discover-page {
  overflow-y: auto;
}

.card-page-head,
.card-panel-head,
.discover-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.card-page-head h1,
.discover-head h1 {
  margin: 0;
  color: var(--ink);
  font-size: 22px;
}

.card-page-head p,
.card-panel-head p,
.discover-head p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.card-page-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-share-hint {
  color: var(--muted);
  font-size: 12px;
}

.card-page-grid {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(260px, 1fr);
  gap: 16px;
  min-height: 0;
  flex: 1;
}

.card-content-panel,
.card-profile {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.card-content-panel {
  overflow: hidden;
}

.card-project-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
  padding-right: 4px;
}

.card-project-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 12px;
  cursor: pointer;
}

.card-project-item:hover {
  border-color: var(--line-strong);
}

.card-project-cover {
  width: 96px;
  height: 58px;
  flex: none;
  overflow: hidden;
  border-radius: 8px;
  background: var(--surface);
}

.card-project-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.card-project-placeholder {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  color: var(--muted);
}

.card-project-body {
  min-width: 0;
  flex: 1;
}

.card-project-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-project-title strong {
  overflow: hidden;
  color: var(--ink);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-project-body p {
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-project-remove,
.card-tag button {
  display: inline-grid;
  width: 24px;
  height: 24px;
  flex: none;
  place-items: center;
  background: transparent;
  border: 0;
  border-radius: 999px;
  color: var(--muted);
  cursor: pointer;
}

.card-project-remove:hover,
.card-tag button:hover {
  background: var(--surface);
  color: var(--ink);
}

.card-empty {
  margin: 0;
  padding: 28px 12px;
  color: var(--muted);
  font-size: 13px;
  text-align: center;
}

.card-profile-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-profile-avatar,
.card-tile-avatar,
.share-avatar {
  display: grid;
  width: 40px;
  height: 40px;
  flex: none;
  place-items: center;
  overflow: hidden;
  background: var(--ink);
  border-radius: 999px;
  color: var(--surface);
  font-size: 14px;
  font-weight: 700;
  object-fit: cover;
}

.card-profile-head strong {
  color: var(--ink);
  font-size: 14px;
}

.card-profile-head p {
  margin: 2px 0 0;
  color: var(--muted);
  font-size: 11px;
}

.card-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--ink);
  font-size: 12px;
  font-weight: 600;
}

.card-field > input,
.card-field select {
  width: 100%;
  height: 34px;
  padding: 0 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  font-size: 12px;
  font-weight: 400;
  outline: none;
}

.card-field small {
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
  line-height: 1.6;
}

.card-preview {
  display: grid;
  min-height: 120px;
  place-items: center;
  overflow: hidden;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.card-preview.large {
  min-height: 180px;
}

.card-preview img {
  width: 100%;
  height: 100%;
  max-height: 260px;
  object-fit: cover;
}

.card-preview-pdf {
  display: flex;
  width: 100%;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  color: var(--muted);
  font-size: 11px;
}

.card-preview-pdf iframe {
  width: 100%;
  min-height: 180px;
  border: 1px solid var(--line);
  border-radius: 8px;
}

.card-preview-file,
.card-preview-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 18px;
  color: var(--muted);
  font-size: 12px;
}

.card-preview-file i,
.card-preview-empty i {
  font-size: 26px;
}

.card-featured-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.card-file-input {
  display: none;
}

.card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.card-tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 4px 2px 8px;
  background: var(--blue-soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--blue);
  font-size: 11px;
}

.card-share-btn,
.card-add-btn,
.card-primary,
.card-secondary,
.card-text-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
}

.card-share-btn,
.card-add-btn,
.card-primary {
  background: var(--ink);
  border: 1px solid var(--ink);
  color: var(--surface);
}

.card-secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}

.card-text-btn {
  height: 30px;
  padding: 0 8px;
  background: transparent;
  border: 0;
  color: var(--muted);
}

.card-share-btn:disabled,
.card-add-btn:disabled,
.card-secondary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.card-share-dialog {
  width: min(560px, 100%);
}

.card-share-note {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.7;
}

.card-share-link {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.card-share-link span {
  flex: 1;
  overflow: hidden;
  color: var(--ink);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-share-link button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 6px;
  color: var(--ink);
  font-size: 11px;
  cursor: pointer;
}

.card-share-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.add-project-dialog {
  width: min(640px, 100%);
}

.add-project-note {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.add-project-list {
  display: flex;
  max-height: 420px;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
}

.add-project-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.add-project-cover {
  width: 72px;
  height: 46px;
  flex: none;
  overflow: hidden;
  border-radius: 6px;
  object-fit: cover;
}

.add-project-cover.placeholder {
  display: grid;
  place-items: center;
  background: var(--surface);
  color: var(--muted);
}

.add-project-info {
  min-width: 0;
  flex: 1;
}

.add-project-info strong {
  color: var(--ink);
  font-size: 13px;
}

.add-project-info p {
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.add-project-action {
  height: 30px;
  padding: 0 12px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 7px;
  color: var(--surface);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.add-project-action:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.discover-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.card-tile {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  cursor: pointer;
}

.card-tile:hover {
  border-color: var(--line-strong);
}

.card-tile-preview {
  height: 180px;
  overflow: hidden;
  background: var(--soft);
}

.card-tile-preview .card-preview {
  height: 100%;
  border: 0;
  border-radius: 0;
}

.card-tile-preview .card-preview img {
  height: 100%;
  max-height: none;
  object-fit: cover;
}

.card-tile-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
}

.card-tile-person {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-tile-person strong {
  color: var(--ink);
  font-size: 14px;
}

.card-tile-person p {
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-tile-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.card-tile-foot {
  display: flex;
  justify-content: space-between;
  color: var(--muted);
  font-size: 11px;
}

.discover-sentinel {
  height: 1px;
}

.discover-more-wrap {
  display: flex;
  justify-content: center;
  padding: 8px 0 24px;
}

.discover-more {
  height: 34px;
  padding: 0 18px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 999px;
  color: var(--ink);
  font-size: 12px;
  cursor: pointer;
}

.discover-end {
  color: var(--muted);
  font-size: 12px;
}

.share-page {
  width: min(960px, 100%);
  min-height: 100vh;
  margin: 0 auto;
  padding: 24px 16px 48px;
}

.share-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 80px 0;
}

.share-head,
.share-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.share-head h1,
.share-card-head h1 {
  margin: 0;
  color: var(--ink);
  font-size: 24px;
}

.share-head p,
.share-card-head p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.share-back {
  color: var(--muted);
  font-size: 12px;
  text-decoration: none;
}

.share-person {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.share-avatar {
  width: 64px;
  height: 64px;
  font-size: 22px;
}

.share-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.share-notice {
  margin: 0 0 12px;
  color: var(--blue);
  font-size: 13px;
}

.share-featured {
  margin-bottom: 16px;
  padding: 14px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.share-stage {
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.share-stage iframe,
.share-stage video,
.share-stage img {
  display: block;
  width: 100%;
  min-height: 480px;
  border: 0;
  object-fit: contain;
}

.share-stage img {
  max-height: 70vh;
}

.share-document {
  padding: 24px;
  color: var(--text);
  line-height: 1.8;
}

.share-projects {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 20px;
}

.share-projects h2 {
  margin: 0;
  color: var(--ink);
  font-size: 16px;
}

.share-social {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 24px;
  padding-top: 18px;
  border-top: 1px solid var(--line);
}

.card-social-comment,
.card-social-message {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  font-size: 12px;
}

.card-social-comment:disabled,
.card-social-message:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.share-foot {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 16px;
}

@media (max-width: 960px) {
  .card-page {
    height: auto;
    overflow: visible;
  }

  .card-page-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .card-content-panel,
  .card-project-list {
    overflow: visible;
  }

  .discover-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .share-card-head,
  .share-head {
    flex-direction: column;
  }

  .share-stage iframe,
  .share-stage video,
  .share-stage img {
    min-height: 320px;
  }
}
```

- [ ] **Step 2: Add sidebar entry style to `home.css`**

在 `.sidebar-footer` 相关样式附近追加：

```css
.sidebar-card-entry {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  margin-bottom: 8px;
  padding: 9px 10px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 9px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
}

.sidebar-card-entry:hover {
  background: var(--surface);
  border-color: var(--line);
  color: var(--ink);
}

.sidebar-card-entry i {
  width: 16px;
  font-size: 14px;
}

.app-sidebar.collapsed .sidebar-card-entry {
  justify-content: center;
  padding: 9px 0;
}

.app-sidebar.collapsed .sidebar-card-entry span {
  display: none;
}
```

- [ ] **Step 3: Run palette / lint check**

Run: `make quality`
Expected: 调色板检查通过，无新增非黑白灰蓝颜色。

- [ ] **Step 4: Commit**

```bash
git add packages/web/src/styles/card.css packages/web/src/styles/home.css
git commit -m "feat(web): add card module styles"
```

---

### Task 10: ops-web 名片管理

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Create: `packages/ops-web/src/mocks/cards.ts`
- Create: `packages/ops-web/src/api/cards.ts`
- Test: `packages/ops-web/src/api/cards.test.ts`
- Create: `packages/ops-web/src/views/CardsView.vue`
- Test: `packages/ops-web/src/views/CardsView.test.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: `PageQuery`、`PageResult`、`TableColumn`、`useAdminPage`、`DataTable`、`TableSearch`、`TablePagination`。
- Produces:
  - `AdminCardSummary`
  - `listCards(query): Promise<PageResult<AdminCardSummary>>`
  - `updateCardHidden(id, hidden): Promise<void>`
  - `deleteCard(id): Promise<void>`
  - 路由 `/cards`、导航「名片管理」。

- [ ] **Step 1: Write the failing API test**

Create `packages/ops-web/src/api/cards.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import { deleteCard, listCards, updateCardHidden } from './cards'

beforeEach(() => localStorage.clear())

describe('cards api', () => {
  it('seeds and paginates cards', async () => {
    const result = await listCards({ page: 1, pageSize: 5 })
    expect(result.items).toHaveLength(5)
    expect(result.total).toBeGreaterThan(5)
  })

  it('filters by keyword', async () => {
    const result = await listCards({ keyword: '林一' })
    expect(result.items.some((card) => card.ownerName.includes('林一'))).toBe(true)
  })

  it('toggles hidden state', async () => {
    const first = (await listCards({ page: 1, pageSize: 1 })).items[0]!
    await updateCardHidden(first.id, true)
    const reloaded = await listCards({ keyword: first.ownerName })
    expect(reloaded.items.find((card) => card.id === first.id)?.hidden).toBe(true)
  })

  it('deletes a card', async () => {
    const first = (await listCards({ page: 1, pageSize: 1 })).items[0]!
    await deleteCard(first.id)
    const reloaded = await listCards({})
    expect(reloaded.items.some((card) => card.id === first.id)).toBe(false)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/ops-web && npm test -- --run src/api/cards.test.ts`
Expected: FAIL，模块不存在。

- [ ] **Step 3: Add type, mock seed and API**

`packages/ops-web/src/types/index.ts` 追加：

```ts
export interface AdminCardSummary {
  id: string
  ownerName: string
  user: string
  tags: string[]
  projectCount: number
  featuredType: string
  hidden: boolean
  updatedAt: string
}
```

创建 `packages/ops-web/src/mocks/cards.ts`：

```ts
import type { AdminCardSummary } from '@/types'

const PEOPLE = [
  ['林一', 'alice', ['Vue', '游戏', '开源']],
  ['周舟', 'bob', ['AI', 'Go', '产品']],
  ['陈默', 'carol', ['设计', 'CSS']],
  ['苏白', 'dave', ['全栈', '可视化']],
  ['顾言', 'erin', ['Go', '架构', 'MySQL']],
  ['叶青', 'frank', ['产品', '增长']],
  ['沈知', 'grace', ['NLP', 'Python']],
  ['许川', 'heidi', ['WebGL', '游戏']],
  ['陆离', 'ivan', ['开源', 'Rust']],
  ['唐糖', 'judy', ['前端', '学习']],
  ['韩野', 'ken', ['SRE', '监控']],
  ['白露', 'lily', ['插画', 'SVG']],
] as const

const TYPES = ['HTML', '图片', 'PDF', '视频', '文档']

export const cardSeed: AdminCardSummary[] = PEOPLE.map(([ownerName, user, tags], index) => ({
  id: `demo-${index + 1}`,
  ownerName,
  user,
  tags: [...tags],
  projectCount: 2 + (index % 4),
  featuredType: TYPES[index % TYPES.length] ?? 'HTML',
  hidden: false,
  updatedAt: new Date(Date.parse('2026-11-08T10:00:00.000Z') - index * 3_600_000).toISOString(),
}))
```

创建 `packages/ops-web/src/api/cards.ts`：

```ts
import { cardSeed } from '@/mocks/cards'
import type { AdminCardSummary, PageQuery, PageResult } from '@/types'

const KEY = 'vibebiz:ops:cards:v1'

function delay(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 200))
}

function read(): AdminCardSummary[] {
  const raw = localStorage.getItem(KEY)
  if (!raw) {
    const seed = cardSeed.map((card) => ({ ...card }))
    localStorage.setItem(KEY, JSON.stringify(seed))
    return seed
  }
  try {
    const parsed = JSON.parse(raw) as AdminCardSummary[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function write(cards: AdminCardSummary[]): void {
  localStorage.setItem(KEY, JSON.stringify(cards))
}

export async function listCards(query: PageQuery = {}): Promise<PageResult<AdminCardSummary>> {
  await delay()
  const keyword = (query.keyword ?? '').trim().toLowerCase()
  const page = Math.max(1, query.page ?? 1)
  const pageSize = Math.max(1, query.pageSize ?? 10)
  const filtered = read().filter((card) => {
    if (!keyword) return true
    return (
      card.ownerName.toLowerCase().includes(keyword) ||
      card.user.toLowerCase().includes(keyword)
    )
  })
  return {
    items: filtered.slice((page - 1) * pageSize, page * pageSize),
    total: filtered.length,
    page,
    pageSize,
  }
}

export async function updateCardHidden(id: string, hidden: boolean): Promise<void> {
  await delay()
  write(read().map((card) => (card.id === id ? { ...card, hidden } : card)))
}

export async function deleteCard(id: string): Promise<void> {
  await delay()
  write(read().filter((card) => card.id !== id))
}
```

- [ ] **Step 4: Run API test**

Run: `cd packages/ops-web && npm test -- --run src/api/cards.test.ts`
Expected: PASS

- [ ] **Step 5: Write the failing view test**

Create `packages/ops-web/src/views/CardsView.test.ts`:

```ts
import { createApp, type App } from 'vue'
import PrimeVue from 'primevue/config'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { deleteCard, listCards, updateCardHidden } from '@/api/cards'
import CardsView from './CardsView.vue'

vi.mock('@/api/cards', () => ({
  listCards: vi.fn(),
  updateCardHidden: vi.fn(),
  deleteCard: vi.fn(),
}))

const listMock = vi.mocked(listCards)
const hideMock = vi.mocked(updateCardHidden)
const deleteMock = vi.mocked(deleteCard)

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
  listMock.mockResolvedValue({
    items: [
      {
        id: 'demo-1',
        ownerName: '林一',
        user: 'alice',
        tags: ['Vue', '游戏'],
        projectCount: 3,
        featuredType: 'HTML',
        hidden: false,
        updatedAt: '2026-11-08T10:00:00.000Z',
      },
    ],
    total: 1,
    page: 1,
    pageSize: 10,
  })
  hideMock.mockResolvedValue(undefined)
  deleteMock.mockResolvedValue(undefined)
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardsView)
  app.use(PrimeVue)
  app.mount(host)
}

describe('CardsView', () => {
  it('renders card rows', async () => {
    mount()
    await flush()
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('Vue')
  })

  it('hides a card and reloads', async () => {
    mount()
    await flush()
    const hideButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('隐藏'),
    )
    hideButton?.click()
    await flush()
    expect(hideMock).toHaveBeenCalledWith('demo-1', true)
    expect(listMock).toHaveBeenCalledTimes(2)
  })

  it('deletes a card after confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    mount()
    await flush()
    const deleteButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('删除'),
    )
    deleteButton?.click()
    await flush()
    expect(deleteMock).toHaveBeenCalledWith('demo-1')
  })
})
```

- [ ] **Step 6: Run view test to verify it fails**

Run: `cd packages/ops-web && npm test -- --run src/views/CardsView.test.ts`
Expected: FAIL，组件不存在。

- [ ] **Step 7: Implement `CardsView.vue`**

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import { ref } from 'vue'

import { deleteCard, listCards, updateCardHidden } from '@/api/cards'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminCardSummary, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(listCards)

const columns: TableColumn[] = [
  { key: 'ownerName', label: '名片', width: '140px' },
  { key: 'user', label: '用户', width: '140px' },
  { key: 'tags', label: '标签', format: (value) => (Array.isArray(value) ? value.join(' / ') : '—') },
  { key: 'projectCount', label: '项目数', width: '90px' },
  { key: 'featuredType', label: '主预览', width: '100px' },
  { key: 'hidden', label: '状态', width: '100px' },
  { key: 'updatedAt', label: '更新时间', width: '180px' },
  { key: 'actions', label: '操作', width: '180px' },
]

const notice = ref('')
const actionError = ref('')
const busyId = ref('')

async function toggleHidden(row: unknown) {
  const card = row as AdminCardSummary
  busyId.value = card.id
  actionError.value = ''
  try {
    await updateCardHidden(card.id, !card.hidden)
    notice.value = card.hidden ? '已恢复名片' : '已隐藏名片'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '更新名片失败'
  } finally {
    busyId.value = ''
  }
}

async function remove(row: unknown) {
  const card = row as AdminCardSummary
  if (!window.confirm(`确定删除「${card.ownerName}」的名片吗？`)) return
  busyId.value = card.id
  actionError.value = ''
  try {
    await deleteCard(card.id)
    notice.value = '名片已删除'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除名片失败'
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>名片管理</h1>
        <p class="page-desc">查看用户发布的名片，隐藏或删除违规内容。</p>
      </div>
      <span class="tag">{{ total }} 张</span>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>
    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无名片"
      @retry="load"
    >
      <template #cell-hidden="{ value }">{{ value ? '已隐藏' : '正常' }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminCardSummary).hidden ? '恢复' : '隐藏'"
            size="small"
            text
            :loading="busyId === (row as AdminCardSummary).id"
            @click="toggleHidden(row)"
          />
          <Button
            label="删除"
            size="small"
            text
            :loading="busyId === (row as AdminCardSummary).id"
            @click="remove(row)"
          />
        </div>
      </template>
    </DataTable>

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

<style scoped>
.inline-notice {
  margin-bottom: 12px;
  color: var(--blue);
  font-size: 13px;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
</style>
```

- [ ] **Step 8: Run view test**

Run: `cd packages/ops-web && npm test -- --run src/views/CardsView.test.ts`
Expected: PASS

- [ ] **Step 9: Add navigation and route**

`packages/ops-web/src/router/navigation.ts` 在「项目」后加入：

```ts
{ name: 'cards', label: '名片管理', path: '/cards', icon: 'pi pi-address-book' },
```

`packages/ops-web/src/router/routes.ts` 导入 `CardsView` 并加入：

```ts
{ path: 'cards', name: 'cards', component: CardsView, meta: { title: '名片管理' } },
```

同步更新 `packages/ops-web/src/router/navigation.test.ts` 中的两个数组：`'/cards'` 紧跟在 `'/projects'` 后，`'名片管理'` 同理。

- [ ] **Step 10: Run ops-web tests**

Run: `cd packages/ops-web && npm test -- --run`
Expected: PASS

- [ ] **Step 11: Commit**

```bash
git add packages/ops-web/src/types/index.ts packages/ops-web/src/mocks/cards.ts packages/ops-web/src/api/cards.ts packages/ops-web/src/api/cards.test.ts packages/ops-web/src/views/CardsView.vue packages/ops-web/src/views/CardsView.test.ts packages/ops-web/src/router/navigation.ts packages/ops-web/src/router/routes.ts packages/ops-web/src/router/navigation.test.ts
git commit -m "feat(ops-web): add business card management"
```

---

### Task 11: 最终验证与收尾

**Files:**
- Modify: `.ai/CURRENT_FOCUS`（通过 `make checkpoint` 自动更新）

**Interfaces:**
- Consumes: Task 1-10 全部产出。
- Produces: 全量测试、构建、lint、质量门禁通过。

- [ ] **Step 1: Run web tests, build and lint**

```bash
cd packages/web
npm test -- --run
npm run build
npm run lint
```

Expected: 全部通过；`vue-tsc` 无类型错误。

- [ ] **Step 2: Run ops-web tests, build and lint**

```bash
cd packages/ops-web
npm test -- --run
npm run build
npm run lint
```

Expected: 全部通过。

- [ ] **Step 3: Run root quality gate**

```bash
make quality
make validate
```

Expected: `make quality` 通过（gofmt / go vet / ruff / 两个前端 eslint / 调色板）；`make validate` 通过。

- [ ] **Step 4: 手工冒烟**

1. `make dev` 启动五个服务。
2. 登录 web，左侧出现「发现」，底部头像上方出现「我的名片」。
3. `/my-card`：添加 1 个已发布项目；设置主预览；添加 2 个标签；点击分享；打开 `/share/card/me`。
4. `/discover`：看到两列卡片，点击「加载更多」追加 6 条。
5. 在 mock 名片中分别打开 HTML / 图片 / PDF / 视频 / 文档项目分享页，确认对应渲染。
6. 名片分享页评论 / 私信按钮为禁用状态。
7. ops-web 登录，进入「名片管理」，隐藏 / 恢复 / 删除各操作一次。
8. 手机宽度（<960px）下确认名片页与发现页变为单列。

- [ ] **Step 5: Harness checkpoint**

```bash
make checkpoint
```

Expected: `.ai/CURRENT_FOCUS` 更新，checkpoint 写入 `.ai/checkpoints/`。

- [ ] **Step 6: Commit**

```bash
git add .ai/CURRENT_FOCUS .ai/checkpoints
git commit -m "chore: checkpoint business card social frontend"
```

---

## Self-Review

**Spec coverage**

| Spec 要求 | 对应任务 |
|---|---|
| 左侧菜单「我的名片」在头像上方 | Task 7 |
| 左侧菜单「发现」 | Task 7 |
| 内容模块 : 个人信息 = 3 : 1 | Task 5 + Task 9 |
| 最多 5 个项目、只能选已发布 | Task 1 + Task 3 |
| 项目列表垂直滚动 | Task 9 |
| 点击项目进入项目分享页 | Task 5 + Task 8 |
| 项目分享页 4 类内容 | Task 8 |
| 主预览只能选已添加项目或上传 PDF/图片 | Task 4 + Task 2 |
| 标签最多 5 个 | Task 1 + Task 4 |
| 整张名片分享页 | Task 5 + Task 8 |
| 发现页两列 + 滚动加载 | Task 6 + Task 9 |
| 发现卡片预览为个人信息模块 | Task 6 |
| ops-web 名片管理隐藏/删除 | Task 10 |
| 评论/私信禁用入口 | Task 8 |
| 前端只改、不改后端 | Global Constraints |

**Placeholder scan:** 无 TBD / TODO / “类似 Task N”；每个代码步骤均给出实际代码。

**Type consistency:** `BusinessCard`、`CardProjectItem`、`CardFeatured`、`CardContentType` 在 Task 1 定义，后续任务只引用；store 方法名与组件 emits 名称跨任务一致；ops-web `AdminCardSummary` 与 API 返回字段一致。

**已知风险:** Task 8 的 mock 测试依赖 `vi.mock('@/stores/card')`，若组件在 setup 阶段访问 `window.location.href`，jsdom 提供默认 location，测试可正常运行；如遇 PrimeVue 组件渲染问题，按现有 `LeaderboardView.test.ts` 的 `matchMedia` mock 处理。
