import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { listPublishedProjects } from '@/api/publish'
import AddProjectDialog from './AddProjectDialog.vue'

vi.mock('@/api/publish', () => ({ listPublishedProjects: vi.fn() }))

const listMock = vi.mocked(listPublishedProjects)
let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount(props: Record<string, unknown>) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(AddProjectDialog, props)
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

const published = (id: string, name: string) => ({
  projectId: id,
  name,
  description: '介绍',
  tags: [],
  coverUrl: null,
  publishedAt: '2026-11-08T00:00:00.000Z',
  url: `/p/${id}`,
})

describe('AddProjectDialog', () => {
  it('lists published projects and disables already added rows', async () => {
    listMock.mockResolvedValue([published('p1', '作品一'), published('p2', '作品二')])
    mount({ existingIds: ['p1'], full: false })
    await flush()
    expect(host.textContent).toContain('作品一')
    expect(host.textContent).toContain('作品二')
    const buttons = host.querySelectorAll<HTMLButtonElement>('.add-project-action')
    expect(buttons[0]?.disabled).toBe(true)
    expect(buttons[1]?.disabled).toBe(false)
  })

  it('disables every row when the card is full', async () => {
    listMock.mockResolvedValue([published('p3', '作品三')])
    mount({ existingIds: [], full: true })
    await flush()
    expect(host.querySelector<HTMLButtonElement>('.add-project-action')?.disabled).toBe(true)
  })

  it('shows the empty state', async () => {
    listMock.mockResolvedValue([])
    mount({ existingIds: [], full: false })
    await flush()
    expect(host.textContent).toContain('暂无已发布项目')
  })

  it('renders the provided add error', async () => {
    listMock.mockResolvedValue([published('p4', '作品四')])
    mount({ existingIds: [], full: false, error: '项目已添加或已达到 5 个上限' })
    await flush()
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('项目已添加或已达到 5 个上限')
  })

  it('disables a project while its add is pending', async () => {
    listMock.mockResolvedValue([published('p5', '作品五')])
    const onAdd = vi.fn()
    mount({ existingIds: [], full: false, onAdd })
    await flush()
    const button = host.querySelector<HTMLButtonElement>('.add-project-action')
    button?.click()
    await flush()
    expect(onAdd).toHaveBeenCalledTimes(1)
    expect(button?.disabled).toBe(true)
  })
})
