<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { computed, ref, watch } from 'vue'

import type { EventSourceConfig, EventSourceKind } from '@/types/superTeam'
import { sourceFields, sourceMeta } from '@/utils/superTeam'

const props = defineProps<{ visible: boolean; source: EventSourceConfig | null }>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  save: [kind: EventSourceKind, fields: Record<string, string>]
}>()

const form = ref<Record<string, string>>({})
const error = ref('')

watch(
  () => props.source,
  (source) => {
    form.value = source ? { ...source.fields } : {}
    error.value = ''
  },
  { immediate: true },
)

const fields = computed(() => (props.source ? sourceFields(props.source.kind) : []))
const meta = computed(() => (props.source ? sourceMeta(props.source.kind) : null))

function close(): void {
  emit('update:visible', false)
}

function save(): void {
  if (!props.source || !meta.value) {
    return
  }
  for (const field of fields.value) {
    if (!form.value[field.key] || !form.value[field.key].trim()) {
      error.value = `请填写${field.label}`
      return
    }
  }
  emit('save', props.source.kind, { ...form.value })
  emit('update:visible', false)
}
</script>

<template>
  <Dialog
    :visible="visible"
    :header="meta ? `配置${meta.label}事件入口` : '配置事件入口'"
    modal
    :style="{ width: '560px' }"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="superteam-dialog-note">
      配置归属当前用户，保存在本地演示存储；ops-web 仅可查看，不可编辑。
    </p>
    <div class="superteam-dialog-form">
      <label v-for="field in fields" :key="field.key" class="superteam-field">
        <span>{{ field.label }}</span>
        <InputText
          v-model="form[field.key]"
          :type="field.secret ? 'password' : 'text'"
          :placeholder="field.placeholder"
        />
      </label>
      <p v-if="error" class="superteam-error" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <Button label="取消" text @click="close" />
      <Button label="保存配置" @click="save" />
    </template>
  </Dialog>
</template>
