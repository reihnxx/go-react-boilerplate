import { EllipsisVertical, KeyRound, Pencil, Plus, Trash2, UserCheck, Users } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTable, type Column } from '@/components/data-table'
import { PageHeader } from '@/components/page-header'
import { SearchInput } from '@/components/search-input'
import { EmptyState } from '@/components/states'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useCurrentUser, usePermission, useStartImpersonation } from '@/features/auth/hooks'
import { useListParams } from '@/hooks/use-list-params'
import { toastError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'
import type { User } from '@/types/api'

import { useDeleteUser, useRoleOptions, useSetUserStatus, useUsers } from './api'
import { UserFormDialog } from './components/UserFormDialog'
import { UserPasswordDialog } from './components/UserPasswordDialog'
import { UserRolesDialog } from './components/UserRolesDialog'

const FILTERS = ['is_active', 'role_id']
const ALL = '__all__'

const isSuperAdmin = (u: User) => u.roles.some((r) => r.is_super_admin)

export function UsersPage() {
  const me = useCurrentUser()
  const can = usePermission()
  const navigate = useNavigate()
  const { params, query, setParam, toggleSort } = useListParams({ sort: 'created_at', order: 'desc', filterKeys: FILTERS })
  const users = useUsers(query)
  const roles = useRoleOptions()

  const [formUser, setFormUser] = useState<User | undefined>()
  const [formOpen, setFormOpen] = useState(false)
  const [rolesUser, setRolesUser] = useState<User | null>(null)
  const [passwordUser, setPasswordUser] = useState<User | null>(null)
  const [deleteUser, setDeleteUser] = useState<User | null>(null)
  const [impersonateUser, setImpersonateUser] = useState<User | null>(null)

  const setStatus = useSetUserStatus()
  const remove = useDeleteUser()
  const impersonate = useStartImpersonation()

  const canUpdate = can('users', 'update')
  const canDelete = can('users', 'delete')
  // Only a super-admin may modify accounts that hold the super-admin role (enforced by the API too).
  const canManage = (u: User) => me.is_super_admin || !isSuperAdmin(u)

  const columns: Column<User>[] = [
    {
      key: 'name',
      header: 'Pengguna',
      sortKey: 'name',
      cell: (u) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{u.name}</div>
          <div className="truncate text-xs text-muted-foreground">
            @{u.username} · {u.email}
          </div>
        </div>
      ),
    },
    {
      key: 'roles',
      header: 'Peran',
      className: 'hidden sm:table-cell',
      cell: (u) =>
        u.roles.length === 0 ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          <div className="flex flex-wrap gap-1">
            {u.roles.map((r) => (
              <Badge key={r.id} variant={r.is_super_admin ? 'default' : 'secondary'}>
                {r.name}
              </Badge>
            ))}
          </div>
        ),
    },
    {
      key: 'is_active',
      header: 'Status',
      sortKey: 'is_active',
      cell: (u) => (
        <div className="flex items-center gap-2">
          <Switch
            checked={u.is_active}
            disabled={!canUpdate || !canManage(u) || u.id === me.user.id || setStatus.isPending}
            aria-label={u.is_active ? `Nonaktifkan ${u.name}` : `Aktifkan ${u.name}`}
            onCheckedChange={(checked) =>
              setStatus.mutate(
                { id: u.id, is_active: checked },
                {
                  onSuccess: () => toast.success(checked ? 'Pengguna diaktifkan' : 'Pengguna dinonaktifkan'),
                  onError: (err) => toastError(err),
                },
              )
            }
          />
          <span className="text-sm text-muted-foreground">{u.is_active ? 'Aktif' : 'Nonaktif'}</span>
        </div>
      ),
    },
    {
      key: 'last_login_at',
      header: 'Login terakhir',
      sortKey: 'last_login_at',
      className: 'hidden lg:table-cell',
      cell: (u) => <span className="text-sm text-muted-foreground">{formatDateTime(u.last_login_at)}</span>,
    },
    {
      key: 'actions',
      header: <span className="sr-only">Aksi</span>,
      className: 'w-12 text-right',
      cell: (u) => {
        const manage = canManage(u)
        const self = u.id === me.user.id
        const canImpersonate = me.is_super_admin && !self && !isSuperAdmin(u) && u.is_active && !me.impersonator
        if (!(canUpdate && manage) && !(canDelete && manage && !self) && !canImpersonate) return null
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={`Aksi untuk ${u.name}`}>
                <EllipsisVertical />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              {canUpdate && manage && (
                <>
                  <DropdownMenuItem
                    onSelect={() => {
                      setFormUser(u)
                      setFormOpen(true)
                    }}
                  >
                    <Pencil /> Ubah data
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => setRolesUser(u)}>
                    <Users /> Atur peran
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => setPasswordUser(u)}>
                    <KeyRound /> Atur kata sandi
                  </DropdownMenuItem>
                </>
              )}
              {canImpersonate && (
                <DropdownMenuItem onSelect={() => setImpersonateUser(u)}>
                  <UserCheck /> Masuk sebagai pengguna ini
                </DropdownMenuItem>
              )}
              {canDelete && manage && !self && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" onSelect={() => setDeleteUser(u)}>
                    <Trash2 /> Hapus
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  return (
    <>
      <PageHeader
        title="Pengguna"
        description="Kelola akun, status dan peran pengguna."
        actions={
          can('users', 'create') && (
            <Button
              onClick={() => {
                setFormUser(undefined)
                setFormOpen(true)
              }}
            >
              <Plus /> Tambah pengguna
            </Button>
          )
        }
      />

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <SearchInput value={params.q} onChange={(v) => setParam('q', v)} placeholder="Cari nama, username, email…" />
        <Select value={params.filters.is_active ?? ALL} onValueChange={(v) => setParam('is_active', v === ALL ? null : v)}>
          <SelectTrigger className="w-full sm:w-40" aria-label="Filter status">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Semua status</SelectItem>
            <SelectItem value="true">Aktif</SelectItem>
            <SelectItem value="false">Nonaktif</SelectItem>
          </SelectContent>
        </Select>
        <Select value={params.filters.role_id ?? ALL} onValueChange={(v) => setParam('role_id', v === ALL ? null : v)}>
          <SelectTrigger className="w-full sm:w-48" aria-label="Filter peran">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Semua peran</SelectItem>
            {roles.data?.map((r) => (
              <SelectItem key={r.id} value={r.id}>
                {r.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <DataTable
        columns={columns}
        rows={users.data?.data}
        rowKey={(u) => u.id}
        isLoading={users.isPending}
        error={users.error}
        onRetry={() => users.refetch()}
        sort={{ key: params.sort, order: params.order }}
        onSort={toggleSort}
        meta={users.data?.meta}
        onPageChange={(p) => setParam('page', p)}
        empty={
          <EmptyState
            title={params.q || Object.keys(params.filters).length ? 'Tidak ada hasil' : 'Belum ada pengguna'}
            description={params.q || Object.keys(params.filters).length ? 'Coba ubah kata kunci atau filter.' : undefined}
          />
        }
      />

      <UserFormDialog open={formOpen} onOpenChange={setFormOpen} user={formUser} />
      <UserRolesDialog user={rolesUser} onOpenChange={(o) => !o && setRolesUser(null)} />
      <UserPasswordDialog user={passwordUser} onOpenChange={(o) => !o && setPasswordUser(null)} />

      <ConfirmDialog
        open={!!deleteUser}
        onOpenChange={(o) => !o && setDeleteUser(null)}
        title="Hapus pengguna?"
        description={
          <>
            Akun <strong>{deleteUser?.name}</strong> akan dihapus dan semua sesinya diakhiri.
          </>
        }
        confirmLabel="Hapus"
        pending={remove.isPending}
        onConfirm={() =>
          deleteUser &&
          remove.mutate(deleteUser.id, {
            onSuccess: () => {
              toast.success('Pengguna dihapus')
              setDeleteUser(null)
            },
            onError: (err) => toastError(err),
          })
        }
      />

      <ConfirmDialog
        open={!!impersonateUser}
        onOpenChange={(o) => !o && setImpersonateUser(null)}
        title="Masuk sebagai pengguna ini?"
        description={
          <>
            Anda akan melihat aplikasi sebagai <strong>{impersonateUser?.name}</strong>. Semua tindakan dicatat atas nama Anda. Gunakan
            tombol &quot;Kembali ke akun saya&quot; untuk keluar.
          </>
        }
        confirmLabel="Masuk"
        destructive={false}
        pending={impersonate.isPending}
        onConfirm={() =>
          impersonateUser &&
          impersonate.mutate(impersonateUser.id, {
            onSuccess: () => navigate('/', { replace: true }),
            onError: (err) => toastError(err),
          })
        }
      />
    </>
  )
}
