import { Check, ShieldCheck } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { toastError } from '@/lib/errors'
import { cn } from '@/lib/utils'

import { safeNext } from '../guards'
import { useCurrentUser, useLogout, useSetActiveRole } from '../hooks'

export function ChooseRolePage() {
  const me = useCurrentUser()
  const setRole = useSetActiveRole()
  const logout = useLogout()
  const navigate = useNavigate()
  const [params] = useSearchParams()

  const choose = (roleId: string) =>
    setRole.mutate(roleId, {
      onSuccess: () => navigate(safeNext(params.get('next')) ?? '/', { replace: true }),
      onError: (err) => toastError(err),
    })

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-xl">Pilih otoritas</CardTitle>
        <CardDescription>
          {me.roles.length > 0
            ? 'Akun Anda memiliki lebih dari satu otoritas. Pilih salah satu untuk melanjutkan.'
            : 'Akun Anda belum memiliki otoritas. Hubungi administrator.'}
        </CardDescription>
      </CardHeader>
      {me.roles.length > 0 && (
        <CardContent className="grid gap-2">
          {me.roles.map((role) => {
            const active = me.active_role?.id === role.id
            const pending = setRole.isPending && setRole.variables === role.id
            return (
              <button
                key={role.id}
                type="button"
                disabled={setRole.isPending}
                onClick={() => choose(role.id)}
                className={cn(
                  'flex items-center gap-3 rounded-lg border p-3 text-left text-sm transition-colors hover:bg-accent disabled:opacity-60',
                  active && 'border-primary bg-primary/5',
                )}
              >
                <ShieldCheck className="size-5 text-primary" />
                <span className="flex-1 font-medium">{role.name}</span>
                {pending ? <Spinner /> : active ? <Check className="size-4 text-primary" aria-label="Aktif" /> : null}
              </button>
            )
          })}
        </CardContent>
      )}
      <CardFooter className="mt-2 justify-center">
        <Button
          variant="link"
          disabled={logout.isPending}
          onClick={() => logout.mutate(undefined, { onSettled: () => navigate('/login', { replace: true }) })}
        >
          Bukan Anda? Keluar
        </Button>
      </CardFooter>
    </Card>
  )
}
