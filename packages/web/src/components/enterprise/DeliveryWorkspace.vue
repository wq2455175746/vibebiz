<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import { useFdeResourcesStore } from '@/stores/fdeResources'
import type {
  DeliveryFlowState,
  DeliveryMethodKind,
  MockStepStatus,
} from '@/types/enterpriseSpace'
import { createEmptyStep, deliveryReady } from '@/utils/enterpriseFlow'

const props = defineProps<{
  projectId: string
  flow: DeliveryFlowState
}>()

const emit = defineEmits<{
  (event: 'confirmed'): void
}>()

const connections = useEnterpriseConnectionsStore()
const fdeResources = useFdeResourcesStore()
const flows = useEnterpriseFlowStore()
const error = ref('')
let timer: ReturnType<typeof setTimeout> | null = null

const methods: { key: DeliveryMethodKind; title: string; description: string }[] = [
  { key: 'k8s', title: 'K8s 集群', description: '部署到已注册的 Kubernetes 集群。' },
  { key: 'cloud', title: '云主机', description: '部署到已注册的云主机节点。' },
  { key: 'argo', title: 'Argo', description: '触发 Argo GitOps 交付。' },
  { key: 'fde', title: 'FDE 人工', description: '转交 FDE 完成人工交付。' },
]

const ready = computed(() => deliveryReady(props.flow))
const running = computed(() => props.flow.execution.status === 'running')
const selectedLabel = computed(() => {
  const method = methods.find((item) => item.key === props.flow.method)
  return method?.title ?? ''
})

function statusText(status: MockStepStatus): string {
  if (status === 'running') return '执行中'
  if (status === 'success') return '已完成'
  if (status === 'error') return '失败'
  return '待执行'
}

function targets(kind: DeliveryMethodKind) {
  if (kind === 'k8s') return connections.k8sClusters
  if (kind === 'cloud') return connections.cloudNodes
  if (kind === 'argo') return connections.argoes
  return fdeResources.items
}

const activeTargets = computed(() => (props.flow.method ? targets(props.flow.method) : []))
const selectedTarget = computed(() =>
  activeTargets.value.find((item) => String(item.id) === props.flow.targetId),
)

function inputValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function updateDeliveryField(field: 'namespace' | 'note', value: string): void {
  const patch: Partial<DeliveryFlowState> = {}
  if (field === 'namespace') patch.namespace = value
  if (field === 'note') patch.note = value
  flows.patchDelivery(props.projectId, patch)
}

function selectMethod(kind: DeliveryMethodKind) {
  if (props.flow.execution.status === 'success' || props.flow.status === 'done') return
  flows.patchDelivery(props.projectId, {
    method: kind,
    targetId: '',
    namespace: '',
    note: '',
    execution: createEmptyStep(),
  })
  error.value = ''
}

function selectTarget(event: Event) {
  const targetId = (event.target as HTMLSelectElement).value
  const k8s = connections.k8sClusters.find((item) => item.id === targetId)
  flows.patchDelivery(props.projectId, {
    targetId,
    namespace: k8s?.defaultNamespace ?? props.flow.namespace,
  })
}

function execute() {
  if (!props.flow.method || !props.flow.targetId) {
    error.value = '请选择交付方式和目标'
    return
  }
  if (!selectedTarget.value) {
    error.value = props.flow.method === 'fde' ? '所选 FDE 已停用，请重新选择' : '所选配置已失效，请重新选择'
    return
  }
  if (running.value) return
  error.value = ''
  const startedAt = new Date().toISOString()
  flows.patchDeliveryExecution(props.projectId, {
    status: 'running',
    startedAt,
    completedAt: null,
    error: '',
  })
  if (props.flow.method === 'fde') return
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    flows.patchDeliveryExecution(props.projectId, {
      status: 'success',
      completedAt: new Date().toISOString(),
    })
    timer = null
  }, 1100)
}

function markManualDone() {
  if (props.flow.method !== 'fde' || props.flow.execution.status !== 'running') return
  flows.patchDeliveryExecution(props.projectId, {
    status: 'success',
    completedAt: new Date().toISOString(),
  })
}

function confirm() {
  if (!ready.value) {
    error.value = '交付执行成功后才能确认'
    return
  }
  error.value = ''
  emit('confirmed')
}

