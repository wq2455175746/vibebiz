import { cpHttp, request } from './client'
import type { BusinessCard, DiscoverCardsResult, PublicProjectCard } from '@/types'

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
