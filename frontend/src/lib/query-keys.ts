// Central query keys so invalidation stays consistent across features.

export const queryKeys = {
  me: ['auth', 'me'] as const,
  authOptions: ['auth', 'options'] as const,
  navigation: (roleId: string | undefined) => ['navigation', roleId] as const,
  users: {
    all: ['users'] as const,
    list: (params: object) => ['users', 'list', params] as const,
  },
  roles: {
    all: ['roles'] as const,
    list: (params: object) => ['roles', 'list', params] as const,
    options: ['roles', 'options'] as const,
    matrix: (id: string) => ['roles', 'matrix', id] as const,
  },
  menus: {
    all: ['menus'] as const,
    list: (params: object) => ['menus', 'list', params] as const,
    groups: ['menus', 'groups'] as const,
    parents: ['menus', 'parents'] as const,
  },
  profile: ['profile'] as const,
}
