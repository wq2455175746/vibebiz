import type { ProjectStage } from '@/types'
import type {
  CloneDraft,
  CloneDraftAsset,
  CloneDraftAssetRole,
  CloneDraftErrors,
  CloneStageDefinition,
  CloneStageKey,
  CloneStageState,
  CloneStageStatus,
  FileLike,
} from '@/types/clone'

export const CLONE_NAME_MAX = 80
export const CLONE_DESCRIPTION_MAX = 2000
export const CLONE_URL_MAX = 1024
export const CLONE_IMAGE_MAX = 9
export const CLONE_IMAGE_BYTES = 10 * 1024 * 1024
export const CLONE_FILE_BYTES = 100 * 1024 * 1024

const IMAGE_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif'])
const ZIP_TYPES = new Set([
  'application/zip',
  'application/x-zip-compressed',
  'application/x-zip',
  'multipart/x-zip',
])

export const CLONE_STAGES: CloneStageDefinition[] = [
  {
    key: 'base_design',
    seq: 1,
    title: '基础设计',
    roleCode: 'bob',
    roleLabel: 'Bob · 系统架构师（目标分析）',
    artifactPath: 'docs/clone/base-design.md',
    workspace: 'document',
    summary: '拆解目标软件的技术架构、功能页面与克隆范围',
  },
  {
    key: 'architecture',
    seq: 2,
    title: '架构',
    roleCode: 'bob',
    roleLabel: 'Bob · 系统架构师（方案设计）',
    artifactPath: 'docs/clone/architecture.md',
    workspace: 'document',
    summary: '设计克隆实现方案、模块边界与接口契约',
  },
  {
    key: 'test',
    seq: 3,
    title: '测试',
    roleCode: 'qa',
    roleLabel: 'QA · 测试开发工程师',
    artifactPath: 'docs/clone/test-plan.md',
    workspace: 'test',
    summary: '生成测试方案，执行单测并输出报告',
  },
  {
    key: 'development',
    seq: 4,
    title: '全栈开发',
    roleCode: 'alex',
    roleLabel: 'Alex · 全栈工程师',
    artifactPath: 'docs/clone/development.md',
    workspace: 'development',
    summary: '按架构完成全栈开发与预览',
  },
  {
    key: 'artifacts',
    seq: 5,
    title: '制品空间',
    roleCode: '',
    roleLabel: '制品流水线',
    artifactPath: '',
    workspace: 'artifact',
    summary: '构建并归档可交付制品',
  },
]

const STAGE_STATUS: CloneStageStatus[] = ['locked', 'active', 'done']

export function isCloneStageKey(value: string): value is CloneStageKey {
  return CLONE_STAGES.some((stage) => stage.key === value)
}

export function cloneStageDefinition(key: CloneStageKey): CloneStageDefinition {
  const definition = CLONE_STAGES.find((stage) => stage.key === key)
  if (!definition) {
    throw new Error(`unknown clone stage: ${key}`)
  }
  return definition
}

export function createInitialCloneStages(): Record<CloneStageKey, CloneStageState> {
  return CLONE_STAGES.reduce(
    (result, stage) => {
      result[stage.key] = {
        status: stage.key === 'base_design' ? 'active' : 'locked',
        startedAt: null,
        completedAt: null,
      }
      return result
    },
    {} as Record<CloneStageKey, CloneStageState>,
  )
}

export function cloneStagesFromProject(
  stages: ProjectStage[],
): Record<CloneStageKey, CloneStageState> {
  const result = createInitialCloneStages()
  for (const stage of stages) {
    if (!isCloneStageKey(stage.stageKey)) {
      continue
    }
    const status = STAGE_STATUS.includes(stage.status as CloneStageStatus)
      ? (stage.status as CloneStageStatus)
      : 'locked'
    result[stage.stageKey] = {
      status,
      startedAt: stage.startedAt ?? null,
      completedAt: stage.completedAt ?? null,
    }
  }
  return result
}

