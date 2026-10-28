export interface NavItem {
  name: string
  label: string
  path: string
  icon: string
}

export const navItems: NavItem[] = [
  { name: 'overview', label: '概览', path: '/', icon: 'pi pi-chart-bar' },
  { name: 'users', label: '用户', path: '/users', icon: 'pi pi-users' },
  { name: 'projects', label: '项目', path: '/projects', icon: 'pi pi-folder' },
  { name: 'sandboxes', label: '沙箱节点', path: '/sandboxes', icon: 'pi pi-server' },
  { name: 'quotas', label: '配额', path: '/quotas', icon: 'pi pi-gauge' },
  { name: 'audit', label: '审计日志', path: '/audit', icon: 'pi pi-list' },
  { name: 'settings', label: '系统设置', path: '/settings', icon: 'pi pi-cog' },
]
