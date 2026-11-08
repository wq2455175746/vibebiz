import type { ChannelGuide, ChannelPackageChannel, ChannelPackageLevel } from '@/types'

export interface ChannelOption {
  value: ChannelPackageChannel
  label: string
}

export const channelOptions: ChannelOption[] = [
  { value: 'wechat_minigame', label: '微信小游戏' },
  { value: 'douyin_minigame', label: '抖音小游戏' },
  { value: 'steam_desktop', label: 'Steam 桌面' },
]

const packageableValues = new Set<ChannelPackageChannel>(channelOptions.map((item) => item.value))

export function packageableChannelOptions(guides: ChannelGuide[]): ChannelOption[] {
  const options = guides
    .filter(
      (guide) => guide.packageable && packageableValues.has(guide.channel as ChannelPackageChannel),
    )
    .map((guide) => ({ value: guide.channel as ChannelPackageChannel, label: guide.title }))
  return options.length > 0 ? options : channelOptions
}

export function channelLabel(channel: ChannelPackageChannel): string {
  return channelOptions.find((item) => item.value === channel)?.label ?? channel
}

export function levelLabel(level: ChannelPackageLevel): string {
  if (level === 'A') return 'A 级：直接兼容'
  if (level === 'B') return 'B 级：适配层兜底，需真机验证'
  if (level === 'C') return 'C 级：需要改造，暂不支持生成'
  return ''
}
