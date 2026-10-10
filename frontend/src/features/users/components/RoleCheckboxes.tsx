import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import type { Role } from '@/types/api'

interface RoleCheckboxesProps {
  roles: Role[] | undefined
  value: string[]
  onChange: (ids: string[]) => void
  /** Roles the current actor may not grant (e.g. super-admin for non-super-admins). */
  isDisabled?: (role: Role) => boolean
  idPrefix: string
}

export function RoleCheckboxes({ roles, value, onChange, isDisabled, idPrefix }: RoleCheckboxesProps) {
  if (!roles) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-5 w-32" />
      </div>
    )
  }
  if (roles.length === 0) return <p className="text-sm text-muted-foreground">Belum ada peran aktif.</p>

  const toggle = (id: string, checked: boolean) => onChange(checked ? [...value, id] : value.filter((v) => v !== id))

  return (
    <div className="grid gap-2 sm:grid-cols-2">
      {roles.map((role) => {
        const id = `${idPrefix}-${role.id}`
        const checked = value.includes(role.id)
        const disabled = isDisabled?.(role) && !checked
        return (
          <div key={role.id} className="flex items-center gap-2">
            <Checkbox id={id} checked={checked} disabled={disabled} onCheckedChange={(c) => toggle(role.id, c === true)} />
            <Label htmlFor={id} className="font-normal">
              {role.name}
              {role.is_super_admin && (
                <Badge variant="secondary" className="ml-1">
                  semua akses
                </Badge>
              )}
            </Label>
          </div>
        )
      })}
    </div>
  )
}
