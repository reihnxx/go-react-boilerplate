import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type QueryValue } from '@/lib/api'
import { queryKeys } from '@/lib/query-keys'
import type { Envelope, ListEnvelope, Menu, MenuGroup } from '@/types/api'

export interface MenuInput {
  name: string
  slug?: string
  type: 'main' | 'child'
  group_id: string | null
  parent_id: string | null
  path: string
  icon: string
  sort_order: number | null
  is_active: boolean
}

export function useMenus(query: Record<string, QueryValue>) {
  return useQuery({
    queryKey: queryKeys.menus.list(query),
    queryFn: ({ signal }) => api.get<ListEnvelope<Menu>>('/menus', query, signal),
    placeholderData: keepPreviousData,
  })
}

export function useMenuGroups() {
  return useQuery({
    queryKey: queryKeys.menus.groups,
    queryFn: async () => (await api.get<Envelope<MenuGroup[]>>('/menu-groups')).data,
    staleTime: 5 * 60_000,
  })
}

/** Main menus (possible parents for a submenu). */
export function useParentMenus() {
  return useQuery({
    queryKey: queryKeys.menus.parents,
    queryFn: async () => (await api.get<ListEnvelope<Menu>>('/menus', { type: 'main', per_page: 100 })).data,
  })
}

function useMenuMutation<TVars>(fn: (vars: TVars) => Promise<unknown>) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.menus.all })
      qc.invalidateQueries({ queryKey: ['navigation'] })
      qc.invalidateQueries({ queryKey: queryKeys.roles.all })
    },
  })
}

function body(input: MenuInput) {
  return {
    ...input,
    sort_order: input.sort_order ?? undefined,
    group_id: input.type === 'main' ? input.group_id : null,
    parent_id: input.type === 'child' ? input.parent_id : null,
  }
}

export const useCreateMenu = () => useMenuMutation((input: MenuInput) => api.post<Envelope<Menu>>('/menus', body(input)))
export const useUpdateMenu = () =>
  useMenuMutation(({ id, ...input }: MenuInput & { id: string }) => api.patch<Envelope<Menu>>(`/menus/${id}`, body(input)))
export const useDeleteMenu = () => useMenuMutation((id: string) => api.delete(`/menus/${id}`))
