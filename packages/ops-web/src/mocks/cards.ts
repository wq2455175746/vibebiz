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