export function isCloneUrl(value: string): boolean {
  const url = value.trim()
  if (!url) {
    return true
  }
  if (url.length > CLONE_URL_MAX) {
    return false
  }
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function validateCloneDraft(draft: CloneDraft): CloneDraftErrors {
  const errors: CloneDraftErrors = {}
  const name = draft.name.trim()
  const description = draft.description.trim()
  if (!name) {
    errors.name = '请填写名称'
  } else if (name.length > CLONE_NAME_MAX) {
    errors.name = `名称不能超过 ${CLONE_NAME_MAX} 字`
  }
  if (!description) {
    errors.description = '请填写描述'
  } else if (description.length > CLONE_DESCRIPTION_MAX) {
    errors.description = `描述不能超过 ${CLONE_DESCRIPTION_MAX} 字`
  }
  if (!isCloneUrl(draft.sourceUrl)) {
    errors.sourceUrl = '请输入 http 或 https 开头的有效链接'
  }
  return errors
}

export function canSubmitClone(draft: CloneDraft): boolean {
  return Object.keys(validateCloneDraft(draft)).length === 0
}

export function isCloneImage(asset: CloneDraftAsset): boolean {
  return asset.role === 'image'
}

export function cloneImageCount(assets: CloneDraftAsset[]): number {
  return assets.filter(isCloneImage).length
}

function isZip(file: FileLike): boolean {
  return file.name.toLowerCase().endsWith('.zip') || ZIP_TYPES.has(file.type)
}

export function validateCloneAsset(
  file: FileLike,
  assets: CloneDraftAsset[],
  role: CloneDraftAssetRole,
): string {
  if (file.size <= 0) {
    return '文件为空'
  }
  if (role === 'image') {
    if (cloneImageCount(assets) >= CLONE_IMAGE_MAX) {
      return `图片最多 ${CLONE_IMAGE_MAX} 张`
    }
    if (!IMAGE_TYPES.has(file.type)) {
      return '仅支持 JPG、PNG、WebP、GIF 图片'
    }
    if (file.size > CLONE_IMAGE_BYTES) {
      return '单张图片不能超过 10MB'
    }
    return ''
  }
  if (role === 'unit_test') {
    if (assets.some((item) => item.role === 'unit_test')) {
      return '只能上传 1 个单元测试压缩包'
    }
    if (!isZip(file)) {
      return '请上传 .zip 压缩包'
    }
    if (file.size > CLONE_FILE_BYTES) {
      return '单元测试压缩包不能超过 100MB'
    }
    return ''
  }
  if (file.size > CLONE_FILE_BYTES) {
    return '文档不能超过 100MB'
  }
  return ''
}

export function addCloneAsset(
  assets: CloneDraftAsset[],
  asset: CloneDraftAsset,
): CloneDraftAsset[] {
  return [...assets, asset]
}

export function removeCloneAsset(assets: CloneDraftAsset[], id: string): CloneDraftAsset[] {
  return assets.filter((asset) => asset.id !== id)
}

export function nextCoverAfterRemoval(
  assets: CloneDraftAsset[],
  coverAssetId: string | null,
  removedId: string,
): string | null {
  const remaining = assets.filter((asset) => asset.id !== removedId)
  if (coverAssetId && coverAssetId !== removedId) {
    const stillThere = remaining.some((asset) => asset.id === coverAssetId)
    if (stillThere) {
      return coverAssetId
    }
  }
  return remaining.find(isCloneImage)?.id ?? null
}

export function firstCloneImage(assets: CloneDraftAsset[]): CloneDraftAsset | undefined {
  return assets.find(isCloneImage)
}

export function createCloneAsset(
  id: string,
  file: FileLike,
  role: CloneDraftAssetRole,
  previewUrl?: string,
  sourceFile?: File,
): CloneDraftAsset {
  return {
    id,
    name: file.name,
    size: file.size,
    mime: file.type,
    role,
    ...(previewUrl ? { previewUrl } : {}),
    ...(sourceFile ? { file: sourceFile } : {}),
  }
}

export function formatCloneSize(bytes: number): string {
  if (bytes <= 0) {
    return '0 B'
  }
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function cloneAssetsByRole<T extends { role: string }>(
  assets: T[],
  role: string,
): T[] {
  return assets.filter((asset) => asset.role === role)
}

export function buildCloneStagePrompt(
  stage: CloneStageDefinition,
  project: { name: string; description: string; sourceUrl: string },
): string {
  if (stage.workspace === 'artifact') {
    return ''
  }
  const lines = [
    `当前阶段：${stage.seq}. ${stage.title}`,
    `负责角色：${stage.roleLabel}`,
    `阶段产物：${stage.artifactPath}`,
    '只完成当前阶段，不要提前进入后续阶段。',
    '',
    `目标名称：${project.name}`,
    `目标描述：${project.description}`,
    `参考 URL：${project.sourceUrl || '未提供'}（仅作参考，不主动抓取）`,
    '',
    '参考图片、文档和单元测试压缩包已放入项目的资料目录。',
  ]
  if (stage.key === 'base_design') {
    lines.push(
      '',
      '先分析目标软件本身：技术栈推测、系统与模块结构、数据模型、API、部署方式、功能与页面清单、克隆范围与信息缺口。',
      '所有推测必须标注为「已确认」「推断」「缺失」，信息不足时列出需要用户补充的问题。',
      '写入 docs/clone/base-design.md。',
    )
  } else if (stage.key === 'architecture') {
    lines.push(
      '',
      '先读取 docs/clone/base-design.md，再设计我们自己的克隆实现架构：技术选型、模块边界、数据模型、接口契约、目录结构、里程碑与风险。',
      '写入 docs/clone/architecture.md。',
    )
  } else if (stage.key === 'test') {
    lines.push(
      '',
      '先读取基础设计与架构产物，生成测试方案并写入 docs/clone/test-plan.md。',
      '测试方案完成后，使用已上传的单测执行测试，并确保生成 docs/clone/test-report.md。',
    )
  } else if (stage.key === 'development') {
    lines.push(
      '',
      '先读取基础设计、架构与测试产物，按架构完成全栈开发，并把完成情况写入 docs/clone/development.md。',
    )
  }
  lines.push('', '完成后等待用户确认，不要自行进入下一阶段。')
  return lines.join('\n')
}
