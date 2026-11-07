import type { RouteRecordRaw } from 'vue-router'

export interface SettingsMenuItem {
  label: string
  icon: string
  to: string
  name: string
  group: string
}

export const settingsMenu: SettingsMenuItem[] = [
  {
    label: '个人信息',
    icon: 'pi pi-user',
    to: '/settings/profile',
    name: 'settings-profile',
    group: '账户',
  },
  {
    label: '通知渠道',
    icon: 'pi pi-bell',
    to: '/settings/notification-channels',
    name: 'settings-notification-channels',
    group: '账户',
  },
  {
    label: '制品空间配置',
    icon: 'pi pi-box',
    to: '/settings/artifact-space',
    name: 'settings-artifact-space',
    group: '企业工程',
  },
  {
    label: '交付空间配置',
    icon: 'pi pi-send',
    to: '/settings/delivery-space',
    name: 'settings-delivery-space',
    group: '企业工程',
  },
  {
    label: '磁盘空间',
    icon: 'pi pi-database',
    to: '/settings/storage',
    name: 'settings-storage',
    group: '工作区',
  },
  {
    label: '工作区设置',
    icon: 'pi pi-briefcase',
    to: '/settings/workspace',
    name: 'settings-workspace',
    group: '工作区',
  },
  {
    label: '成员',
    icon: 'pi pi-users',
    to: '/settings/members',
    name: 'settings-members',
    group: '成员与访问权限',
  },
]

export const settingsGroups = [...new Set(settingsMenu.map((item) => item.group))]

export const settingsRoute: RouteRecordRaw = {
  path: '/settings',
  component: () => import('@/layouts/SettingsLayout.vue'),
  children: [
    { path: '', name: 'settings', redirect: '/settings/profile' },
    {
      path: 'profile',
      name: 'settings-profile',
      component: () => import('@/views/settings/ProfileView.vue'),
    },
    {
      path: 'notification-channels',
      name: 'settings-notification-channels',
      component: () => import('@/views/settings/NotificationChannelsView.vue'),
    },
    {
      path: 'artifact-space',
      name: 'settings-artifact-space',
      component: () => import('@/views/settings/ArtifactSpaceConfigView.vue'),
    },
    {
      path: 'delivery-space',
      name: 'settings-delivery-space',
      component: () => import('@/views/settings/DeliverySpaceConfigView.vue'),
    },
    {
      path: 'storage',
      name: 'settings-storage',
      component: () => import('@/views/settings/StorageView.vue'),
    },
    {
      path: 'workspace',
      name: 'settings-workspace',
      component: () => import('@/views/settings/WorkspaceView.vue'),
    },
    {
      path: 'members',
      name: 'settings-members',
      component: () => import('@/views/settings/MembersView.vue'),
    },
  ],
}
