import { zodResolver } from '@hookform/resolvers/zod'
import { Controller, useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { FormField } from '@/components/form-field'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { handleFormError } from '@/lib/errors'
import type { Role } from '@/types/api'

import { useCreateRole, useUpdateRole } from './api'

const schema = z.object({
  name: z.string().trim().min(1, 'Nama peran wajib diisi.').max(100, 'Nama peran maksimal 100 karakter.'),
  is_active: z.boolean(),
})
type Values = z.infer<typeof schema>

export function RoleFormDialog({ open, onOpenChange, role }: { open: boolean; onOpenChange: (o: boolean) => void; role?: Role }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {open && <RoleForm key={role?.id ?? 'new'} role={role} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function RoleForm({ role, onDone }: { role?: Role; onDone: () => void }) {
  const create = useCreateRole()
  const update = useUpdateRole()
  const pending = create.isPending || update.isPending
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: role?.name ?? '', is_active: role?.is_active ?? true },
  })
  const { errors } = form.formState

  const onSubmit = form.handleSubmit((values) => {
    const opts = {
      onSuccess: () => {
        toast.success(role ? 'Peran diperbarui' : 'Peran ditambahkan')
        onDone()
      },
      onError: (err: unknown) => handleFormError(err, form.setError, ['name', 'is_active']),
    }
    if (role) update.mutate({ id: role.id, ...values }, opts)
    else create.mutate({ name: values.name }, opts)
  })

  return (
    <form noValidate onSubmit={onSubmit}>
      <DialogHeader>
        <DialogTitle>{role ? 'Ubah peran' : 'Tambah peran'}</DialogTitle>
        <DialogDescription>
          {role ? (
            <>
              Slug <code className="rounded bg-muted px-1">{role.slug}</code> tidak berubah walau nama diganti.
            </>
          ) : (
            'Atur hak aksesnya setelah peran dibuat.'
          )}
        </DialogDescription>
      </DialogHeader>
      <FieldGroup className="py-4">
        <FormField label="Nama peran" htmlFor="role-name" error={errors.name?.message} required>
          <Input id="role-name" autoComplete="off" autoFocus aria-invalid={!!errors.name} {...form.register('name')} />
        </FormField>
        {role && !role.is_system && (
          <Controller
            control={form.control}
            name="is_active"
            render={({ field }) => (
              <Field orientation="horizontal">
                <Switch id="role-active" checked={field.value} onCheckedChange={field.onChange} />
                <FieldLabel htmlFor="role-active" className="font-normal">
                  Peran aktif
                </FieldLabel>
              </Field>
            )}
          />
        )}
      </FieldGroup>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button type="submit" disabled={pending}>
          {pending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </form>
  )
}
