import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { apiError, defaultNav, makeMe, mockApi, ok, renderApp } from '@/test/utils'
import type { PermissionMatrix, Role, User } from '@/types/api'

const meta = (total: number) => ({ page: 1, per_page: 20, total, total_pages: 1 })

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
const superRole: Role = { ...userRole, id: 'role-super', name: 'Super Admin', slug: 'super-admin', is_super_admin: true }

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

function usersApi(me = makeMe({ permissions: { users: ['view', 'create', 'update', 'delete'] } }), extra = {}) {
  return mockApi({
    'GET /auth/me': () => ok(me),
    'GET /navigation': () => ok(defaultNav),
    'GET /users': () => ok([budi], meta(1)),
    'GET /roles': () => ok([superRole, userRole], meta(2)),
    ...extra,
  })
}

describe('users page', () => {
  it('lists users and hides controls the role may not use', async () => {
    usersApi(makeMe({ permissions: { users: ['view'] } }))
    renderApp('/users')
    expect(await screen.findByText('Budi')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Tambah pengguna/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Aksi untuk Budi/ })).not.toBeInTheDocument()
    expect(screen.getByRole('switch', { name: /Nonaktifkan Budi/ })).toBeDisabled()
  })

  it('sends search and sort to the API via the URL', async () => {
    const { fetchMock } = usersApi()
    const { user, router } = renderApp('/users')
    await screen.findByText('Budi')
    await user.type(screen.getByRole('searchbox'), 'bud')
    await waitFor(() => expect(router.state.location.search).toContain('q=bud'))
    await user.click(screen.getByRole('button', { name: /^Pengguna/ }))
    await waitFor(() => expect(router.state.location.search).toContain('sort=name'))
    await waitFor(() => {
      const url = fetchMock.mock.calls
        .map((c) => String(c[0]))
        .filter((u) => u.includes('/users?'))
        .at(-1)
      expect(url).toContain('q=bud')
      expect(url).toContain('sort=name')
    })
  })

  it('validates the create form and maps server field errors', async () => {
    const { calls } = usersApi(undefined, {
      'POST /users': () => apiError(422, 'VALIDATION_ERROR', 'Data yang dikirim tidak valid.', { email: 'Email sudah digunakan.' }),
    })
    const { user } = renderApp('/users')
    await user.click(await screen.findByRole('button', { name: /Tambah pengguna/ }))
    const dialog = await screen.findByRole('dialog')

    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Nama wajib diisi.')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)

    await user.type(within(dialog).getByLabelText(/^Nama/), 'Siti')
    await user.type(within(dialog).getByLabelText(/^Username/), 'siti')
    await user.type(within(dialog).getByLabelText(/^Email/), 'siti@example.com')
    await user.type(within(dialog).getByLabelText(/^Kata sandi/), 'Rahasia123')
    await user.type(within(dialog).getByLabelText(/^Konfirmasi/), 'Beda12345')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Konfirmasi kata sandi tidak cocok.')).toBeInTheDocument()

    await user.clear(within(dialog).getByLabelText(/^Konfirmasi/))
    await user.type(within(dialog).getByLabelText(/^Konfirmasi/), 'Rahasia123')
    await user.click(within(dialog).getByRole('checkbox', { name: 'User' }))
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Email sudah digunakan.')).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({
      name: 'Siti',
      username: 'siti',
      email: 'siti@example.com',
      role_ids: ['role-user'],
      is_active: true,
    })
  })

  it('does not let non-super-admins grant the super-admin role', async () => {
    usersApi()
    const { user } = renderApp('/users')
    await user.click(await screen.findByRole('button', { name: /Tambah pengguna/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('checkbox', { name: /Super Admin/ })).toBeDisabled()
  })

  it('asks for confirmation before deleting', async () => {
    const { calls } = usersApi(undefined, { 'DELETE /users/u2': () => new Response(null, { status: 204 }) })
    const { user } = renderApp('/users')
    await user.click(await screen.findByRole('button', { name: /Aksi untuk Budi/ }))
    await user.click(await screen.findByRole('menuitem', { name: /Hapus/ }))
    const confirm = await screen.findByRole('alertdialog')
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    await user.click(within(confirm).getByRole('button', { name: 'Hapus' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.path === '/users/u2')).toBe(true))
  })
})

