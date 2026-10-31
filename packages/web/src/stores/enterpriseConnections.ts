import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  ArgoConnection,
  ArtifactConnectionInput,
  CloudConnection,
  CodeRepoConnection,
  DeliveryConnectionInput,
  EnterpriseConnection,
  HarborConnection,
  JenkinsConnection,
  K8sConnection,
} from '@/types/enterpriseSpace'
import { loadConnections, saveConnections } from '@/utils/enterpriseConnections'
import { randomId } from '@/utils/id'

export const useEnterpriseConnectionsStore = defineStore('enterpriseConnections', () => {
  const connections = ref<EnterpriseConnection[]>(loadConnections())

  const codeRepos = computed(() =>
    connections.value.filter((item): item is CodeRepoConnection => item.kind === 'code-repo'),
  )
  const harbors = computed(() =>
    connections.value.filter((item): item is HarborConnection => item.kind === 'harbor'),
  )
  const jenkins = computed(() =>
    connections.value.filter((item): item is JenkinsConnection => item.kind === 'jenkins'),
  )
  const k8sClusters = computed(() =>
    connections.value.filter((item): item is K8sConnection => item.kind === 'k8s'),
  )
  const cloudNodes = computed(() =>
    connections.value.filter((item): item is CloudConnection => item.kind === 'cloud'),
  )
  const argoes = computed(() =>
    connections.value.filter((item): item is ArgoConnection => item.kind === 'argo'),
  )

  function persist(): void {
    saveConnections(connections.value)
  }

  function create(input: ArtifactConnectionInput | DeliveryConnectionInput): EnterpriseConnection {
    const now = new Date().toISOString()
    const connection = {
      ...input,
      id: randomId(),
      createdAt: now,
      updatedAt: now,
    } as EnterpriseConnection
    connections.value = [...connections.value, connection]
    persist()
    return connection
  }

  function update(input: EnterpriseConnection): void {
    const index = connections.value.findIndex((item) => item.id === input.id)
    if (index < 0) return
    const previous = connections.value[index]
    const next = { ...input, updatedAt: new Date().toISOString() } as EnterpriseConnection
    if ('secret' in next && 'secret' in previous && !next.secret) {
      next.secret = previous.secret
    }
    if ('kubeconfig' in next && 'kubeconfig' in previous && !next.kubeconfig) {
      next.kubeconfig = previous.kubeconfig
    }
    const copy = [...connections.value]
    copy[index] = next
    connections.value = copy
    persist()
  }

  function remove(id: string): void {
    connections.value = connections.value.filter((item) => item.id !== id)
    persist()
  }

  return {
    connections,
    codeRepos,
    harbors,
    jenkins,
    k8sClusters,
    cloudNodes,
    argoes,
    create,
    update,
    remove,
  }
})
