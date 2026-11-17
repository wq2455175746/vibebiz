import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import CardLikeButton from './CardLikeButton.vue'

let app: App | null = null
let host: HTMLElement

function mount(
  props: { liked: boolean; count: number; busy?: boolean },
  listeners: Record<string, unknown> = {},
) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardLikeButton, { ...props, ...listeners })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardLikeButton', () => {
  it('renders the liked state and count', () => {
    mount({ liked: true, count: 3 })
    expect(host.querySelector('.card-like-button.is-liked')).not.toBeNull()
    expect(host.querySelector('.card-like-button i')?.className).toContain('pi-heart-fill')
    expect(host.textContent).toContain('3')
  })

  it('renders the unliked state and count', () => {
    mount({ liked: false, count: 0 })
    expect(host.querySelector('.card-like-button.is-liked')).toBeNull()
    expect(host.querySelector('.card-like-button i')?.className).toContain('pi-heart')
    expect(host.textContent).toContain('0')
  })

  it('emits toggle when clicked', () => {
    const toggle = vi.fn()
    mount({ liked: false, count: 1 }, { onToggle: toggle })
    host.querySelector<HTMLButtonElement>('.card-like-button')?.click()
    expect(toggle).toHaveBeenCalledTimes(1)
  })

  it('does not emit while busy', () => {
    const toggle = vi.fn()
    mount({ liked: false, count: 1, busy: true }, { onToggle: toggle })
    const button = host.querySelector<HTMLButtonElement>('.card-like-button')
    expect(button?.disabled).toBe(true)
    button?.click()
    expect(toggle).not.toHaveBeenCalled()
  })

  it('stops click propagation', () => {
    const parentClick = vi.fn()
    host = document.createElement('div')
    document.body.appendChild(host)
    host.addEventListener('click', parentClick)
    app = createApp(CardLikeButton, { liked: false, count: 1 })
    app.mount(host)
    host.querySelector<HTMLButtonElement>('.card-like-button')?.click()
    expect(parentClick).not.toHaveBeenCalled()
  })
})