onMounted(() => {
  if (fdeResources.items.length === 0) {
    void fdeResources.load()
  }
})

onBeforeUnmount(() => {
  if (timer) {
    clearTimeout(timer)
    flows.patchDeliveryExecution(props.projectId, { ...createEmptyStep() })
  }
})
</script>

<template>
  <section class="delivery-workspace">
    <header class="pipeline-head">
      <div>
        <p class="enterprise-stage-eyebrow">阶段 7</p>
        <h1>交付空间</h1>
        <p>选择一种交付方式，执行成功并确认后完成企业项目交付。</p>
      </div>
      <button
        class="enterprise-primary"
        type="button"
        :disabled="!ready || flow.status === 'done'"
        @click="confirm"
      >
        <i :class="flow.status === 'done' ? 'pi pi-check' : 'pi pi-flag'" />
        {{ flow.status === 'done' ? '交付已完成' : '确认交付完成' }}
      </button>
    </header>

    <p v-if="error" class="enterprise-stage-error">{{ error }}</p>

    <div class="delivery-methods">
      <button
        v-for="method in methods"
        :key="method.key"
        type="button"
        class="delivery-method"
        :class="{ active: flow.method === method.key }"
        :disabled="running || flow.execution.status === 'success' || flow.status === 'done'"
        @click="selectMethod(method.key)"
      >
        <strong>{{ method.title }}</strong>
        <span>{{ method.description }}</span>
        <small>{{ targets(method.key).length }} {{ method.key === 'fde' ? '位可用人员' : '个可用配置' }}</small>
      </button>
    </div>

    <section v-if="flow.method" class="pipeline-card delivery-target">
      <span class="pipeline-index">→</span>
      <div class="pipeline-body">
        <header>
          <strong>交付到{{ selectedLabel }}</strong>
          <span class="pipeline-status">{{ statusText(flow.execution.status) }}</span>
        </header>

        <div class="pipeline-fields">
          <select
            class="pipeline-input"
            :value="flow.targetId"
            :disabled="running || flow.execution.status === 'success'"
            @change="selectTarget"
          >
            <option value="">选择目标</option>
            <option v-for="item in activeTargets" :key="item.id" :value="String(item.id)">
              {{ item.name }}
            </option>
          </select>
          <input
            v-if="flow.method === 'k8s'"
            :value="flow.namespace"
            class="pipeline-input"
            placeholder="命名空间"
            :disabled="running || flow.execution.status === 'success'"
            @input="updateDeliveryField('namespace', inputValue($event))"
          />
          <input
            v-if="flow.method === 'fde'"
            :value="flow.note"
            class="pipeline-input"
            placeholder="交付备注"
            :disabled="running || flow.execution.status === 'success'"
            @input="updateDeliveryField('note', inputValue($event))"
          />
        </div>

        <button
          v-if="flow.execution.status !== 'success'"
          class="pipeline-run"
          type="button"
          :disabled="!flow.targetId || (flow.method !== 'fde' && running)"
          @click="
            flow.method === 'fde' && flow.execution.status === 'running'
              ? markManualDone()
              : execute()
          "
        >
          {{
            flow.method === 'fde' && flow.execution.status === 'running'
              ? '标记人工完成'
              : '执行交付'
          }}
        </button>

        <p v-if="!activeTargets.length" class="pipeline-hint">
          <template v-if="flow.method === 'fde'">
            运营尚未配置可用的 FDE 人员，请联系运营。
          </template>
          <template v-else>
            请先在设置中配置{{ selectedLabel }}。<RouterLink to="/settings/delivery-space">去配置</RouterLink>
          </template>
        </p>
        <p v-if="fdeResources.error && flow.method === 'fde'" class="pipeline-hint">
          {{ fdeResources.error }}
        </p>
      </div>
    </section>

    <section v-if="ready" class="pipeline-result">
      <h2>交付完成</h2>
      <p>方式：{{ selectedLabel }} · 目标：{{ selectedTarget?.name ?? flow.targetId }}</p>
      <p v-if="flow.namespace">命名空间：{{ flow.namespace }}</p>
      <p v-if="flow.note">备注：{{ flow.note }}</p>
    </section>
  </section>
</template>
