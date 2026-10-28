import axios from 'axios'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AvatarUploader from './AvatarUploader.vue'

vi.mock('axios', () => ({
  default: { put: vi.fn() },
}))

const putMock = vi.mocked(axios.put)

const file = new File(['avatar'], 'a.png', { type: 'image/png' })
const presigned = {
  uploadUrl: 'https://bucket.example.com/a.png',
  key: 'avatar/1/a.png',
  headers: { 'Content-Type': 'image/png' },
}

let app: App | null = null
let host: HTMLElement

function mount(uploaded = vi.fn(), overrides: Record<string, unknown> = {}) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(AvatarUploader, {
    src: null,
    name: 'Alice',
    presign: vi.fn().mockResolvedValue(presigned),
    confirm: vi.fn().mockResolvedValue(undefined),
    fallback: vi.fn().mockResolvedValue(undefined),
    onUploaded: uploaded,
    ...overrides,
  })
  app.mount(host)
  return uploaded
}

async function choose(fileToUpload: File) {
  const input = host.querySelector('input[type="file"]') as HTMLInputElement
  Object.defineProperty(input, 'files', { value: [fileToUpload], configurable: true })
  input.dispatchEvent(new Event('change'))
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

beforeEach(() => {
  putMock.mockReset()
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('AvatarUploader fallback', () => {
  it('uses the fallback when the direct upload fails', async () => {
    putMock.mockRejectedValue(new Error('CORS blocked'))
    const fallback = vi.fn().mockResolvedValue(undefined)
    const confirm = vi.fn()
    const uploaded = mount(vi.fn(), { fallback, confirm })

    await choose(file)

    expect(fallback).toHaveBeenCalledWith(file)
    expect(confirm).not.toHaveBeenCalled()
    expect(uploaded).toHaveBeenCalledTimes(1)
  })

  it('does not use the fallback when presign fails', async () => {
    const fallback = vi.fn()
    const uploaded = mount(vi.fn(), {
      presign: vi.fn().mockRejectedValue(new Error('presign failed')),
      fallback,
    })

    await choose(file)

    expect(putMock).not.toHaveBeenCalled()
    expect(fallback).not.toHaveBeenCalled()
    expect(uploaded).not.toHaveBeenCalled()
  })

  it('does not use the fallback for client-side validation errors', async () => {
    const fallback = vi.fn()
    mount(vi.fn(), { fallback })

    await choose(new File(['avatar'], 'a.gif', { type: 'image/gif' }))

    expect(fallback).not.toHaveBeenCalled()
  })

  it('shows an error when the fallback fails', async () => {
    putMock.mockRejectedValue(new Error('network error'))
    const fallback = vi.fn().mockRejectedValue(new Error('fallback unavailable'))
    const uploaded = mount(vi.fn(), { fallback })

    await choose(file)

    expect(uploaded).not.toHaveBeenCalled()
    expect(host.textContent).toContain('fallback unavailable')
  })
})
