import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'

import { FormField } from '@/components/form-field'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { handleFormError } from '@/lib/errors'
import type { User } from '@/types/api'

import { useSetUserPassword } from '../api'
import { setPasswordSchema, type SetPasswordValues } from '../schemas'

export function UserPasswordDialog({ user, onOpenChange }: { user: User | null; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={!!user} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {user && <PasswordForm key={user.id} user={user} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function PasswordForm({ user, onDone }: { user: User; onDone: () => void }) {
  const setPassword = useSetUserPassword()
  const form = useForm<SetPasswordValues>({
    resolver: zodResolver(setPasswordSchema),
    defaultValues: { password: '', password_confirmation: '' },
  })
  const { errors } = form.formState

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit((values) =>
        setPassword.mutate(
          { id: user.id, ...values },
          {
            onSuccess: () => {
              toast.success('Kata sandi diperbarui', { description: 'Semua sesi pengguna tersebut telah diakhiri.' })
              onDone()
            },
            onError: (err) => handleFormError(err, form.setError, ['password', 'password_confirmation']),
          },
        ),
      )}
    >
      <DialogHeader>
        <DialogTitle>Atur kata sandi</DialogTitle>
        <DialogDescription>
          Kata sandi baru untuk <strong>{user.name}</strong>. Pengguna akan dikeluarkan dari semua sesi.
        </DialogDescription>
      </DialogHeader>
      <FieldGroup className="py-4">
        <FormField label="Kata sandi baru" htmlFor="new-password" error={errors.password?.message} required>
          <Input
            id="new-password"
            type="password"
            autoComplete="new-password"
            aria-invalid={!!errors.password}
            {...form.register('password')}
          />
        </FormField>
        <FormField label="Konfirmasi" htmlFor="new-password-confirmation" error={errors.password_confirmation?.message} required>
          <Input
            id="new-password-confirmation"
            type="password"
            autoComplete="new-password"
            aria-invalid={!!errors.password_confirmation}
            {...form.register('password_confirmation')}
          />
        </FormField>
      </FieldGroup>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button type="submit" disabled={setPassword.isPending}>
          {setPassword.isPending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </form>
  )
}
