// Shapes returned by the Go API (see backend/api/openapi.yaml).

export type Action = 'view' | 'create' | 'update' | 'delete' | 'download'

export interface PageMeta {
  page: number
  per_page: number
  total: number
  total_pages: number
}

export interface Envelope<T> {
  data: T
}

export interface ListEnvelope<T> {
  data: T[]
  meta: PageMeta
}

export interface RoleRef {
  id: string
  name: string
  slug: string
  is_super_admin: boolean
}

export interface MeUser {
  id: string
  name: string
  username: string
  email: string
}

export interface Me {
  user: MeUser
  roles: RoleRef[]
  active_role: RoleRef | null
  requires_role_selection: boolean
  is_super_admin: boolean
  permissions: Record<string, Action[]>
  csrf_token: string
  impersonator: MeUser | null
}

export interface AuthOptions {
  app_name: string
  registration_enabled: boolean
}

export interface User {
  id: string
  name: string
  username: string
  email: string
  is_active: boolean
  last_login_at: string | null
  created_at: string
  updated_at: string
  roles: RoleRef[]
}

export interface Role {
  id: string
  name: string
  slug: string
  is_super_admin: boolean
  is_system: boolean
  is_active: boolean
  user_count: number
  created_at: string
  updated_at: string
}

export interface ActionInfo {
  code: Action
  label: string
}

export interface MatrixMenu {
  id: string
  name: string
  slug: string
  is_active: boolean
  granted: Action[]
  children: MatrixMenu[]
}

export interface PermissionMatrix {
  role: Role
  read_only: boolean
  actions: ActionInfo[]
  menus: MatrixMenu[]
}

export interface MenuGroup {
  id: string
  name: string
  sort_order: number
}

export interface Menu {
  id: string
  type: 'main' | 'child'
  group_id: string | null
  group_name: string | null
  parent_id: string | null
  parent_name: string | null
  name: string
  slug: string
  path: string | null
  icon: string | null
  sort_order: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface NavItem {
  id: string
  name: string
  slug: string
  path: string | null
  icon: string | null
  children: NavItem[]
}

export interface NavGroup {
  id: string
  name: string
  items: NavItem[]
}

export interface Profile {
  id: string
  name: string
  username: string
  email: string
  phone: string | null
  birth_place: string | null
  birth_date: string | null
  address: string | null
}
