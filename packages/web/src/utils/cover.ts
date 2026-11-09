export const COVER_WIDTH = 1200
export const COVER_HEIGHT = 675
export const COVER_MAX_SOURCE_BYTES = 20 * 1024 * 1024

export interface CoverDrawRect {
  dx: number
  dy: number
  dw: number
  dh: number
}

export function coverDrawRect(
  sourceWidth: number,
  sourceHeight: number,
  targetWidth = COVER_WIDTH,
  targetHeight = COVER_HEIGHT,
): CoverDrawRect {
  if (sourceWidth <= 0 || sourceHeight <= 0) {
    throw new Error('invalid image size')
  }
  const scale = Math.max(targetWidth / sourceWidth, targetHeight / sourceHeight)
  const dw = sourceWidth * scale
  const dh = sourceHeight * scale
  return { dx: (targetWidth - dw) / 2, dy: (targetHeight - dh) / 2, dw, dh }
}

export function validateCoverFile(file: { type: string; size: number }): string {
  if (!file.type.startsWith('image/')) return '请选择图片文件'
  if (file.size <= 0) return '图片为空'
  if (file.size > COVER_MAX_SOURCE_BYTES) return '图片不能超过 20MB'
  return ''
}

function loadImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const image = new Image()
    image.onload = () => {
      URL.revokeObjectURL(url)
      resolve(image)
    }
    image.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('图片读取失败'))
    }
    image.src = url
  })
}

export async function createCoverBlob(file: File): Promise<Blob> {
  const error = validateCoverFile(file)
  if (error) throw new Error(error)
  const image = await loadImage(file)
  const canvas = document.createElement('canvas')
  canvas.width = COVER_WIDTH
  canvas.height = COVER_HEIGHT
  const context = canvas.getContext('2d')
  if (!context) throw new Error('无法处理图片')
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, COVER_WIDTH, COVER_HEIGHT)
  const rect = coverDrawRect(image.naturalWidth || image.width, image.naturalHeight || image.height)
  context.drawImage(image, rect.dx, rect.dy, rect.dw, rect.dh)
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('图片导出失败'))), 'image/png')
  })
}
