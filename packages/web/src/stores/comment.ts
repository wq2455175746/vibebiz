import { defineStore } from 'pinia'
import { ref } from 'vue'

import {
  createCardComment,
  deleteCardComment,
  getCardCommentReplies,
  getCardComments,
  updateCardComment,
} from '@/api/cards'
import type { CardComment, CardCommentDeleteResult } from '@/types'
import { findComment, mergeCommentPage, patchComment, removeComment } from '@/utils/comment'

const ROOT_PAGE_SIZE = 20
const REPLY_PAGE_SIZE = 10

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

export const useCommentStore = defineStore('cardComment', () => {
  const comments = ref<CardComment[]>([])
  const nextCursor = ref('')
  const hasMore = ref(false)
  const replyCursors = ref<Record<string, string>>({})
  const replyHasMore = ref<Record<string, boolean>>({})
  const loading = ref(false)
  const submitting = ref(false)
  const error = ref('')

  async function loadRoots(cardId: string): Promise<void> {
    loading.value = true
    try {
      const page = await getCardComments(cardId)
      comments.value = page.comments
      nextCursor.value = page.nextCursor
      hasMore.value = page.hasMore
      replyCursors.value = {}
      replyHasMore.value = {}
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '评论加载失败')
    } finally {
      loading.value = false
    }
  }

  async function loadMore(cardId: string): Promise<void> {
    if (!hasMore.value || loading.value) return
    loading.value = true
    try {
      const page = await getCardComments(cardId, {
        cursor: nextCursor.value,
        limit: ROOT_PAGE_SIZE,
      })
      comments.value = mergeCommentPage(comments.value, page.comments, true)
      nextCursor.value = page.nextCursor
      hasMore.value = page.hasMore
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '评论加载失败')
    } finally {
      loading.value = false
    }
  }

  async function loadReplies(cardId: string, rootId: string, cursor?: string): Promise<void> {
    const effectiveCursor = cursor ?? replyCursors.value[rootId] ?? ''
    try {
      const page = await getCardCommentReplies(cardId, rootId, {
        cursor: effectiveCursor,
        limit: REPLY_PAGE_SIZE,
      })
      const root = findComment(comments.value, rootId)
      if (!root) return
      comments.value = patchComment(comments.value, rootId, {
        replies: mergeCommentPage(root.replies ?? [], page.comments, Boolean(effectiveCursor)),
      })
      replyCursors.value = { ...replyCursors.value, [rootId]: page.nextCursor }
      replyHasMore.value = { ...replyHasMore.value, [rootId]: page.hasMore }
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '回复加载失败')
      throw cause
    }
  }

  async function create(cardId: string, content: string, parentId?: string): Promise<CardComment> {
    submitting.value = true
    try {
      const created = await createCardComment(cardId, { content, parentId })
      if (parentId) {
        const parent = findComment(comments.value, parentId)
        const targetRootId = parent?.rootId || parentId
        comments.value = patchComment(comments.value, parentId, {
          replies: [...(parent?.replies ?? []), created],
        })
        comments.value = patchComment(comments.value, targetRootId, {
          replyCount: (findComment(comments.value, targetRootId)?.replyCount ?? 0) + 1,
        })
      } else {
        comments.value = [created, ...comments.value]
      }
      error.value = ''
      return created
    } catch (cause) {
      error.value = messageOf(cause, '评论发表失败')
      throw cause
    } finally {
      submitting.value = false
    }
  }

  async function update(cardId: string, commentId: string, content: string): Promise<void> {
    try {
      const updated = await updateCardComment(cardId, commentId, content)
      comments.value = patchComment(comments.value, commentId, updated)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '评论更新失败')
      throw cause
    }
  }

  async function remove(cardId: string, commentId: string): Promise<CardCommentDeleteResult> {
    try {
      const result = await deleteCardComment(cardId, commentId)
      if (result.tombstoned) {
        comments.value = patchComment(comments.value, commentId, {
          status: 'deleted',
          content: '',
          editable: false,
          deletable: false,
        })
      } else {
        comments.value = removeComment(comments.value, commentId)
      }
      error.value = ''
      return result
    } catch (cause) {
      error.value = messageOf(cause, '评论删除失败')
      throw cause
    }
  }

  return {
    comments,
    nextCursor,
    hasMore,
    replyCursors,
    replyHasMore,
    loading,
    submitting,
    error,
    loadRoots,
    loadMore,
    loadReplies,
    create,
    update,
    remove,
  }
})
