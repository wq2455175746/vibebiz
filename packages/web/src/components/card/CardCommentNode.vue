<script setup lang="ts">
import { computed, ref } from 'vue'

import type { CardComment } from '@/types'

const props = defineProps<{
  comment: CardComment
  currentUserId: number | null
  hasMoreReplies?: (commentId: string) => boolean | undefined
}>()
const emit = defineEmits<{
  reply: [comment: CardComment]
  edit: [comment: CardComment, content: string, done: (ok: boolean) => void]
  delete: [comment: CardComment]
  'load-replies': [comment: CardComment]
}>()

const editing = ref(false)
const confirmingDelete = ref(false)
const expanded = ref(false)
const draft = ref(props.comment.content)
const isLong = computed(() => {
  const content = props.comment.content
  return content.split('\n').length > 8 || content.length > 240
})
const collapsed = computed(() => isLong.value && !expanded.value)
const previewCount = computed(() => props.comment.replies?.length ?? 0)
const replyHasMore = computed(() => props.hasMoreReplies?.(props.comment.id))
const remainingReplies = computed(() => Math.max(props.comment.replyCount - previewCount.value, 0))
const showReplyAffordance = computed(() => {
  if (props.comment.replyCount <= 0) return false
  if (replyHasMore.value === true) return true
  if (replyHasMore.value === false) return false
  return remainingReplies.value > 0
})
const replyAffordanceLabel = computed(() =>
  previewCount.value === 0 ? `查看 ${props.comment.replyCount} 条回复` : '加载更多回复',
)

function save() {
  const content = draft.value.trim()
  if (!content) return
  emit('edit', props.comment, content, (ok) => {
    if (ok) {
      draft.value = content
      editing.value = false
    }
  })
}

function cancel() {
  draft.value = props.comment.content
  editing.value = false
}

function requestDelete() {
  confirmingDelete.value = true
}

function confirmDelete() {
  confirmingDelete.value = false
  emit('delete', props.comment)
}

function cancelDelete() {
  confirmingDelete.value = false
}
</script>

<template>
  <article
    class="card-comment-node"
    :style="{ marginLeft: `${Math.min(props.comment.depth - 1, 2) * 20}px` }"
  >
    <img
      v-if="props.comment.author.avatarUrl"
      class="card-comment-avatar"
      :src="props.comment.author.avatarUrl"
      alt=""
    />
    <span v-else class="card-comment-avatar">{{
      props.comment.author.name.charAt(0).toUpperCase()
    }}</span>
    <div class="card-comment-body">
      <strong>{{ props.comment.author.name }}</strong>
      <span
        v-if="props.comment.depth === 3 && props.comment.replyToName"
        class="card-comment-reply-to"
      >
        回复 @{{ props.comment.replyToName }}
      </span>
      <span v-if="props.comment.editedAt" class="card-comment-edited">已编辑</span>
      <p v-if="props.comment.status === 'deleted'" class="card-comment-deleted">该评论已删除</p>
      <template v-else>
        <textarea v-if="editing" v-model="draft" maxlength="500" />
        <p v-else class="card-comment-content" :class="{ 'is-collapsed': collapsed }">{{ props.comment.content }}</p>
        <button
          v-if="!editing && isLong"
          type="button"
          class="card-comment-more"
          @click="expanded = !expanded"
        >
          {{ expanded ? '收起' : '展开' }}
        </button>
        <div class="card-comment-actions">
          <button type="button" @click="emit('reply', props.comment)">回复</button>
          <button v-if="props.comment.editable" type="button" @click="editing = true">编辑</button>
          <template v-if="props.comment.deletable">
            <button v-if="!confirmingDelete" type="button" @click="requestDelete">删除</button>
            <button v-else type="button" @click="confirmDelete">确认删除</button>
            <button v-if="confirmingDelete" type="button" @click="cancelDelete">取消</button>
          </template>
          <button v-if="editing" type="button" @click="save">保存</button>
          <button v-if="editing" type="button" @click="cancel">取消</button>
        </div>
      </template>
      <button
        v-if="showReplyAffordance"
        type="button"
        class="card-comment-more"
        @click="emit('load-replies', props.comment)"
      >
        {{ replyAffordanceLabel }}
      </button>
      <CardCommentNode
        v-for="reply in props.comment.replies"
        :key="reply.id"
        :comment="reply"
        :current-user-id="props.currentUserId"
        :has-more-replies="props.hasMoreReplies"
        @reply="emit('reply', $event)"
        @edit="(item, content, done) => emit('edit', item, content, done)"
        @delete="(item) => emit('delete', item)"
        @load-replies="(item) => emit('load-replies', item)"
      />
    </div>
  </article>
</template>
