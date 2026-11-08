import type { ChannelPackageChannel } from '@/types'

export const channelOptions: { value: ChannelPackageChannel; label: string }[] = [
  { value: 'wechat_minigame', label: '微信小游戏' },
  { value: 'douyin_minigame', label: '抖音小游戏' },
  { value: 'steam_desktop', label: 'Steam 桌面' },
]

export function channelLabel(channel: ChannelPackageChannel): string {
  return channelOptions.find((item) => item.value === channel)?.label ?? channel
}

export function levelLabel(level: 'A' | 'B' | 'C'): string {
  if (level === 'A') return 'A 级：直接兼容'
  if (level === 'B') return 'B 级：适配层兜底，需真机验证'
  return 'C 级：需要改造，暂不支持生成'
}
