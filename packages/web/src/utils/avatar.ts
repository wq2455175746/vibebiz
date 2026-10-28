export const ACCEPTED_AVATAR_TYPES = ['image/jpeg', 'image/png', 'image/webp']
export const MAX_AVATAR_BYTES = 5 * 1024 * 1024

export interface AvatarFileMeta {
  type: string
  size: number
}

export function validateAvatarFile(file: AvatarFileMeta): string | null {
  if (!ACCEPTED_AVATAR_TYPES.includes(file.type)) {
    return '仅支持 JPG、PNG 或 WebP 图片'
  }
  if (file.size > MAX_AVATAR_BYTES) {
    return '图片大小不能超过 5MB'
  }
  return null
}
