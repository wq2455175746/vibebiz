<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import type { ArtifactFlowState, ArtifactStepKey, MockStepStatus } from '@/types/enterpriseSpace'
import {
  artifactReady,
  artifactStepUnlocked,
  createEmptyStep,
  dockerPullCommand,
  downloadTextFile,
} from '@/utils/enterpriseFlow'

const props = defineProps<{
  projectId: string
  flow: ArtifactFlowState
}>()

const emit = defineEmits<{
  (event: 'confirmed'): void
}>()

const connections = useEnterpriseConnectionsStore()
const flows = useEnterpriseFlowStore()
const error = ref('')
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null
let runningStep: ArtifactStepKey | null = null
let copyTimer: ReturnType<typeof setTimeout> | null = null

const selectedRepo = computed(() =>
  connections.codeRepos.find((item) => item.id === props.flow.codeRepoId),
)
const selectedJenkins = computed(() =>
  connections.jenkins.find((item) => item.id === props.flow.jenkinsId),
)
const selectedHarbor = computed(() =>
  connections.harbors.find((item) => item.id === props.flow.harborId),
)
const ready = computed(() => artifactReady(props.flow))
const running = computed(
  () =>
    props.flow.code.status === 'running' ||
    props.flow.jenkins.status === 'running' ||
    props.flow.harbor.status === 'running',
)
const dockerCommand = computed(() =>
  selectedHarbor.value ? dockerPullCommand(props.flow, selectedHarbor.value) : '',
)

function statusText(status: MockStepStatus): string {
  if (status === 'running') return '执行中'
  if (status === 'success') return '已完成'
  if (status === 'error') return '失败'
  return '待执行'
}

function inputValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function selectValue(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

function updateField(
  field: 'branch' | 'commitMessage' | 'jenkinsJob' | 'image' | 'tag',
  value: string,
): void {
  const patch: Partial<ArtifactFlowState> = {}
  if (field === 'branch') patch.branch = value
  if (field === 'commitMessage') patch.commitMessage = value
  if (field === 'jenkinsJob') patch.jenkinsJob = value
  if (field === 'image') patch.image = value
  if (field === 'tag') patch.tag = value
  flows.patchArtifact(props.projectId, patch)
}

function selectRepo(event: Event) {
  const id = selectValue(event)
  const repo = connections.codeRepos.find((item) => item.id === id)
  flows.patchArtifact(props.projectId, {
    codeRepoId: id,
    branch: repo?.defaultBranch ?? '',
  })
}

function selectJenkins(event: Event) {
  const id = selectValue(event)
  flows.patchArtifact(props.projectId, { jenkinsId: id, jenkinsJob: '' })
}

function selectHarbor(event: Event) {
  flows.patchArtifact(props.projectId, {
    harborId: selectValue(event),
  })
}

function startStep(step: ArtifactStepKey): void {
  if (running.value || props.flow.status === 'done') return
  if (step === 'code' && (!props.flow.codeRepoId || !props.flow.branch.trim())) {
    error.value = '请选择代码仓库并填写分支'
    return
  }
  if (step === 'jenkins' && (!props.flow.jenkinsId || !props.flow.jenkinsJob.trim())) {
    error.value = '请选择 Jenkins 配置和任务'
    return
  }
  if (step === 'harbor' && (!props.flow.harborId || !props.flow.image.trim())) {
    error.value = '请选择 Harbor 并填写镜像名'
    return
  }
  error.value = ''
  const startedAt = new Date().toISOString()
  flows.patchArtifactStep(props.projectId, step, {
    status: 'running',
    startedAt,
    completedAt: null,
    error: '',
  })
  runningStep = step
  timer = setTimeout(() => {
    flows.patchArtifactStep(props.projectId, step, {
      status: 'success',
      completedAt: new Date().toISOString(),
    })
    runningStep = null
    timer = null
  }, 1100)
}

function confirm() {
  if (!ready.value) {
    error.value = '三步全部成功后才能确认制品'
    return
  }
  error.value = ''
  emit('confirmed')
}

async function copyCommand() {
  if (!dockerCommand.value) return
  await navigator.clipboard?.writeText(dockerCommand.value)
  copied.value = true
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copied.value = false
  }, 1500)
}

function downloadCode() {
  downloadTextFile(
    `vibebiz-${props.projectId}-source-demo.txt`,
    `Vibebiz 代码下载演示文件\n项目：${props.projectId}\n仓库：${selectedRepo.value?.url ?? '-'}\n分支：${props.flow.branch}\n\n前端演示文件，未连接真实代码仓库。`,
  )
}

function downloadHelm() {
  const image = props.flow.image.replace(/[^\w.-]+/g, '-') || 'image'
  const tag = props.flow.tag.replace(/[^\w.-]+/g, '-') || 'latest'
  downloadTextFile(
    `vibebiz-${image}-${tag}-helm-demo.yaml`,
    `# Vibebiz Helm 下载演示文件\n# 前端演示文件，未连接真实构建系统\nimage: ${dockerCommand.value || '-'}\nnamespace: ${selectedHarbor.value?.namespace ?? '-'}\n`,
  )
}

onMounted(() => {
  for (const step of ['code', 'jenkins', 'harbor'] as ArtifactStepKey[]) {
    if (props.flow[step].status === 'running') {
      flows.patchArtifactStep(props.projectId, step, {
        ...createEmptyStep(),
      })
    }
  }
})

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
  if (copyTimer) clearTimeout(copyTimer)
  if (runningStep) {
    flows.patchArtifactStep(props.projectId, runningStep, {
      ...createEmptyStep(),
    })
  }
})
</script>

