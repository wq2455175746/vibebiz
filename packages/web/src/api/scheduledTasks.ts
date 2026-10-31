import { cpHttp, request } from './client'
import type { ScheduledTask, ScheduledTaskDraft, ScheduledTaskInput } from '@/types'

export function parseScheduledTask(prompt: string): Promise<ScheduledTaskDraft> {
  return request<ScheduledTaskDraft>(cpHttp, {
    method: 'post',
    url: '/scheduled-tasks/parse',
    data: { prompt },
    timeout: 45_000,
  })
}

export function createScheduledTask(input: ScheduledTaskInput): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, {
    method: 'post',
    url: '/scheduled-tasks',
    data: input,
  })
}

export async function listScheduledTasks(): Promise<ScheduledTask[]> {
  const result = await request<{ items: ScheduledTask[] }>(cpHttp, {
    method: 'get',
    url: '/scheduled-tasks',
  })
  return result.items ?? []
}

export function updateScheduledTask(id: number, input: ScheduledTaskInput): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, {
    method: 'put',
    url: `/scheduled-tasks/${id}`,
    data: input,
  })
}

export function pauseScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/pause` })
}

export function resumeScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/resume` })
}

export function stopScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/stop` })
}

export async function deleteScheduledTask(id: number, deleteProject: boolean): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/scheduled-tasks/${id}`,
    params: { deleteProject },
  })
}
