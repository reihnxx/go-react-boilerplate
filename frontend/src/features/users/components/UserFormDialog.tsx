import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { Controller, useForm, type UseFormRegisterReturn } from 'react-hook-form'
import { toast } from 'sonner'

import { FormField } from '@/components/form-field'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { useCurrentUser } from '@/features/auth/hooks'
import { handleFormError } from '@/lib/errors'
import type { User } from '@/types/api'

import { useCreateUser, useRoleOptions, useUpdateUser } from '../api'
import { createUserSchema, updateUserSchema, type CreateUserValues, type UpdateUserValues } from '../schemas'
import { RoleCheckboxes } from './RoleCheckboxes'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Edit this user; create a new one when undefined. */
  user?: User
}

export function UserFormDialog({ open, onOpenChange, user }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {user ? <EditForm user={user} onDone={() => onOpenChange(false)} /> : <CreateForm onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

const CREATE_FIELDS = ['name', 'username', 'email', 'password', 'password_confirmation', 'is_active', 'role_ids'] as const

function CreateForm({ onDone }: { onDone: () => void }) {
  const me = useCurrentUser()
  const roles = useRoleOptions()
  const create = useCreateUser()
  const form = useForm<CreateUserValues>({
    resolver: zodResolver(createUserSchema),
    defaultValues: { name: '', username: '', email: '', password: '', password_confirmation: '', is_active: true, role_ids: [] },
  })
  const { errors } = form.formState

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit((values) =>
        create.mutate(values, {
          onSuccess: () => {
            toast.success('Pengguna berhasil ditambahkan')
            onDone()
          },
          onError: (err) => handleFormError(err, form.setError, CREATE_FIELDS),
        }),
      )}
    >
      <DialogHeader>
        <DialogTitle>Tambah pengguna</DialogTitle>
        <DialogDescription>Kata sandi minimal 8 karakter dengan huruf dan angka.</DialogDescription>
      </DialogHeader>
      <FieldGroup className="py-4">
        <ProfileFields register={form.register} errors={form.formState.errors} />
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Kata sandi" htmlFor="password" error={errors.password?.message} required>
            <Input
              id="password"
              type="password"
              autoComplete="new-password"
              aria-invalid={!!errors.password}
              {...form.register('password')}
            />
          </FormField>
          <FormField label="Konfirmasi" htmlFor="password_confirmation" error={errors.password_confirmation?.message} required>
            <Input
              id="password_confirmation"
              type="password"
              autoComplete="new-password"
              aria-invalid={!!errors.password_confirmation}
              {...form.register('password_confirmation')}
            />
          </FormField>
        </div>
        <FormField label="Peran" htmlFor="role_ids" error={errors.role_ids?.message}>
          <Controller
            control={form.control}
            name="role_ids"
            render={({ field }) => (
              <RoleCheckboxes
                idPrefix="create-role"
                roles={roles.data}
                value={field.value}
                onChange={field.onChange}
                isDisabled={(r) => r.is_super_admin && !me.is_super_admin}
              />
            )}
          />
        </FormField>
        <Controller
          control={form.control}
          name="is_active"
          render={({ field }) => (
            <Field orientation="horizontal">
              <Switch id="is_active" checked={field.value} onCheckedChange={field.onChange} />
              <FieldLabel htmlFor="is_active" className="font-normal">
                Akun aktif
              </FieldLabel>
            </Field>
          )}
        />
      </FieldGroup>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button type="submit" disabled={create.isPending}>
          {create.isPending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </form>
  )
}

const UPDATE_FIELDS = ['name', 'username', 'email'] as const

function EditForm({ user, onDone }: { user: User; onDone: () => void }) {
  const update = useUpdateUser()
  const form = useForm<UpdateUserValues>({
    resolver: zodResolver(updateUserSchema),
    defaultValues: { name: user.name, username: user.username, email: user.email },
  })
  useEffect(() => form.reset({ name: user.name, username: user.username, email: user.email }), [user, form])

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit((values) =>
        update.mutate(
          { id: user.id, ...values },
          {
            onSuccess: () => {
              toast.success('Data pengguna diperbarui')
              onDone()
            },
            onError: (err) => handleFormError(err, form.setError, UPDATE_FIELDS),
          },
        ),
      )}
    >
      <DialogHeader>
        <DialogTitle>Ubah pengguna</DialogTitle>
        <DialogDescription>Peran, status dan kata sandi diatur dari menu aksi di tabel.</DialogDescription>
      </DialogHeader>
      <FieldGroup className="py-4">
        <ProfileFields register={form.register} errors={form.formState.errors} />
      </FieldGroup>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          Batal
        </Button>
        <Button type="submit" disabled={update.isPending}>
          {update.isPending && <Spinner />}
          Simpan
        </Button>
      </DialogFooter>
    </form>
  )
}

type ProfileKey = 'name' | 'username' | 'email'

/** Name/username/email inputs shared by the create and edit forms. */
function ProfileFields({
  register,
  errors,
}: {
  register: (name: ProfileKey) => UseFormRegisterReturn
  errors: Partial<Record<ProfileKey, { message?: string }>>
}) {
  return (
    <>
      <FormField label="Nama" htmlFor="name" error={errors.name?.message} required>
        <Input id="name" autoComplete="off" aria-invalid={!!errors.name} {...register('name')} />
      </FormField>
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="Username" htmlFor="username" error={errors.username?.message} required>
          <Input id="username" autoComplete="off" aria-invalid={!!errors.username} {...register('username')} />
        </FormField>
        <FormField label="Email" htmlFor="email" error={errors.email?.message} required>
          <Input id="email" type="email" autoComplete="off" aria-invalid={!!errors.email} {...register('email')} />
        </FormField>
      </div>
    </>
  )
}
