// End-to-end-ish tests of the real route tree with a mocked API.
import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { apiError, defaultNav, makeMe, mockApi, ok, renderApp } from '@/test/utils'

const options = () => ok({ app_name: 'Test', registration_enabled: false })

describe('authentication guard', () => {
  it('redirects anonymous users to login and keeps the target as next', async () => {
    mockApi({ 'GET /auth/me': () => apiError(401, 'UNAUTHENTICATED'), 'GET /auth/options': options })
    const { router } = renderApp('/users?page=2')
    expect(await screen.findByRole('button', { name: 'Masuk' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toBe('?next=%2Fusers%3Fpage%3D2')
  })

  it('shows a retryable error when the API is unreachable', async () => {
    mockApi({ 'GET /auth/me': () => apiError(500, 'INTERNAL_ERROR', 'Terjadi kesalahan pada server.') })
    renderApp('/dashboard')
    expect(await screen.findByText('Terjadi kesalahan pada server.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /coba lagi/i })).toBeInTheDocument()
  })

  it('sends signed-in users away from the login page', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe()), 'GET /navigation': () => ok(defaultNav) })
    const { router } = renderApp('/login?next=%2Fprofile')
    await waitFor(() => expect(router.state.location.pathname).toBe('/profile'))
  })
})

describe('login', () => {
  it('validates required fields before calling the API', async () => {
    const { calls } = mockApi({ 'GET /auth/me': () => apiError(401, 'UNAUTHENTICATED'), 'GET /auth/options': options })
    const { user } = renderApp('/login')
    await user.click(await screen.findByRole('button', { name: 'Masuk' }))
    expect(await screen.findByText('Username atau email wajib diisi.')).toBeInTheDocument()
    expect(screen.getByText('Kata sandi wajib diisi.')).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/auth/login')).toBe(false)
  })

  it('shows the server message for wrong credentials', async () => {
    mockApi({
      'GET /auth/me': () => apiError(401, 'UNAUTHENTICATED'),
      'GET /auth/options': options,
      'POST /auth/login': () => apiError(401, 'INVALID_CREDENTIALS', 'Username/email atau kata sandi salah.'),
    })
    const { user, router } = renderApp('/login')
    await user.type(await screen.findByLabelText('Username atau email'), 'budi')
    await user.type(screen.getByLabelText('Kata sandi'), 'salah123')
    await user.click(screen.getByRole('button', { name: 'Masuk' }))
    expect(await screen.findByText('Username/email atau kata sandi salah.')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
  })

  it('signs in and continues to the requested page', async () => {
    let loggedIn = false
    const { calls } = mockApi({
      'GET /auth/me': () => (loggedIn ? ok(makeMe()) : apiError(401, 'UNAUTHENTICATED')),
      'GET /auth/options': options,
      'POST /auth/login': () => {
        loggedIn = true
        return ok(makeMe())
      },
      'GET /navigation': () => ok(defaultNav),
      'GET /profile': () =>
        ok({
          id: 'u1',
          name: 'Budi Santoso',
          username: 'budi',
          email: 'budi@example.com',
          phone: null,
          birth_place: null,
          birth_date: null,
          address: null,
        }),
    })
    const { user, router } = renderApp('/login?next=%2Fprofile')
    await user.type(await screen.findByLabelText('Username atau email'), 'budi')
    await user.type(screen.getByLabelText('Kata sandi'), 'Rahasia123')
    await user.click(screen.getByRole('button', { name: 'Masuk' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/profile'))
    expect(await screen.findByRole('heading', { name: 'Profil' })).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/auth/login')?.body).toEqual({ login: 'budi', password: 'Rahasia123' })
  })

  it('ignores an external next URL', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe()), 'GET /navigation': () => ok(defaultNav) })
    const { router } = renderApp('/login?next=https%3A%2F%2Fevil.example')
    await waitFor(() => expect(router.state.location.pathname).toBe('/dashboard'))
  })
})

describe('authorization in the UI', () => {
  it('shows access denied for pages the active role cannot view', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe()), 'GET /navigation': () => ok(defaultNav) })
    renderApp('/users')
    expect(await screen.findByText('Akses ditolak')).toBeInTheDocument()
  })

  it('renders the sidebar from the navigation API', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe()), 'GET /navigation': () => ok(defaultNav) })
    renderApp('/dashboard')
    const nav = await screen.findByRole('link', { name: 'Profil' })
    expect(nav).toHaveAttribute('href', '/profile')
    await waitFor(() =>
      expect(screen.getAllByRole('link', { name: 'Beranda' }).some((l) => l.getAttribute('data-active') === 'true')).toBe(true),
    )
    expect(screen.queryByRole('link', { name: 'Pengguna' })).not.toBeInTheDocument()
  })

  it('asks multi-role users to choose a role first', async () => {
    const roles = [
      { id: 'r1', name: 'Operator', slug: 'operator', is_super_admin: false },
      { id: 'r2', name: 'User', slug: 'user', is_super_admin: false },
    ]
    let active: (typeof roles)[number] | null = null
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe({ roles, active_role: active, requires_role_selection: !active })),
      'PUT /auth/active-role': ({ body }) => {
        active = roles.find((r) => r.id === (body as { role_id: string }).role_id) ?? null
        return ok(makeMe({ roles, active_role: active, requires_role_selection: false }))
      },
      'GET /navigation': () => ok(defaultNav),
    })
    const { user, router } = renderApp('/dashboard')
    expect(await screen.findByText('Pilih otoritas')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/choose-role')
    await user.click(screen.getByRole('button', { name: /Operator/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/dashboard'))
    expect(calls.find((c) => c.path === '/auth/active-role')?.headers.get('X-CSRF-Token')).toBe('csrf-test-token')
  })

  it('returns to login when the session expires during use', async () => {
    let expired = false
    mockApi({
      'GET /auth/me': () => (expired ? apiError(401, 'UNAUTHENTICATED') : ok(makeMe())),
      'GET /navigation': () => ok(defaultNav),
      'GET /profile': () => {
        expired = true
        return apiError(401, 'UNAUTHENTICATED')
      },
      'GET /auth/options': options,
    })
    const { router } = renderApp('/profile')
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })

  it('shows the impersonation banner', async () => {
    mockApi({
      'GET /auth/me': () => ok(makeMe({ impersonator: { id: 'a', name: 'Admin Utama', username: 'admin', email: 'a@x' } })),
      'GET /navigation': () => ok(defaultNav),
    })
    renderApp('/dashboard')
    const banner = (await screen.findByText(/Anda sedang masuk sebagai/)).parentElement!
    expect(within(banner).getByText('Admin Utama', { exact: false })).toBeInTheDocument()
    expect(within(banner).getByRole('button', { name: 'Kembali ke akun saya' })).toBeInTheDocument()
  })

  it('shows a not-found page for unknown routes', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe()), 'GET /navigation': () => ok(defaultNav) })
    renderApp('/tidak-ada')
    expect(await screen.findByText('Halaman tidak ditemukan')).toBeInTheDocument()
  })
})
