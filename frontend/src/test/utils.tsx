import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { vi } from 'vitest'

import { Providers } from '@/app/providers'
import { createQueryClient, wireApiHandlers } from '@/app/query-client'
import { routes } from '@/app/routes'
import { setCsrfToken } from '@/lib/api'
import type { Action, Me, NavGroup } from '@/types/api'

type Handler = (req: { url: URL; init: RequestInit; body: unknown }) => Response | Promise<Response>

export function json(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

export const ok = (data: unknown, meta?: unknown) => json(200, meta ? { data, meta } : { data })
export const apiError = (status: number, code: string, message = 'error', fields?: Record<string, string>) =>
  json(status, { error: { code, message, fields } })

/**
 * Stubs global fetch. Keys are "METHOD /path" relative to /api/v1 (query
 * string ignored). Unmatched requests fail the test loudly with a 599.
 */
export function mockApi(handlers: Record<string, Handler>) {
  const calls: { method: string; path: string; body: unknown; headers: Headers }[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = new URL(String(input), 'http://localhost')
    const method = (init.method ?? 'GET').toUpperCase()
    const path = url.pathname.replace(/^\/api\/v1/, '')
    const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined
    calls.push({ method, path, body, headers: new Headers(init.headers) })
    const handler = handlers[`${method} ${path}`]
    if (!handler) return json(599, { error: { code: 'UNMOCKED', message: `${method} ${path} not mocked` } })
    return handler({ url, init, body })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { fetchMock, calls }
}

export function makeMe(overrides: Partial<Me> & { permissions?: Record<string, Action[]> } = {}): Me {
  const role = { id: 'role-user', name: 'User', slug: 'user', is_super_admin: false }
  return {
    user: { id: 'u1', name: 'Budi Santoso', username: 'budi', email: 'budi@example.com' },
    roles: [role],
    active_role: role,
    requires_role_selection: false,
    is_super_admin: false,
    permissions: { dashboard: ['view'], profile: ['view'] },
    csrf_token: 'csrf-test-token',
    impersonator: null,
    ...overrides,
  }
}

export const defaultNav: NavGroup[] = [
  {
    id: 'g1',
    name: 'Beranda',
    items: [
      { id: 'm1', name: 'Beranda', slug: 'dashboard', path: '/dashboard', icon: 'layout-dashboard', children: [] },
      { id: 'm2', name: 'Profil', slug: 'profile', path: '/profile', icon: 'user', children: [] },
    ],
  },
]

/** Renders the whole app (real routes, guards and providers) at path. */
export function renderApp(path: string) {
  setCsrfToken(null)
  const qc = createQueryClient()
  qc.setDefaultOptions({ queries: { retry: false, staleTime: 0 }, mutations: { retry: false } })
  wireApiHandlers(qc)
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const user = userEvent.setup()
  const utils = render(
    <Providers client={qc}>
      <RouterProvider router={router} />
    </Providers>,
  )
  return { ...utils, router, qc, user }
}
