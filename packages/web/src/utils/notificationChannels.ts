import type { NotificationChannelInput, NotificationChannelType } from '@/types'

export const notificationChannelTypes: {
  value: NotificationChannelType
  label: string
  icon: string
  description: string
}[] = [
  {
    value: 'webhook',
    label: 'Webhook',
    icon: 'pi pi-link',
    description: '向自定义 HTTPS 地址发送 JSON 测试消息。',
  },
  {
    value: 'feishu',
    label: '飞书机器人',
    icon: 'pi pi-send',
    description: '使用飞书群自定义机器人 Webhook。',
  },
  {
    value: 'wecom',
    label: '企微机器人',
    icon: 'pi pi-comments',
    description: '使用企业微信群机器人 Webhook。',
  },
  {
    value: 'dingtalk',
    label: '钉钉机器人',
    icon: 'pi pi-bell',
    description: '使用钉钉群自定义机器人 Webhook，支持加签。',
  },
  {
    value: 'email',
    label: '邮箱',
    icon: 'pi pi-envelope',
    description: '通过平台 SMTP 发送到指定邮箱。',
  },
]

export interface ChannelDraft {
  id?: number
  name: string
  type: NotificationChannelType
  url: string
  secret: string
  to: string
  enabled: boolean
}

export function emptyChannelDraft(type: NotificationChannelType = 'webhook'): ChannelDraft {
  return { name: '', type, url: '', secret: '', to: '', enabled: true }
}

export function typeLabel(type: NotificationChannelType): string {
  return notificationChannelTypes.find((item) => item.value === type)?.label ?? type
}

export function typeIcon(type: NotificationChannelType): string {
  return notificationChannelTypes.find((item) => item.value === type)?.icon ?? 'pi pi-bell'
}

export function validateChannelDraft(draft: ChannelDraft): string | null {
  const name = draft.name.trim()
  if (!name) {
    return '请输入渠道名称'
  }
  if (name.length > 64) {
    return '渠道名称不能超过 64 个字符'
  }
  if (draft.type === 'email') {
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.to.trim())) {
      return '请输入有效邮箱'
    }
    return null
  }
  if (!draft.url.trim()) {
    return '请输入 Webhook 地址'
  }
  try {
    const parsed = new URL(draft.url.trim())
    if (parsed.protocol !== 'https:') {
      return 'Webhook 地址必须使用 HTTPS'
    }
  } catch {
    return '请输入有效 Webhook 地址'
  }
  return null
}

export function draftToInput(draft: ChannelDraft): NotificationChannelInput {
  return {
    name: draft.name.trim(),
    type: draft.type,
    enabled: draft.enabled,
    config:
      draft.type === 'email'
        ? { to: draft.to.trim() }
        : {
            url: draft.url.trim(),
            ...(draft.secret.trim() ? { secret: draft.secret.trim() } : {}),
          },
  }
}
