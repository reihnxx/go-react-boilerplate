// Flows that were previously only verified by hand.
import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { apiError, defaultNav, json, makeMe, mockApi, ok, renderApp } from '@/test/utils'
import type { Profile, Role, User } from '@/types/api'

const meta = (total: number) => ({ page: 1, per_page: 20, total, total_pages: 1 })
const options = () => ok({ app_name: 'Test', registration_enabled: true })
const anon = () => apiError(401, 'UNAUTHENTICATED')

const userRole: Role = {
  id: 'role-user',
  name: 'User',
  slug: 'user',
  is_super_admin: false,
  is_system: true,
  is_active: true,
  user_count: 1,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}
const opRole: Role = { ...userRole, id: 'role-op', name: 'Operator', slug: 'operator', is_system: false }
const budi: User = {
  id: 'u2',
  name: 'Budi',
  username: 'budi',
  email: 'budi@example.com',
  is_active: true,
  last_login_at: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  roles: [{ id: 'role-user', name: 'User', slug: 'user', is_super_admin: false }],
}
const admin = () => makeMe({ is_super_admin: true, permissions: {} })

function usersPage(extra: Parameters<typeof mockApi>[0] = {}) {
  return mockApi({
    'GET /auth/me': () => ok(admin()),
    'GET /navigation': () => ok(defaultNav),
    'GET /users': () => ok([budi], meta(1)),
    'GET /roles': () => ok([userRole, opRole], meta(2)),
    ...extra,
  })
}

async function openRowAction(user: ReturnType<typeof renderApp>['user'], item: RegExp) {
  await user.click(await screen.findByRole('button', { name: /Aksi untuk Budi/ }))
  await user.click(await screen.findByRole('menuitem', { name: item }))
  return screen.findByRole('dialog')
}

describe('user administration dialogs', () => {
  it('edits a user', async () => {
    const { calls } = usersPage({ 'PATCH /users/u2': ({ body }) => ok({ ...budi, ...(body as object) }) })
    const { user } = renderApp('/users')
    const dialog = await openRowAction(user, /Ubah data/)
    const name = within(dialog).getByLabelText(/^Nama/)
    expect(name).toHaveValue('Budi')
    await user.clear(name)
    await user.type(name, 'Budi Baru')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ name: 'Budi Baru', username: 'budi', email: 'budi@example.com' }),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('replaces a user’s roles', async () => {
    const { calls } = usersPage({ 'PUT /users/u2/roles': () => ok(budi) })
    const { user } = renderApp('/users')
    const dialog = await openRowAction(user, /Atur peran/)
    expect(within(dialog).getByRole('checkbox', { name: 'User' })).toBeChecked()
    await user.click(within(dialog).getByRole('checkbox', { name: 'Operator' }))
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ role_ids: ['role-user', 'role-op'] }))
  })

  it('shows API refusals (e.g. last super-admin) as an error', async () => {
    usersPage({
      'PUT /users/u2/roles': () => apiError(409, 'CONFLICT', 'Tidak dapat mencabut peran Super Admin dari Super Admin aktif terakhir.'),
    })
    const { user } = renderApp('/users')
    const dialog = await openRowAction(user, /Atur peran/)
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await screen.findByText(/Super Admin aktif terakhir/)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument() // stays open
  })

  it('sets a password with client-side validation', async () => {
    const { calls } = usersPage({ 'PUT /users/u2/password': () => new Response(null, { status: 204 }) })
    const { user } = renderApp('/users')
    const dialog = await openRowAction(user, /Atur kata sandi/)
    await user.type(within(dialog).getByLabelText(/^Kata sandi baru/), 'pendek')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Kata sandi minimal 8 karakter.')).toBeInTheDocument()
    await user.clear(within(dialog).getByLabelText(/^Kata sandi baru/))
    await user.type(within(dialog).getByLabelText(/^Kata sandi baru/), 'Rahasia123')
    await user.type(within(dialog).getByLabelText(/^Konfirmasi/), 'Rahasia123')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ password: 'Rahasia123', password_confirmation: 'Rahasia123' }),
    )
  })

  it('toggles the active status', async () => {
    const { calls } = usersPage({ 'PATCH /users/u2/status': () => ok({ ...budi, is_active: false }) })
    const { user } = renderApp('/users')
    await user.click(await screen.findByRole('switch', { name: /Nonaktifkan Budi/ }))
    await waitFor(() => expect(calls.find((c) => c.path === '/users/u2/status')?.body).toEqual({ is_active: false }))
  })

  it('does not offer self-destructive actions on the own account', async () => {
    const self = { ...budi, id: 'u1', name: 'Budi Santoso' }
    mockApi({
      'GET /auth/me': () => ok(admin()),
      'GET /navigation': () => ok(defaultNav),
      'GET /users': () => ok([self], meta(1)),
      'GET /roles': () => ok([userRole], meta(1)),
    })
    const { user } = renderApp('/users')
    expect(await screen.findByRole('switch', { name: /Budi Santoso/ })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: /Aksi untuk Budi Santoso/ }))
    expect(await screen.findByRole('menuitem', { name: /Ubah data/ })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: /Hapus/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: /Masuk sebagai/ })).not.toBeInTheDocument()
  })
})

