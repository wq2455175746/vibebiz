import { cpHttp, request } from './client'
import type {
  BusinessCard,
  CardComment,
  CardCommentDeleteResult,
  CardCommentPage,
  CardLikeState,
  DiscoverCardsResult,
  PublicProjectCard,
} from '@/types'

export function getMyCard(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'get', url: '/me/card' })
}

export function updateMyCard(input: { headline: string; tags: string[] }): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'put', url: '/me/card', data: input })
}

export function addCardProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'post',
    url: '/me/card/items',
    data: { projectId },
  })
}

export function removeCardProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'delete',
    url: `/me/card/items/${encodeURIComponent(projectId)}`,
  })
}

export function setCardFeaturedProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'put',
    url: '/me/card/featured',
    data: { kind: 'project', projectId },
  })
}

export function uploadCardFeatured(file: File): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'post',
    url: '/me/card/featured/upload',
    data: file,
    headers: {
      'Content-Type': file.type,
      'X-File-Name': encodeURIComponent(file.name),
    },
    timeout: 30_000,
  })
}

export function clearCardFeatured(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'delete', url: '/me/card/featured' })
}

export function publishMyCard(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'post', url: '/me/card/publish' })
}

export function getDiscoverCards(
  params: { page?: number; pageSize?: number } = {},
): Promise<DiscoverCardsResult> {
  return request<DiscoverCardsResult>(cpHttp, {
    method: 'get',
    url: '/cards/discover',
    params,
  })
}

export function getPublicCard(id: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(id)}`,
  })
}

export function getPublicCardProject(projectId: string): Promise<PublicProjectCard> {
  return request<PublicProjectCard>(cpHttp, {
    method: 'get',
    url: `/public/card-projects/${encodeURIComponent(projectId)}`,
  })
}

export function getCardComments(
  cardId: string,
  params: { cursor?: string; limit?: number } = {},
): Promise<CardCommentPage> {
  return request<CardCommentPage>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments`,
    params,
  })
}

export function getCardCommentReplies(
  cardId: string,
  commentId: string,
  params: { cursor?: string; limit?: number } = {},
): Promise<CardCommentPage> {
  return request<CardCommentPage>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}/replies`,
    params,
  })
}

export function createCardComment(
  cardId: string,
  input: { content: string; parentId?: string },
): Promise<CardComment> {
  return request<CardComment>(cpHttp, {
    method: 'post',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments`,
    data: input,
  })
}

export function updateCardComment(
  cardId: string,
  commentId: string,
  content: string,
): Promise<CardComment> {
  return request<CardComment>(cpHttp, {
    method: 'put',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`,
    data: { content },
  })
}

export function deleteCardComment(
  cardId: string,
  commentId: string,
): Promise<CardCommentDeleteResult> {
  return request<CardCommentDeleteResult>(cpHttp, {
    method: 'delete',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`,
  })
}

export function likeCard(cardId: string): Promise<CardLikeState> {
  return request<CardLikeState>(cpHttp, {
    method: 'put',
    url: `/public/cards/${encodeURIComponent(cardId)}/like`,
  })
}

export function unlikeCard(cardId: string): Promise<CardLikeState> {
  return request<CardLikeState>(cpHttp, {
    method: 'delete',
    url: `/public/cards/${encodeURIComponent(cardId)}/like`,
  })
}
