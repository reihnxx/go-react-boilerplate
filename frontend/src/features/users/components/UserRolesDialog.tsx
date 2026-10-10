import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Spinner } from '@/components/ui/spinner'
import { useCurrentUser } from '@/features/auth/hooks'
import { toastError } from '@/lib/errors'
import type { User } from '@/types/api'

import { useRoleOptions, useSetUserRoles } from '../api'
import { RoleCheckboxes } from './RoleCheckboxes'

export function UserRolesDialog({ user, onOpenChange }: { user: User | null; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={!!user} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {user && <RolesForm key={user.id} user={user} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function RolesForm({ user, onDone }: { user: User; onDone: () => void }) {
  const me = useCurrentUser()
  const roles = useRoleOptions()
  const setRoles = useSetUserRoles()
  const [value, setValue] = useState(() => user.roles.map((r) => r.id))

  return (
    <>
      <DialogHeader>
        <DialogTitle>Peran pengguna</DialogTitle>
        <DialogDescription>
          Atur otoritas untuk <strong>{user.name}</strong>. Jika lebih dari satu, pengguna memilih salah satu saat masuk.
        </DialogDescription>
      </DialogHeader>
      <div className="py-4">
        <RoleCheckboxes
          idPrefix="user-role"
          roles={roles.data}
          value={value}
          onChange={setValue}
          isDisabled={(r) => r.is_super_admin && !me.is_super_admin}
        />
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button
          disabled={setRoles.isPending}
          onClick={() =>
            setRoles.mutate(
              { id: user.id, role_ids: value },
              {
                onSuccess: () => {
                  toast.success('Peran pengguna diperbarui')
                  onDone()
                },
                onError: (err) => toastError(err),
              },
            )
          }
        >
          {setRoles.isPending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </>
  )
}
