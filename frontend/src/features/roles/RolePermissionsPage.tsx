import { ArrowLeft, Info, Save } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'

import { PageHeader } from '@/components/page-header'
import { ErrorState, LoadingState } from '@/components/states'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Spinner } from '@/components/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePermission } from '@/features/auth/hooks'
import { toastError } from '@/lib/errors'
import { cn } from '@/lib/utils'
import type { Action, MatrixMenu, PermissionMatrix } from '@/types/api'

import { useRoleMatrix, useSaveMatrix } from './api'

type Grants = Record<string, Action[]>

function toGrants(menus: MatrixMenu[]): Grants {
  const out: Grants = {}
  const walk = (list: MatrixMenu[]) =>
    list.forEach((m) => {
      out[m.id] = [...m.granted]
      walk(m.children)
    })
  walk(menus)
  return out
}

export function RolePermissionsPage() {
  const { id = '' } = useParams()
  const matrix = useRoleMatrix(id)

  if (matrix.isPending) return <LoadingState />
  if (matrix.error) return <ErrorState error={matrix.error} onRetry={() => matrix.refetch()} />
  return <MatrixEditor key={matrix.dataUpdatedAt} matrix={matrix.data} />
}

export function MatrixEditor({ matrix }: { matrix: PermissionMatrix }) {
  const can = usePermission()
  const save = useSaveMatrix(matrix.role.id)
  const initial = useMemo(() => toGrants(matrix.menus), [matrix])
  const [grants, setGrants] = useState<Grants>(initial)

  const editable = !matrix.read_only && can('roles', 'update')
  const dirty = JSON.stringify(grants) !== JSON.stringify(initial)

  const has = (menuId: string, a: Action) => grants[menuId]?.includes(a) ?? false
  const toggle = (menuId: string, a: Action, on: boolean) =>
    setGrants((g) => {
      const current = g[menuId] ?? []
      // Keep canonical action order so "dirty" detection is order-independent.
      const next = matrix.actions.map((x) => x.code).filter((c) => (c === a ? on : current.includes(c)))
      return { ...g, [menuId]: next }
    })

  // Menus with submenus only need "view" (the submenus carry the real actions),
  // same as the Laravel boilerplate.
  const actionsFor = (m: MatrixMenu): Action[] => (m.children.length > 0 ? ['view'] : matrix.actions.map((a) => a.code))
  // Non-super-admins can only grant what they hold; the API enforces this too.
  const canGrant = (m: MatrixMenu, a: Action) => editable && (can(m.slug, a) || initial[m.id]?.includes(a))

  const onSave = () =>
    save.mutate(
      Object.entries(grants)
        .filter(([, actions]) => actions.length > 0)
        .map(([menu_id, actions]) => ({ menu_id, actions })),
      {
        onSuccess: () => toast.success('Hak akses disimpan'),
        onError: (err) => toastError(err),
      },
    )

  const rows: { menu: MatrixMenu; depth: number }[] = []
  const flatten = (list: MatrixMenu[], depth: number) =>
    list.forEach((m) => {
      rows.push({ menu: m, depth })
      flatten(m.children, depth + 1)
    })
  flatten(matrix.menus, 0)

  return (
    <>
      <PageHeader
        title={`Hak akses: ${matrix.role.name}`}
        description="Centang aksi yang boleh dilakukan peran ini pada setiap menu."
        actions={
          <>
            <Button variant="outline" asChild>
              <Link to="/roles">
                <ArrowLeft /> Kembali
              </Link>
            </Button>
            {editable && (
              <Button onClick={onSave} disabled={!dirty || save.isPending}>
                {save.isPending ? <Spinner /> : <Save />}
                Simpan
              </Button>
            )}
          </>
        }
      />
      {matrix.read_only && (
        <Alert>
          <Info />
          <AlertDescription>Super Admin selalu memiliki semua hak akses dan tidak dapat diubah.</AlertDescription>
        </Alert>
      )}
      <div className="overflow-x-auto rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 hover:bg-muted/40">
              <TableHead>Menu</TableHead>
              {matrix.actions.map((a) => (
                <TableHead key={a.code} className="w-20 text-center">
                  {a.label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map(({ menu, depth }) => {
              const allowed = actionsFor(menu)
              return (
                <TableRow key={menu.id}>
                  <TableCell>
                    <div className={cn('flex items-center gap-2', depth > 0 && 'pl-6')}>
                      <span className={cn(depth === 0 && 'font-medium')}>{menu.name}</span>
                      {!menu.is_active && <Badge variant="secondary">nonaktif</Badge>}
                    </div>
                  </TableCell>
                  {matrix.actions.map((a) => (
                    <TableCell key={a.code} className="text-center">
                      {allowed.includes(a.code) ? (
                        <Checkbox
                          aria-label={`${a.label} ${menu.name}`}
                          checked={has(menu.id, a.code)}
                          disabled={!canGrant(menu, a.code)}
                          onCheckedChange={(c) => toggle(menu.id, a.code, c === true)}
                        />
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
