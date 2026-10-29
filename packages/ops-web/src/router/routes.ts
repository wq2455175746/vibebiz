import type { RouteRecordRaw } from 'vue-router'

import AuditView from '@/views/AuditView.vue'
import ModelsView from '@/views/ModelsView.vue'
import RolesView from '@/views/RolesView.vue'
import NotificationsView from '@/views/NotificationsView.vue'
import OverviewView from '@/views/OverviewView.vue'
import ProjectsView from '@/views/ProjectsView.vue'
import QuotasView from '@/views/QuotasView.vue'
import SandboxesView from '@/views/SandboxesView.vue'
import SettingsView from '@/views/SettingsView.vue'
import UsersView from '@/views/UsersView.vue'

export const childRoutes: RouteRecordRaw[] = [
  { path: '', name: 'overview', component: OverviewView, meta: { title: '概览' } },
  { path: 'users', name: 'users', component: UsersView, meta: { title: '用户' } },
  { path: 'projects', name: 'projects', component: ProjectsView, meta: { title: '项目' } },
  {
    path: 'sandboxes',
    name: 'sandboxes',
    component: SandboxesView,
    meta: { title: '沙箱节点' },
  },
  { path: 'models', name: 'models', component: ModelsView, meta: { title: '模型管理' } },
  { path: 'roles', name: 'roles', component: RolesView, meta: { title: '角色管理' } },
  { path: 'quotas', name: 'quotas', component: QuotasView, meta: { title: '配额' } },
  { path: 'audit', name: 'audit', component: AuditView, meta: { title: '审计日志' } },
  {
    path: 'notifications',
    name: 'notifications',
    component: NotificationsView,
    meta: { title: '消息通知' },
  },
  { path: 'settings', name: 'settings', component: SettingsView, meta: { title: '系统设置' } },
]
