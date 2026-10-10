import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type QueryValue } from '@/lib/api'
import { queryKeys } from '@/lib/query-keys'
import type { Action, Envelope, ListEnvelope, PermissionMatrix, Role } from '@/types/api'

export function useRoles(query: Record<string, QueryValue>) {
  return useQuery({
    queryKey: queryKeys.roles.list(query),
    queryFn: ({ signal }) => api.get<ListEnvelope<Role>>('/roles', query, signal),
    placeholderData: keepPreviousData,
  })
}

export function useRoleMatrix(id: string) {
  return useQuery({
    queryKey: queryKeys.roles.matrix(id),
    queryFn: async () => (await api.get<Envelope<PermissionMatrix>>(`/roles/${id}/permissions`)).data,
  })
}

function useRoleMutation<TVars, TResult>(fn: (vars: TVars) => Promise<TResult>) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.roles.all })
      qc.invalidateQueries({ queryKey: queryKeys.users.all })
    },
  })
}

export const useCreateRole = () => useRoleMutation((input: { name: string }) => api.post<Envelope<Role>>('/roles', input))

export const useUpdateRole = () =>
  useRoleMutation(({ id, ...input }: { id: string; name: string; is_active: boolean }) => api.patch<Envelope<Role>>(`/roles/${id}`, input))

export const useDeleteRole = () => useRoleMutation((id: string) => api.delete(`/roles/${id}`))

export function useSaveMatrix(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (permissions: { menu_id: string; actions: Action[] }[]) =>
      (await api.put<Envelope<PermissionMatrix>>(`/roles/${id}/permissions`, { permissions })).data,
    onSuccess: (matrix) => {
      qc.setQueryData(queryKeys.roles.matrix(id), matrix)
      // The current user's own permissions/navigation may have changed.
      qc.invalidateQueries({ queryKey: queryKeys.me })
      qc.invalidateQueries({ queryKey: ['navigation'] })
    },
  })
}
