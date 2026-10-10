// Route guards. These only shape the UX; every API call is authorized by the
// backend regardless of what the UI shows.
import type { ReactNode } from 'react'
import { Navigate, Outlet, useLocation, useSearchParams } from 'react-router'

import { ErrorState, FullPageLoader } from '@/components/states'
import { ForbiddenPage } from '@/pages/ForbiddenPage'
import type { Action } from '@/types/api'

import { useMe, usePermission, wasSignedOutByUser } from './hooks'

/** Accepts only same-app relative paths to avoid open redirects. */
export function safeNext(next: string | null): string | null {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return null
  return next
}

export function RequireAuth() {
  const { data: me, isPending, error, refetch } = useMe()
  const location = useLocation()

  if (isPending) return <FullPageLoader />
  if (error) return <ErrorState className="min-h-svh" error={error} onRetry={() => refetch()} />
  if (!me) {
    const next = location.pathname + location.search
    const to = next === '/' || wasSignedOutByUser() ? '/login' : `/login?next=${encodeURIComponent(next)}`
    return <Navigate to={to} replace />
  }
  return <Outlet />
}

export function RequireActiveRole() {
  const { data: me } = useMe()
  const location = useLocation()
  if (me?.requires_role_selection) {
    const next = location.pathname + location.search
    return <Navigate to={`/choose-role?next=${encodeURIComponent(next)}`} replace />
  }
  return <Outlet />
}

/** Redirects signed-in users away from login/register pages. */
export function GuestOnly() {
  const { data: me, isPending } = useMe()
  const [params] = useSearchParams()
  if (isPending) return <FullPageLoader />
  if (me) return <Navigate to={safeNext(params.get('next')) ?? '/'} replace />
  return <Outlet />
}

export function RequirePermission({ menu, action = 'view', children }: { menu: string; action?: Action; children: ReactNode }) {
  const can = usePermission()
  return can(menu, action) ? children : <ForbiddenPage />
}