describe('permission matrix', () => {
  const matrix = (overrides: Partial<PermissionMatrix> = {}): PermissionMatrix => ({
    role: userRole,
    read_only: false,
    actions: [
      { code: 'view', label: 'Lihat' },
      { code: 'create', label: 'Tambah' },
      { code: 'update', label: 'Ubah' },
      { code: 'delete', label: 'Hapus' },
      { code: 'download', label: 'Unduh' },
    ],
    menus: [
      { id: 'm-dash', name: 'Beranda', slug: 'dashboard', is_active: true, granted: ['view'], children: [] },
      {
        id: 'm-rep',
        name: 'Laporan',
        slug: 'reports',
        is_active: true,
        granted: [],
        children: [{ id: 'm-rep-a', name: 'Rekap', slug: 'report_a', is_active: true, granted: [], children: [] }],
      },
    ],
    ...overrides,
  })

  it('saves only changed grants, parents offer only "view"', async () => {
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe({ is_super_admin: true })),
      'GET /navigation': () => ok(defaultNav),
      'GET /roles/role-user/permissions': () => ok(matrix()),
      'PUT /roles/role-user/permissions': () => ok(matrix()),
    })
    const { user } = renderApp('/roles/role-user/permissions')
    const save = await screen.findByRole('button', { name: /Simpan/ })
    expect(save).toBeDisabled()
    expect(screen.queryByRole('checkbox', { name: 'Tambah Laporan' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: 'Lihat Rekap' }))
    expect(save).toBeEnabled()
    await user.click(screen.getByRole('checkbox', { name: 'Lihat Rekap' }))
    expect(save).toBeDisabled() // back to the original state

    await user.click(screen.getByRole('checkbox', { name: 'Lihat Rekap' }))
    await user.click(save)
    await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
      permissions: [
        { menu_id: 'm-dash', actions: ['view'] },
        { menu_id: 'm-rep-a', actions: ['view'] },
      ],
    })
  })

  it('is read-only for the super-admin role', async () => {
    mockApi({
      'GET /auth/me': () => ok(makeMe({ is_super_admin: true })),
      'GET /navigation': () => ok(defaultNav),
      'GET /roles/role-super/permissions': () => ok(matrix({ role: superRole, read_only: true })),
    })
    renderApp('/roles/role-super/permissions')
    expect(await screen.findByText(/selalu memiliki semua hak akses/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Simpan/ })).not.toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Lihat Beranda' })).toBeDisabled()
  })

  it('disables grants the current user does not hold', async () => {
    mockApi({
      'GET /auth/me': () => ok(makeMe({ permissions: { roles: ['view', 'update'], dashboard: ['view'] } })),
      'GET /navigation': () => ok(defaultNav),
      'GET /roles/role-user/permissions': () => ok(matrix()),
    })
    renderApp('/roles/role-user/permissions')
    expect(await screen.findByRole('checkbox', { name: 'Lihat Beranda' })).toBeEnabled()
    expect(screen.getByRole('checkbox', { name: 'Hapus Beranda' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Lihat Rekap' })).toBeDisabled()
  })
})

describe('menu form', () => {
  it('applies main/child rules before submitting', async () => {
    const { calls } = mockApi({
      'GET /auth/me': () => ok(makeMe({ is_super_admin: true })),
      'GET /navigation': () => ok(defaultNav),
      'GET /menus': () => ok([], meta(0)),
      'GET /menu-groups': () => ok([{ id: 'g1', name: 'Beranda', sort_order: 10 }]),
    })
    const { user } = renderApp('/menus')
    await user.click(await screen.findByRole('button', { name: /Tambah menu/ }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(/^Nama menu/), 'Laporan')
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Grup menu wajib diisi untuk menu utama.')).toBeInTheDocument()
    expect(within(dialog).getByText('Ikon wajib diisi untuk menu utama.')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('radio', { name: 'Submenu' }))
    await user.click(within(dialog).getByRole('button', { name: 'Simpan' }))
    expect(await within(dialog).findByText('Menu induk wajib diisi untuk submenu.')).toBeInTheDocument()
    expect(within(dialog).getByText('Path wajib diisi untuk submenu.')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })
})
