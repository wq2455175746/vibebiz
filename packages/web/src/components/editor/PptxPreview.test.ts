import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const pptx = vi.hoisted(() => ({
  load: vi.fn(),
  destroy: vi.fn(),
  getSlideCount: vi.fn(() => 12),
  getCurrentSlide: vi.fn(() => 0),
}))

vi.mock('pptx-viewer', () => ({
  PPTXViewer: class {
    load = pptx.load
    destroy = pptx.destroy
    getSlideCount = pptx.getSlideCount
    getCurrentSlide = pptx.getCurrentSlide
  },
}))

import PptxPreview from './PptxPreview.vue'

let app: App | null = null
let host: HTMLElement
const fetchMock = vi.fn()

function mount(src = '/dp/api/v1/projects/p1/preview/demo.pptx') {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(PptxPreview, { src })
  app.mount(host)
  return host
}

async function flush() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await Promise.resolve()
}

beforeEach(() => {
  pptx.load.mockReset().mockResolvedValue(undefined)
  pptx.destroy.mockReset()
  pptx.getSlideCount.mockReset().mockReturnValue(12)
  pptx.getCurrentSlide.mockReset().mockReturnValue(0)
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  vi.unstubAllGlobals()
})

describe('PptxPreview', () => {
  it('fetches the authenticated pptx and renders it with the slide count', async () => {
    const buffer = new ArrayBuffer(8)
    fetchMock.mockResolvedValue({ ok: true, arrayBuffer: vi.fn().mockResolvedValue(buffer) })

    const container = mount()
    await flush()

    expect(fetchMock).toHaveBeenCalledWith(
      '/dp/api/v1/projects/p1/preview/demo.pptx',
      expect.objectContaining({ credentials: 'include', cache: 'no-store' }),
    )
    expect(pptx.load).toHaveBeenCalledWith(buffer)
    expect(container.textContent).toContain('12 页')
    expect(container.textContent).toContain('1 / 12')
  })

  it('shows an error when the file cannot be loaded', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 403 })

    const container = mount()
    await flush()

    expect(container.textContent).toContain('PPTX 预览加载失败')
    expect(container.textContent).toContain('403')
    expect(pptx.load).not.toHaveBeenCalled()
  })

  it('destroys the viewer when unmounted', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      arrayBuffer: vi.fn().mockResolvedValue(new ArrayBuffer(8)),
    })

    mount()
    await flush()
    app?.unmount()
    app = null

    expect(pptx.destroy).toHaveBeenCalledTimes(1)
  })
})
