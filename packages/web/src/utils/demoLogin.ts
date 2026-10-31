export interface LoginDefaults {
  identifier: string
  password: string
}

export const demoLoginEnabled = import.meta.env.VITE_DEMO_LOGIN !== 'false'

export function loginDefaults(enabled: boolean): LoginDefaults {
  if (!enabled) {
    return { identifier: '', password: '' }
  }
  return {
    identifier: import.meta.env.VITE_DEMO_IDENTIFIER ?? '',
    password: import.meta.env.VITE_DEMO_PASSWORD ?? '',
  }
}
