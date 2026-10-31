import type {
  CodeRepoConnection,
  EnterpriseConnection,
  HarborConnection,
  JenkinsConnection,
} from '@/types/enterpriseSpace'

export const CONNECTIONS_STORAGE_KEY = 'vibebiz:enterprise:connections:v1'

interface PersistedConnections {
  version: 1
  connections: EnterpriseConnection[]
}

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>

function isConnection(value: unknown): value is EnterpriseConnection {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<EnterpriseConnection>
  return typeof item.id === 'string' && typeof item.name === 'string' && typeof item.kind === 'string'
}

export function loadConnections(storage: ReadStorage = localStorage): EnterpriseConnection[] {
  try {
    const raw = storage.getItem(CONNECTIONS_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Partial<PersistedConnections>
    if (parsed.version !== 1 || !Array.isArray(parsed.connections)) return []
    return parsed.connections.filter(isConnection)
  } catch {
    return []
  }
}

export function saveConnections(
  connections: EnterpriseConnection[],
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(CONNECTIONS_STORAGE_KEY, JSON.stringify({ version: 1, connections }))
}

export function maskedCredential(value: string): string {
  return value ? '••••••••' : '未配置'
}

export function connectionDescription(connection: EnterpriseConnection): string {
  switch (connection.kind) {
    case 'code-repo':
      return `${connection.provider.toUpperCase()} · ${connection.url}`
    case 'harbor':
      return `${connection.registryUrl.replace(/\/+$/, '')}/${connection.namespace}`
    case 'jenkins':
      return `${connection.url} · ${connection.jobs.length} 个任务`
    case 'k8s':
      return `${connection.apiServer} · ${connection.defaultNamespace || 'default'}`
    case 'cloud':
      return `${connection.username}@${connection.host}:${connection.port}`
    case 'argo':
      return connection.url
    case 'fde':
      return connection.contact
  }
}

export function codeRepoTarget(connection: CodeRepoConnection): string {
  return `${connection.url} · ${connection.defaultBranch}`
}

export function harborTarget(connection: HarborConnection): string {
  return `${connection.registryUrl.replace(/\/+$/, '')}/${connection.namespace}`
}

export function jenkinsTarget(connection: JenkinsConnection): string {
  return `${connection.url} · ${connection.jobs.join(', ')}`
}
