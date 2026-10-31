export type CodeRepoProvider = 'github' | 'gitlab'
export type ArtifactConnectionKind = 'code-repo' | 'harbor' | 'jenkins'
export type DeliveryConnectionKind = 'k8s' | 'cloud' | 'argo'
export type DeliveryMethodKind = DeliveryConnectionKind | 'fde'
export type ConnectionKind = ArtifactConnectionKind | DeliveryConnectionKind

export interface ConnectionBase {
  id: string
  name: string
  createdAt: string
  updatedAt: string
}

export interface CodeRepoConnection extends ConnectionBase {
  kind: 'code-repo'
  provider: CodeRepoProvider
  url: string
  username: string
  secret: string
  defaultBranch: string
}

export interface HarborConnection extends ConnectionBase {
  kind: 'harbor'
  registryUrl: string
  namespace: string
  username: string
  secret: string
}

export interface JenkinsConnection extends ConnectionBase {
  kind: 'jenkins'
  url: string
  username: string
  secret: string
  jobs: string[]
}

export interface K8sConnection extends ConnectionBase {
  kind: 'k8s'
  apiServer: string
  kubeconfig: string
  defaultNamespace: string
}

export interface CloudConnection extends ConnectionBase {
  kind: 'cloud'
  host: string
  port: number
  username: string
  secret: string
}

export interface ArgoConnection extends ConnectionBase {
  kind: 'argo'
  url: string
  username: string
  secret: string
}

export type ArtifactConnection = CodeRepoConnection | HarborConnection | JenkinsConnection
export type DeliveryConnection = K8sConnection | CloudConnection | ArgoConnection
export type EnterpriseConnection = ArtifactConnection | DeliveryConnection

export type ArtifactConnectionInput =
  | Omit<CodeRepoConnection, 'id' | 'createdAt' | 'updatedAt'>
  | Omit<HarborConnection, 'id' | 'createdAt' | 'updatedAt'>
  | Omit<JenkinsConnection, 'id' | 'createdAt' | 'updatedAt'>

export type DeliveryConnectionInput =
  | Omit<K8sConnection, 'id' | 'createdAt' | 'updatedAt'>
  | Omit<CloudConnection, 'id' | 'createdAt' | 'updatedAt'>
  | Omit<ArgoConnection, 'id' | 'createdAt' | 'updatedAt'>

export type MockStepStatus = 'idle' | 'running' | 'success' | 'error'

export interface MockStepState {
  status: MockStepStatus
  startedAt: string | null
  completedAt: string | null
  error: string
}

export type ArtifactStepKey = 'code' | 'jenkins' | 'harbor'

export interface ArtifactFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  codeRepoId: string
  branch: string
  commitMessage: string
  code: MockStepState
  jenkinsId: string
  jenkinsJob: string
  jenkins: MockStepState
  harborId: string
  image: string
  tag: string
  harbor: MockStepState
}

export interface DeliveryFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  method: DeliveryMethodKind | null
  targetId: string
  namespace: string
  note: string
  execution: MockStepState
}

export interface FdeResource {
  id: number
  name: string
  contact: string
  skills: string[]
  region: string
  note: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface EnterpriseFlow {
  projectId: string
  artifact: ArtifactFlowState
  delivery: DeliveryFlowState
  updatedAt: string
}
