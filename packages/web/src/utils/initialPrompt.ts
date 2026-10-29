export function shouldSendInitialPrompt(
  prompt: string,
  hasHistory: boolean,
  append: boolean,
): boolean {
  return Boolean(prompt.trim()) && (append || !hasHistory)
}

type AppendStorage = Pick<Storage, 'getItem' | 'removeItem'>

export function takeInitialAppend(
  projectId: string,
  storage: AppendStorage = sessionStorage,
): boolean {
  const key = `vibebiz:initial-append:${projectId}`
  const append = storage.getItem(key) === '1'
  storage.removeItem(key)
  return append
}
