<script setup lang="ts">
import { computed, ref } from 'vue'

import ConnectionList from '@/components/settings/ConnectionList.vue'
import DeliveryConnectionDialog from '@/components/settings/DeliveryConnectionDialog.vue'
import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import type {
  DeliveryConnection,
  DeliveryConnectionInput,
  DeliveryConnectionKind,
  EnterpriseConnection,
} from '@/types/enterpriseSpace'

const store = useEnterpriseConnectionsStore()
const activeTab = ref<DeliveryConnectionKind>('k8s')
const dialogOpen = ref(false)
const editing = ref<DeliveryConnection | null>(null)

const tabs: { key: DeliveryConnectionKind; label: string; empty: string }[] = [
  { key: 'k8s', label: 'K8s 集群', empty: '添加集群后，项目可部署到该集群。' },
  { key: 'cloud', label: '云主机', empty: '添加云主机节点，供项目直接部署。' },
  { key: 'argo', label: 'Argo', empty: '添加 Argo 地址，供项目触发 GitOps 交付。' },
]

const items = computed<EnterpriseConnection[]>(() => {
  if (activeTab.value === 'k8s') return store.k8sClusters
  if (activeTab.value === 'cloud') return store.cloudNodes
  return store.argoes
})

const activeLabel = computed(() => tabs.find((item) => item.key === activeTab.value)?.label ?? '')

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(item: EnterpriseConnection) {
  editing.value = item as DeliveryConnection
  dialogOpen.value = true
}

function save(input: DeliveryConnectionInput) {
  if (editing.value) {
    store.update({ ...editing.value, ...input } as DeliveryConnection)
  } else {
    store.create(input)
  }
  dialogOpen.value = false
  editing.value = null
}

function remove(item: EnterpriseConnection) {
  if (!window.confirm(`确定删除「${item.name}」吗？`)) return
  store.remove(item.id)
}
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>交付空间配置</h1>
        <p>维护 K8s、云主机和 Argo 连接，供企业项目阶段 7 使用。FDE 人员由运营在后台维护。</p>
      </div>
      <button class="config-add" type="button" @click="openCreate">
        <i class="pi pi-plus" />
        新增{{ activeLabel }}
      </button>
    </header>

    <p class="settings-notice">配置仅保存在当前浏览器；接入后端前请勿填写真实生产密钥。</p>

    <div class="config-tabs" role="tablist" aria-label="交付空间配置分类">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="config-tab"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>

    <ConnectionList
      :items="items"
      :empty-title="`暂无${activeLabel}`"
      :empty-text="tabs.find((item) => item.key === activeTab)?.empty ?? ''"
      @edit="openEdit"
      @remove="remove"
    />

    <DeliveryConnectionDialog
      v-if="dialogOpen"
      :kind="activeTab"
      :connection="editing"
      @close="dialogOpen = false"
      @save="save"
    />
  </section>
</template>
