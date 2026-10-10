import { Pencil, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTable, type Column } from '@/components/data-table'
import { PageHeader } from '@/components/page-header'
import { SearchInput } from '@/components/search-input'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { usePermission } from '@/features/auth/hooks'
import { useListParams } from '@/hooks/use-list-params'
import { toastError } from '@/lib/errors'
import type { Role } from '@/types/api'

import { useDeleteRole, useRoles } from './api'
import { RoleFormDialog } from './RoleFormDialog'

export function RolesPage() {
  const can = usePermission()
  const { params, query, setParam, toggleSort } = useListParams({ sort: 'name', order: 'asc' })
  const roles = useRoles(query)
  const remove = useDeleteRole()
  const [formOpen, setFormOpen] = useState(false)
  const [editRole, setEditRole] = useState<Role | undefined>()
  const [deleteRole, setDeleteRole] = useState<Role | null>(null)

  const columns: Column<Role>[] = [
    {
      key: 'name',
      header: 'Peran',
      sortKey: 'name',
      cell: (r) => (
        <div>
          <div className="flex items-center gap-2 font-medium">
            {r.name}
            {r.is_super_admin && <Badge>semua akses</Badge>}
            {r.is_system && !r.is_super_admin && <Badge variant="secondary">sistem</Badge>}
          </div>
          <code className="text-xs text-muted-foreground">{r.slug}</code>
        </div>
      ),
    },
    {
      key: 'user_count',
      header: 'Pengguna',
      sortKey: 'user_count',
      className: 'w-28',
      cell: (r) => <span className="tabular-nums">{r.user_count}</span>,
    },
    {
      key: 'is_active',
      header: 'Status',
      className: 'w-28',
      cell: (r) => <Badge variant={r.is_active ? 'outline' : 'secondary'}>{r.is_active ? 'Aktif' : 'Nonaktif'}</Badge>,
    },
    {
      key: 'actions',
      header: <span className="sr-only">Aksi</span>,
      className: 'w-40 text-right',
      cell: (r) => (
        <div className="flex justify-end gap-1">
          <IconAction label="Hak akses" to={`/roles/${r.id}/permissions`} icon={<ShieldCheck />} />
          {can('roles', 'update') && (
            <IconAction
              label="Ubah"
              icon={<Pencil />}
              onClick={() => {
                setEditRole(r)
                setFormOpen(true)
              }}
            />
          )}
          {can('roles', 'delete') && !r.is_system && (
            <IconAction label="Hapus" icon={<Trash2 />} destructive onClick={() => setDeleteRole(r)} />
          )}
        </div>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Otoritas"
        description="Peran menentukan menu dan aksi yang dapat diakses pengguna."
        actions={
          can('roles', 'create') && (
            <Button
              onClick={() => {
                setEditRole(undefined)
                setFormOpen(true)
              }}
            >
              <Plus /> Tambah peran
            </Button>
          )
        }
      />
      <SearchInput value={params.q} onChange={(v) => setParam('q', v)} placeholder="Cari peran…" />
      <DataTable
        columns={columns}
        rows={roles.data?.data}
        rowKey={(r) => r.id}
        isLoading={roles.isPending}
        error={roles.error}
        onRetry={() => roles.refetch()}
        sort={{ key: params.sort, order: params.order }}
        onSort={toggleSort}
        meta={roles.data?.meta}
        onPageChange={(p) => setParam('page', p)}
      />
      <RoleFormDialog open={formOpen} onOpenChange={setFormOpen} role={editRole} />
      <ConfirmDialog
        open={!!deleteRole}
        onOpenChange={(o) => !o && setDeleteRole(null)}
        title="Hapus peran?"
        description={
          <>
            Peran <strong>{deleteRole?.name}</strong> akan dihapus dan dicabut dari {deleteRole?.user_count ?? 0} pengguna.
          </>
        }
        confirmLabel="Hapus"
        pending={remove.isPending}
        onConfirm={() =>
          deleteRole &&
          remove.mutate(deleteRole.id, {
            onSuccess: () => {
              toast.success('Peran dihapus')
              setDeleteRole(null)
            },
            onError: (err) => toastError(err),
          })
        }
      />
    </>
  )
}

function IconAction({
  label,
  icon,
  onClick,
  to,
  destructive,
}: {
  label: string
  icon: ReactNode
  onClick?: () => void
  to?: string
  destructive?: boolean
}) {
  const button = (
    <Button
      variant="ghost"
      size="icon"
      aria-label={label}
      className={destructive ? 'text-destructive hover:text-destructive' : undefined}
      onClick={onClick}
      asChild={!!to}
    >
      {to ? <Link to={to}>{icon}</Link> : icon}
    </Button>
  )
  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
