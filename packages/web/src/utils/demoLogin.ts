export interface LoginDefaults {
  identifier: string
  password: string
}

export const demoLoginEnabled = import.meta.env.VITE_DEMO_LOGIN !== 'false'

export function loginDefaults(enabled: boolean): LoginDefaults {
  return enabled
    ? { identifier: 'wanghg11', password: '88888888' }
    : { identifier: '', password: '' }
}