<template>
  <section class="artifact-workspace">
    <header class="pipeline-head">
      <div>
        <p class="enterprise-stage-eyebrow">阶段 6</p>
        <h1>制品空间</h1>
        <p>按顺序上传代码、触发 Jenkins 构建，并把镜像推送到 Harbor。</p>
      </div>
      <button
        class="enterprise-primary"
        type="button"
        :disabled="!ready || flow.status === 'done'"
        @click="confirm"
      >
        <i :class="flow.status === 'done' ? 'pi pi-check' : 'pi pi-flag'" />
        {{ flow.status === 'done' ? '制品已完成' : '确认制品完成' }}
      </button>
    </header>

    <p v-if="error" class="enterprise-stage-error">{{ error }}</p>

    <div class="pipeline-list">
      <article class="pipeline-card" :class="{ done: flow.code.status === 'success' }">
        <span class="pipeline-index">1</span>
        <div class="pipeline-body">
          <header>
            <strong>上传代码</strong>
            <span class="pipeline-status">{{ statusText(flow.code.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.codeRepoId"
              :disabled="!artifactStepUnlocked(flow, 'code') || flow.code.status === 'success'"
              @change="selectRepo"
            >
              <option value="">选择代码仓库</option>
              <option v-for="item in connections.codeRepos" :key="item.id" :value="item.id">
                {{ item.name }} · {{ item.provider }}
              </option>
            </select>
            <input
              :value="flow.branch"
              class="pipeline-input"
              placeholder="分支"
              :disabled="flow.code.status === 'success'"
              @input="updateField('branch', inputValue($event))"
            />
            <input
              :value="flow.commitMessage"
              class="pipeline-input"
              placeholder="提交信息"
              :disabled="flow.code.status === 'success'"
              @input="updateField('commitMessage', inputValue($event))"
            />
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'code') ||
              running ||
              flow.code.status === 'success' ||
              !connections.codeRepos.length
            "
            @click="startStep('code')"
          >
            上传代码
          </button>
          <p v-if="!connections.codeRepos.length" class="pipeline-hint">
            请先在设置中配置代码仓库。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>

      <article class="pipeline-card" :class="{ done: flow.jenkins.status === 'success' }">
        <span class="pipeline-index">2</span>
        <div class="pipeline-body">
          <header>
            <strong>Jenkins 构建</strong>
            <span class="pipeline-status">{{ statusText(flow.jenkins.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.jenkinsId"
              :disabled="
                !artifactStepUnlocked(flow, 'jenkins') || flow.jenkins.status === 'success'
              "
              @change="selectJenkins"
            >
              <option value="">选择 Jenkins</option>
              <option v-for="item in connections.jenkins" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
            <select
              :value="flow.jenkinsJob"
              class="pipeline-input"
              :disabled="
                !artifactStepUnlocked(flow, 'jenkins') ||
                !selectedJenkins ||
                flow.jenkins.status === 'success'
              "
              @change="updateField('jenkinsJob', selectValue($event))"
            >
              <option value="">选择任务</option>
              <option v-for="job in selectedJenkins?.jobs ?? []" :key="job" :value="job">
                {{ job }}
              </option>
            </select>
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'jenkins') ||
              running ||
              flow.jenkins.status === 'success' ||
              !connections.jenkins.length
            "
            @click="startStep('jenkins')"
          >
            开始构建
          </button>
          <p v-if="!connections.jenkins.length" class="pipeline-hint">
            请先在设置中配置 Jenkins。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>

      <article class="pipeline-card" :class="{ done: flow.harbor.status === 'success' }">
        <span class="pipeline-index">3</span>
        <div class="pipeline-body">
          <header>
            <strong>Harbor 推送</strong>
            <span class="pipeline-status">{{ statusText(flow.harbor.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.harborId"
              :disabled="!artifactStepUnlocked(flow, 'harbor') || flow.harbor.status === 'success'"
              @change="selectHarbor"
            >
              <option value="">选择 Harbor</option>
              <option v-for="item in connections.harbors" :key="item.id" :value="item.id">
                {{ item.name }} · {{ item.namespace }}
              </option>
            </select>
            <input
              :value="flow.image"
              class="pipeline-input"
              placeholder="镜像名"
              :disabled="flow.harbor.status === 'success'"
              @input="updateField('image', inputValue($event))"
            />
            <input
              :value="flow.tag"
              class="pipeline-input"
              placeholder="Tag"
              :disabled="flow.harbor.status === 'success'"
              @input="updateField('tag', inputValue($event))"
            />
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'harbor') ||
              running ||
              flow.harbor.status === 'success' ||
              !connections.harbors.length
            "
            @click="startStep('harbor')"
          >
            推送镜像
          </button>
          <p v-if="!connections.harbors.length" class="pipeline-hint">
            请先在设置中配置 Harbor。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>
    </div>

    <section v-if="ready" class="pipeline-result">
      <h2>制品已就绪</h2>
      <code>{{ dockerCommand }}</code>
      <div class="pipeline-result-actions">
        <button class="enterprise-secondary" type="button" @click="copyCommand">
          <i class="pi pi-copy" />
          {{ copied ? '已复制' : '复制 Docker 命令' }}
        </button>
        <button class="enterprise-secondary" type="button" @click="downloadCode">
          <i class="pi pi-download" />
          手动下载代码
        </button>
        <button class="enterprise-secondary" type="button" @click="downloadHelm">
          <i class="pi pi-download" />
          下载 Helm 文件
        </button>
      </div>
    </section>
  </section>
</template>