describe('roles page', () => {
  it('creates a role and hides delete for system roles', async () => {
    const { calls } = mockApi({
      'GET /auth/me': () => ok(admin()),
      'GET /navigation': () => ok(defaultNav),
      'GET /roles': () => ok([userRole, opRole], meta(2)),
      'POST /roles': () => json(201, { data: { ...opRole, id: 'new', name: 'Guru' } }),
    })
    const { user } = renderApp('/roles')
    await screen.findByText('Operator')
    const rows = screen.getAllByRole('row')
    const userRow = rows.find((r) => within(r).queryByText('User'))!
    const opRow = rows.find((r) => within(r).queryByText('Operator'))!
    expect(within(userRow).queryByRole('button', { name: 'Hapus' })).not.toBeInTheDocument()
    expect(within(opRow).getByRole('button', { name: 'Hapus' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Tambah peran/ }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(/Nama peran/), 'Guru')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ name: 'Guru' }))
  })
})

describe('session and account pages', () => {
  it('switches the active role from the user menu', async () => {
    const roles = [
      { id: 'r1', name: 'User', slug: 'user', is_super_admin: false },
      { id: 'r2', name: 'Operator', slug: 'operator', is_super_admin: false },
    ]
    let active = roles[0]
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe({ roles, active_role: active })),
      'PUT /auth/active-role': ({ body }) => {
        active = roles.find((r) => r.id === (body as { role_id: string }).role_id)!
        return ok(makeMe({ roles, active_role: active }))
      },
      'GET /navigation': () => ok(defaultNav),
    })
    const { user, router } = renderApp('/dashboard')
    await user.click(await screen.findByRole('button', { name: 'Menu pengguna' }))
    await user.click(await screen.findByRole('menuitem', { name: /Ganti otoritas/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/choose-role'))
    await user.click(await screen.findByRole('button', { name: /Operator/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/dashboard'))
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ role_id: 'r2' })
  })

  it('logs out to a plain login page', async () => {
    let loggedOut = false
    mockApi({
      'GET /auth/me': () => (loggedOut ? anon() : ok(makeMe())),
      'GET /navigation': () => ok(defaultNav),
      'POST /auth/logout': () => {
        loggedOut = true
        return new Response(null, { status: 204 })
      },
      'GET /auth/options': options,
    })
    const { user, router } = renderApp('/profile')
    await user.click(await screen.findByRole('button', { name: 'Menu pengguna' }))
    await user.click(await screen.findByRole('menuitem', { name: /Keluar/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toBe('')
  })

  it('updates the profile and maps server errors', async () => {
    const profile: Profile = {
      id: 'u1',
      name: 'Budi Santoso',
      username: 'budi',
      email: 'budi@example.com',
      phone: null,
      birth_place: null,
      birth_date: null,
      address: null,
    }
    let attempt = 0
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe()),
      'GET /navigation': () => ok(defaultNav),
      'GET /profile': () => ok(profile),
      'PUT /profile': ({ body }) =>
        ++attempt === 1
          ? apiError(422, 'VALIDATION_ERROR', 'x', { email: 'Email sudah digunakan.' })
          : ok({ ...profile, ...(body as object) }),
    })
    const { user } = renderApp('/profile')
    const phone = await screen.findByLabelText('No HP')
    await user.type(phone, '123')
    await user.click(screen.getByRole('button', { name: 'Simpan profil' }))
    expect(await screen.findByText(/Format No HP tidak valid/)).toBeInTheDocument()

    await user.clear(phone)
    await user.type(phone, '081234567890')
    await user.click(screen.getByRole('button', { name: 'Simpan profil' }))
    expect(await screen.findByText('Email sudah digunakan.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Simpan profil' }))
    await waitFor(() => expect(attempt).toBe(2))
    expect(calls.filter((c) => c.method === 'PUT').at(-1)?.body).toMatchObject({ phone: '081234567890', name: 'Budi Santoso' })
  })

  it('changes the own password', async () => {
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe()),
      'GET /navigation': () => ok(defaultNav),
      'GET /profile': () =>
        ok({ id: 'u1', name: 'Budi', username: 'budi', email: 'b@x.id', phone: null, birth_place: null, birth_date: null, address: null }),
      'PUT /profile/password': () => apiError(422, 'VALIDATION_ERROR', 'x', { current_password: 'Kata sandi lama salah.' }),
    })
    const { user } = renderApp('/profile')
    await user.type(await screen.findByLabelText('Kata sandi lama'), 'salah123')
    await user.type(screen.getByLabelText('Kata sandi baru'), 'BaruSekali1')
    await user.type(screen.getByLabelText('Konfirmasi'), 'BaruSekali1')
    await user.click(screen.getByRole('button', { name: 'Ubah kata sandi' }))
    expect(await screen.findByText('Kata sandi lama salah.')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/profile/password')?.headers.get('X-CSRF-Token')).toBe('csrf-test-token')
  })
})

