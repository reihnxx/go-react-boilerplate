import { CornerDownRight, EllipsisVertical, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTable, type Column } from '@/components/data-table'
import { PageHeader } from '@/components/page-header'
import { SearchInput } from '@/components/search-input'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { usePermission } from '@/features/auth/hooks'
import { useListParams } from '@/hooks/use-list-params'
import { toastError } from '@/lib/errors'
import { DynamicIcon } from '@/components/dynamic-icon'
import type { Menu } from '@/types/api'

import { useDeleteMenu, useMenuGroups, useMenus } from './api'
import { MenuFormDialog } from './MenuFormDialog'

const FILTERS = ['type', 'group_id']
const ALL = '__all__'

export function MenusPage() {
  const can = usePermission()
  const { params, query, setParam, toggleSort } = useListParams({ sort: 'position', order: 'asc', filterKeys: FILTERS })
  const menus = useMenus(query)
  const groups = useMenuGroups()
  const remove = useDeleteMenu()
  const [formOpen, setFormOpen] = useState(false)
  const [editMenu, setEditMenu] = useState<Menu | undefined>()
  const [deleteMenu, setDeleteMenu] = useState<Menu | null>(null)

  const columns: Column<Menu>[] = [
    {
      key: 'name',
      header: 'Menu',
      sortKey: 'name',
      cell: (m) => {
        return (
          <div className="flex items-center gap-2">
            {m.type === 'child' ? (
              <CornerDownRight className="ml-2 size-4 text-muted-foreground" />
            ) : (
              <DynamicIcon icon={m.icon} className="size-4 text-primary" />
            )}
            <div className="min-w-0">
              <div className="truncate font-medium">{m.name}</div>
              <code className="text-xs text-muted-foreground">{m.slug}</code>
            </div>
          </div>
        )
      },
    },
    {
      key: 'group',
      header: 'Grup / induk',
      className: 'hidden md:table-cell',
      cell: (m) => (
        <span className="text-sm text-muted-foreground">
          {m.group_name}
          {m.parent_name && ` › ${m.parent_name}`}
        </span>
      ),
    },
    {
      key: 'path',
      header: 'Path',
      cell: (m) => (m.path ? <code className="text-sm">{m.path}</code> : <span className="text-muted-foreground">—</span>),
    },
    {
      key: 'sort_order',
      header: 'Urutan',
      sortKey: 'sort_order',
      className: 'hidden w-20 sm:table-cell',
      cell: (m) => <span className="tabular-nums">{m.sort_order}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      className: 'w-24',
      cell: (m) => <Badge variant={m.is_active ? 'outline' : 'secondary'}>{m.is_active ? 'Aktif' : 'Nonaktif'}</Badge>,
    },
    {
      key: 'actions',
      header: <span className="sr-only">Aksi</span>,
      className: 'w-12 text-right',
      cell: (m) =>
        (can('menus', 'update') || can('menus', 'delete')) && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={`Aksi untuk ${m.name}`}>
                <EllipsisVertical />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              {can('menus', 'update') && (
                <DropdownMenuItem
                  onSelect={() => {
                    setEditMenu(m)
                    setFormOpen(true)
                  }}
                >
                  <Pencil /> Ubah
                </DropdownMenuItem>
              )}
              {can('menus', 'delete') && (
                <DropdownMenuItem variant="destructive" onSelect={() => setDeleteMenu(m)}>
                  <Trash2 /> Hapus
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Manajemen Menu"
        description="Menu sidebar dan kunci hak akses (slug) untuk setiap halaman."
        actions={
          can('menus', 'create') && (
            <Button
              onClick={() => {
                setEditMenu(undefined)
                setFormOpen(true)
              }}
            >
              <Plus /> Tambah menu
            </Button>
          )
        }
      />
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <SearchInput value={params.q} onChange={(v) => setParam('q', v)} placeholder="Cari nama, slug, path…" />
        <Select value={params.filters.type ?? ALL} onValueChange={(v) => setParam('type', v === ALL ? null : v)}>
          <SelectTrigger className="w-full sm:w-40" aria-label="Filter jenis">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Semua jenis</SelectItem>
            <SelectItem value="main">Menu utama</SelectItem>
            <SelectItem value="child">Submenu</SelectItem>
          </SelectContent>
        </Select>
        <Select value={params.filters.group_id ?? ALL} onValueChange={(v) => setParam('group_id', v === ALL ? null : v)}>
          <SelectTrigger className="w-full sm:w-44" aria-label="Filter grup">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Semua grup</SelectItem>
            {groups.data?.map((g) => (
              <SelectItem key={g.id} value={g.id}>
                {g.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <DataTable
        columns={columns}
        rows={menus.data?.data}
        rowKey={(m) => m.id}
        isLoading={menus.isPending}
        error={menus.error}
        onRetry={() => menus.refetch()}
        sort={{ key: params.sort, order: params.order }}
        onSort={toggleSort}
        meta={menus.data?.meta}
        onPageChange={(p) => setParam('page', p)}
      />
      <MenuFormDialog open={formOpen} onOpenChange={setFormOpen} menu={editMenu} />
      <ConfirmDialog
        open={!!deleteMenu}
        onOpenChange={(o) => !o && setDeleteMenu(null)}
        title="Hapus menu?"
        description={
          <>
            Menu <strong>{deleteMenu?.name}</strong> dan semua hak akses pada menu ini akan dihapus.
          </>
        }
        confirmLabel="Hapus"
        pending={remove.isPending}
        onConfirm={() =>
          deleteMenu &&
          remove.mutate(deleteMenu.id, {
            onSuccess: () => {
              toast.success('Menu dihapus')
              setDeleteMenu(null)
            },
            onError: (err) => toastError(err),
          })
        }
      />
    </>
  )
}
