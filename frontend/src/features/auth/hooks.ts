import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'

import { queryKeys } from '@/lib/query-keys'
import { api } from '@/lib/api'
import type { Action, Envelope, Me, NavGroup, NavItem } from '@/types/api'

import * as authApi from './api'

export function useMe() {
  return useQuery({ queryKey: queryKeys.me, queryFn: authApi.fetchMe, staleTime: 5 * 60_000, retry: false })
}

/** The current user; only valid inside authenticated routes. */
export function useCurrentUser(): Me {
  const { data } = useMe()
  if (!data) throw new Error('useCurrentUser used outside an authenticated route')
  return data
}

/**
 * Replaces the whole client cache with a new identity (after login, role
 * switch or impersonation) so no data from the previous identity leaks.
 */
export function resetSession(qc: QueryClient, me: Me | null) {
  // Everything except ['auth', …] (current user + public auth options).
  const notMe = { predicate: (q: { queryKey: readonly unknown[] }) => q.queryKey[0] !== 'auth' }
  qc.setQueryData(queryKeys.me, me)
  if (me) {
    // Reset (not remove) so mounted observers stay attached and refetch with
    // the new identity; removing would leave them showing stale data.
    void qc.resetQueries(notMe)
  } else {
    // Signed out: guards unmount everything, just drop the cached data.
    void qc.cancelQueries(notMe)
    qc.removeQueries(notMe)
  }
}

/** UI permission check. The backend enforces the same rules independently. */
export function usePermission() {
  const { data: me } = useMe()
  return useCallback(
    (menu: string, action: Action = 'view') => !!me && (me.is_super_admin || !!me.permissions[menu]?.includes(action)),
    [me],
  )
}

export function useNavigation() {
  const { data: me } = useMe()
  const roleId = me?.active_role?.id
  return useQuery({
    queryKey: queryKeys.navigation(roleId),
    queryFn: async () => (await api.get<Envelope<NavGroup[]>>('/navigation')).data,
    enabled: !!roleId,
    staleTime: 5 * 60_000,
  })
}

/** First navigable path in the sidebar (used as the landing page). */
export function firstPath(groups: NavGroup[] | undefined): string | null {
  const walk = (items: NavItem[]): string | null => {
    for (const item of items) {
      if (item.path) return item.path
      const child = walk(item.children)
      if (child) return child
    }
    return null
  }
  for (const g of groups ?? []) {
    const p = walk(g.items)
    if (p) return p
  }
  return null
}

// Set while the user has explicitly signed out, so the auth guard sends them
// to a plain /login instead of /login?next=… (the next person to sign in
// should not land on the previous user's page).
let signedOutByUser = false
export const wasSignedOutByUser = () => signedOutByUser

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: authApi.login,
    onSuccess: (me) => {
      signedOutByUser = false
      resetSession(qc, me)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => {
      signedOutByUser = true
      return authApi.logout()
    },
    // Even if the request fails (e.g. session already gone) the client forgets the identity.
    onSettled: () => resetSession(qc, null),
  })
}

export function useSetActiveRole() {
  const qc = useQueryClient()
  return useMutation({ mutationFn: authApi.setActiveRole, onSuccess: (me) => resetSession(qc, me) })
}

export function useStartImpersonation() {
  const qc = useQueryClient()
  return useMutation({ mutationFn: authApi.startImpersonation, onSuccess: (me) => resetSession(qc, me) })
}

export function useStopImpersonation() {
  const qc = useQueryClient()
  return useMutation({ mutationFn: authApi.stopImpersonation, onSuccess: (me) => resetSession(qc, me) })
}

export function useAuthOptions() {
  return useQuery({ queryKey: queryKeys.authOptions, queryFn: authApi.fetchAuthOptions, staleTime: Infinity })
}