describe('public auth pages', () => {
  it('requests a reset link and shows the generic answer', async () => {
    const { calls } = mockApi({
      'GET /auth/me': anon,
      'POST /auth/forgot-password': () => ok({ message: 'Jika email terdaftar, tautan reset password telah dikirim.' }),
    })
    const { user } = renderApp('/forgot-password')
    await user.type(await screen.findByLabelText('Email'), 'budi@example.com')
    await user.click(screen.getByRole('button', { name: /Kirim tautan/ }))
    expect(await screen.findByText(/Jika email terdaftar/)).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ email: 'budi@example.com' })
  })

  it('resets the password with the token from the link', async () => {
    const { calls } = mockApi({
      'GET /auth/me': anon,
      'GET /auth/options': options,
      'POST /auth/reset-password': () => ok({ message: 'Password berhasil direset. Silakan login.' }),
    })
    const { user, router } = renderApp('/reset-password?token=abc123')
    await user.type(await screen.findByLabelText('Kata sandi baru'), 'BaruSekali1')
    await user.type(screen.getByLabelText('Konfirmasi kata sandi'), 'BaruSekali1')
    await user.click(screen.getByRole('button', { name: 'Simpan kata sandi' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({
      token: 'abc123',
      password: 'BaruSekali1',
      password_confirmation: 'BaruSekali1',
    })
  })

  it('reports an expired reset token', async () => {
    mockApi({
      'GET /auth/me': anon,
      'POST /auth/reset-password': () =>
        apiError(422, 'VALIDATION_ERROR', 'x', { token: 'Tautan reset tidak valid atau sudah kedaluwarsa.' }),
    })
    const { user } = renderApp('/reset-password?token=old')
    await user.type(await screen.findByLabelText('Kata sandi baru'), 'BaruSekali1')
    await user.type(screen.getByLabelText('Konfirmasi kata sandi'), 'BaruSekali1')
    await user.click(screen.getByRole('button', { name: 'Simpan kata sandi' }))
    expect(await screen.findByText(/sudah kedaluwarsa/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Minta tautan baru' })).toBeInTheDocument()
  })

  it('registers when enabled and is hidden when disabled', async () => {
    const { calls } = mockApi({
      'GET /auth/me': anon,
      'GET /auth/options': options,
      'POST /auth/register': () => json(201, { data: { message: 'Berhasil mendaftar! Silakan login untuk masuk.' } }),
    })
    const { user, router } = renderApp('/register')
    await user.type(await screen.findByLabelText('Nama lengkap'), 'Siti')
    await user.type(screen.getByLabelText('Username'), 'siti')
    await user.type(screen.getByLabelText('Email'), 'siti@example.com')
    await user.type(screen.getByLabelText('Kata sandi'), 'Rahasia123')
    await user.type(screen.getByLabelText('Konfirmasi kata sandi'), 'Rahasia123')
    await user.click(screen.getByRole('button', { name: 'Daftar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(calls.find((c) => c.path === '/auth/register')?.body).toMatchObject({ username: 'siti' })
  })

  it('shows not-found for /register when registration is disabled', async () => {
    mockApi({ 'GET /auth/me': anon, 'GET /auth/options': () => ok({ app_name: 'T', registration_enabled: false }) })
    renderApp('/register')
    expect(await screen.findByText('Halaman tidak ditemukan')).toBeInTheDocument()
  })
})

describe('landing page', () => {
  it('sends roles without dashboard access to their first menu', async () => {
    const nav = [
      { id: 'g', name: 'Data', items: [{ id: 'm', name: 'Laporan', slug: 'reports', path: '/laporan', icon: 'file-text', children: [] }] },
    ]
    mockApi({ 'GET /auth/me': () => ok(makeMe({ permissions: { reports: ['view'] } })), 'GET /navigation': () => ok(nav) })
    const { router } = renderApp('/')
    await waitFor(() => expect(router.state.location.pathname).toBe('/laporan'))
  })

  it('explains when a role has no menus at all', async () => {
    mockApi({ 'GET /auth/me': () => ok(makeMe({ permissions: {} })), 'GET /navigation': () => ok([]) })
    renderApp('/')
    expect(await screen.findByText('Belum ada menu')).toBeInTheDocument()
  })
})

describe('menu management', () => {
  it('creates a main menu with group and icon', async () => {
    const { calls } = mockApi({
      'GET /auth/me': () => ok(admin()),
      'GET /navigation': () => ok(defaultNav),
      'GET /menus': () => ok([], meta(0)),
      'GET /menu-groups': () =>
        ok([
          { id: 'g1', name: 'Beranda', sort_order: 10 },
          { id: 'g2', name: 'Manajemen', sort_order: 20 },
        ]),
      'POST /menus': ({ body }) => json(201, { data: { id: 'n', ...(body as object) } }),
    })
    const { user } = renderApp('/menus')
    await user.click(await screen.findByRole('button', { name: /Tambah menu/ }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(/^Nama menu/), 'Laporan Bulanan')
    await user.click(within(dialog).getByRole('combobox', { name: /Grup menu/ }))
    await user.click(await screen.findByRole('option', { name: 'Manajemen' }))
    await user.type(within(dialog).getByLabelText(/^Path/), '/laporan')
    await user.type(within(dialog).getByRole('textbox', { name: 'Cari ikon' }), 'file')
    await user.click(within(dialog).getByRole('option', { name: 'file-text' }))
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({
        name: 'Laporan Bulanan',
        type: 'main',
        group_id: 'g2',
        parent_id: null,
        path: '/laporan',
        icon: 'file-text',
        is_active: true,
      }),
    )
  })
})
