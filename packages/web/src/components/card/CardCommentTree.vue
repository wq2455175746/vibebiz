<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import CardCommentNode from './CardCommentNode.vue'
import { useCommentStore } from '@/stores/comment'
import { countComments } from '@/utils/comment'
import type { CardComment } from '@/types'

const props = defineProps<{ cardId: string; currentUserId: number | null; commentCount: number }>()
const emit = defineEmits<{
  'count-change': [count: number]
  'login-required': []
}>()
const store = useCommentStore()
const draft = ref('')
const replyTo = ref<CardComment | null>(null)
const loadedCount = computed(() => countComments(store.comments))

onMounted(() => void store.loadRoots(props.cardId))

async function submit() {
  if (props.currentUserId === null) {
    emit('login-required')
    return
  }
  if (!draft.value.trim()) return
  try {
    await store.create(props.cardId, draft.value, replyTo.value?.id)
    draft.value = ''
    replyTo.value = null
    emit('count-change', props.commentCount + 1)
  } catch {
    // store.error already carries the message
  }
}

async function update(item: CardComment, content: string, done: (ok: boolean) => void) {
  try {
    await store.update(props.cardId, item.id, content)
    done(true)
  } catch {
    done(false)
  }
}

function selectReply(item: CardComment) {
  if (props.currentUserId === null) {
    emit('login-required')
    return
  }
  replyTo.value = item
}

function hasMoreReplies(commentId: string): boolean | undefined {
  return store.replyHasMore[commentId]
}

async function remove(item: CardComment) {
  try {
    const result = await store.remove(props.cardId, item.id)
    emit('count-change', result.commentCount)
  } catch {
    // keep the comment when the delete fails
  }
}

async function loadReplies(item: CardComment) {
  try {
    await store.loadReplies(props.cardId, item.id)
  } catch {
    // keep the loaded replies when fetching fails
  }
}
</script>

<template>
  <section class="card-comments" id="comments">
    <header><h2>评论</h2></header>
    <form class="card-comment-form" @submit.prevent="submit">
      <textarea v-model="draft" maxlength="500" placeholder="写下你的评论" />
      <p v-if="replyTo" class="card-comment-reply-hint">
        回复 @{{ replyTo.author.name }}
        <button type="button" @click="replyTo = null">取消</button>
      </p>
      <button type="submit" :disabled="store.submitting">发表</button>
    </form>
    <p v-if="store.error" class="error-text">{{ store.error }}</p>
    <p v-if="!store.loading && store.comments.length === 0" class="card-empty">还没有评论。</p>
    <CardCommentNode
      v-for="comment in store.comments"
      :key="comment.id"
      :comment="comment"
      :current-user-id="currentUserId"
      :has-more-replies="hasMoreReplies"
      @reply="selectReply"
      @edit="update"
      @delete="remove"
      @load-replies="loadReplies"
    />
    <p v-if="loadedCount >= 200" class="card-comments-hint">
      已加载 {{ loadedCount }} 条评论，继续加载可能影响页面性能。
    </p>
    <button
      v-if="store.hasMore"
      class="card-secondary"
      type="button"
      @click="store.loadMore(props.cardId)"
    >
      加载更多评论
    </button>
  </section>
</template>
