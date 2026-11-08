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
