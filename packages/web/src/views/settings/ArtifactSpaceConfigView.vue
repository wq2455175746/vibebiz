<script setup lang="ts">
import { computed, ref } from 'vue'

import ArtifactConnectionDialog from '@/components/settings/ArtifactConnectionDialog.vue'
import ConnectionList from '@/components/settings/ConnectionList.vue'
import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import type {
  ArtifactConnection,
  ArtifactConnectionInput,
  ArtifactConnectionKind,
  EnterpriseConnection,
} from '@/types/enterpriseSpace'

const store = useEnterpriseConnectionsStore()
const activeTab = ref<ArtifactConnectionKind>('code-repo')
const dialogOpen = ref(false)
const editing = ref<ArtifactConnection | null>(null)

const tabs: { key: ArtifactConnectionKind; label: string; empty: string }[] = [
  { key: 'code-repo', label: '代码仓库', empty: '添加 GitHub 或 GitLab 仓库，供项目上传代码。' },
  { key: 'harbor', label: 'Harbor', empty: '添加镜像仓库，供项目推送和拉取镜像。' },
  { key: 'jenkins', label: 'Jenkins', empty: '添加 Jenkins 实例和任务，供项目触发构建。' },
]

const items = computed<EnterpriseConnection[]>(() => {
  if (activeTab.value === 'code-repo') return store.codeRepos
  if (activeTab.value === 'harbor') return store.harbors
  return store.jenkins
})

const activeLabel = computed(() => tabs.find((item) => item.key === activeTab.value)?.label ?? '')

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(item: EnterpriseConnection) {
  editing.value = item as ArtifactConnection
  dialogOpen.value = true
}

function save(input: ArtifactConnectionInput) {
  if (editing.value) {
    store.update({ ...editing.value, ...input } as ArtifactConnection)
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
        <h1>制品空间配置</h1>
        <p>维护代码仓库、Harbor 和 Jenkins 连接，供企业项目阶段 6 使用。</p>
      </div>
      <button class="config-add" type="button" @click="openCreate">
        <i class="pi pi-plus" />
        新增{{ activeLabel }}
      </button>
    </header>

    <p class="settings-notice">配置仅保存在当前浏览器；接入后端前请勿填写真实生产密钥。</p>

    <div class="config-tabs" role="tablist" aria-label="制品空间配置分类">
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

    <ArtifactConnectionDialog
      v-if="dialogOpen"
      :kind="activeTab"
      :connection="editing"
      @close="dialogOpen = false"
      @save="save"
    />
  </section>
</template>
