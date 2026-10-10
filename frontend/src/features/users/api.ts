import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type QueryValue } from '@/lib/api'
import { queryKeys } from '@/lib/query-keys'
import type { Envelope, ListEnvelope, Role, User } from '@/types/api'

export interface UserInput {
  name: string
  username: string
  email: string
}

export interface CreateUserInput extends UserInput {
  password: string
  password_confirmation: string
  is_active: boolean
  role_ids: string[]
}

export function useUsers(query: Record<string, QueryValue>) {
  return useQuery({
    queryKey: queryKeys.users.list(query),
    queryFn: ({ signal }) => api.get<ListEnvelope<User>>('/users', query, signal),
    placeholderData: keepPreviousData,
  })
}

/** Active roles for selects and checkboxes. */
export function useRoleOptions() {
  return useQuery({
    queryKey: queryKeys.roles.options,
    queryFn: async () => (await api.get<ListEnvelope<Role>>('/roles', { per_page: 100, is_active: true, sort: 'name' })).data,
    staleTime: 60_000,
  })
}

function useUserMutation<TVars>(fn: (vars: TVars) => Promise<unknown>) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.users.all }),
  })
}

export const useCreateUser = () => useUserMutation((input: CreateUserInput) => api.post<Envelope<User>>('/users', input))

export const useUpdateUser = () =>
  useUserMutation(({ id, ...input }: UserInput & { id: string }) => api.patch<Envelope<User>>(`/users/${id}`, input))

export const useSetUserStatus = () =>
  useUserMutation(({ id, is_active }: { id: string; is_active: boolean }) =>
    api.patch<Envelope<User>>(`/users/${id}/status`, { is_active }),
  )

export const useSetUserRoles = () =>
  useUserMutation(({ id, role_ids }: { id: string; role_ids: string[] }) => api.put<Envelope<User>>(`/users/${id}/roles`, { role_ids }))

export const useSetUserPassword = () =>
  useUserMutation(({ id, ...body }: { id: string; password: string; password_confirmation: string }) =>
    api.put(`/users/${id}/password`, body),
  )

export const useDeleteUser = () => useUserMutation((id: string) => api.delete(`/users/${id}`))
