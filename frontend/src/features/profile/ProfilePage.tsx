import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { FormField } from '@/components/form-field'
import { PageHeader } from '@/components/page-header'
import { ErrorState, LoadingState } from '@/components/states'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { passwordRule } from '@/features/auth/schemas'
import { api } from '@/lib/api'
import { handleFormError } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'
import type { Envelope, Profile } from '@/types/api'

const profileSchema = z.object({
  name: z.string().trim().min(1, 'Nama lengkap wajib diisi.').max(255),
  email: z.string().trim().min(1, 'Email wajib diisi.').pipe(z.email('Format email tidak valid.')),
  phone: z
    .string()
    .trim()
    .refine((v) => v === '' || /^(\+62|62|08)[0-9]{8,13}$/.test(v), 'Format No HP tidak valid (contoh: 08123456789).'),
  birth_place: z.string().trim().max(100, 'Tempat lahir maksimal 100 karakter.'),
  birth_date: z.string().refine((v) => v === '' || new Date(v + 'T00:00:00') <= new Date(), 'Tanggal lahir tidak boleh di masa depan.'),
  address: z.string().trim().max(500, 'Alamat maksimal 500 karakter.'),
})
type ProfileValues = z.infer<typeof profileSchema>
const PROFILE_FIELDS = ['name', 'email', 'phone', 'birth_place', 'birth_date', 'address'] as const

const passwordSchema = z
  .object({
    current_password: z.string().min(1, 'Kata sandi lama wajib diisi.'),
    password: passwordRule,
    password_confirmation: z.string(),
  })
  .refine((v) => v.password === v.password_confirmation, { path: ['password_confirmation'], message: 'Konfirmasi kata sandi tidak cocok.' })
type PasswordValues = z.infer<typeof passwordSchema>

export function ProfilePage() {
  const profile = useQuery({
    queryKey: queryKeys.profile,
    queryFn: async () => (await api.get<Envelope<Profile>>('/profile')).data,
  })

  return (
    <>
      <PageHeader title="Profil" description="Data diri dan keamanan akun Anda." />
      <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
        {profile.isPending ? (
          <Card>
            <LoadingState />
          </Card>
        ) : profile.error ? (
          <Card>
            <ErrorState error={profile.error} onRetry={() => profile.refetch()} />
          </Card>
        ) : (
          <ProfileForm key={profile.dataUpdatedAt} profile={profile.data} />
        )}
        <PasswordForm />
      </div>
    </>
  )
}

function ProfileForm({ profile }: { profile: Profile }) {
  const qc = useQueryClient()
  const save = useMutation({
    mutationFn: async (v: ProfileValues) => (await api.put<Envelope<Profile>>('/profile', v)).data,
    onSuccess: (data) => {
      qc.setQueryData(queryKeys.profile, data)
      qc.invalidateQueries({ queryKey: queryKeys.me })
      toast.success('Profil disimpan')
    },
  })
  const form = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: {
      name: profile.name,
      email: profile.email,
      phone: profile.phone ?? '',
      birth_place: profile.birth_place ?? '',
      birth_date: profile.birth_date ?? '',
      address: profile.address ?? '',
    },
  })
  const { errors, isDirty } = form.formState

  return (
    <Card>
      <form
        noValidate
        onSubmit={form.handleSubmit((v) => save.mutate(v, { onError: (err) => handleFormError(err, form.setError, PROFILE_FIELDS) }))}
      >
        <CardHeader>
          <CardTitle>Data diri</CardTitle>
          <CardDescription>
            Username: <span className="font-medium text-foreground">{profile.username}</span>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="Nama lengkap" htmlFor="p-name" error={errors.name?.message} required>
                <Input id="p-name" autoComplete="name" aria-invalid={!!errors.name} {...form.register('name')} />
              </FormField>
              <FormField label="Email" htmlFor="p-email" error={errors.email?.message} required>
                <Input id="p-email" type="email" autoComplete="email" aria-invalid={!!errors.email} {...form.register('email')} />
              </FormField>
              <FormField label="No HP" htmlFor="p-phone" error={errors.phone?.message}>
                <Input
                  id="p-phone"
                  type="tel"
                  autoComplete="tel"
                  placeholder="08123456789"
                  aria-invalid={!!errors.phone}
                  {...form.register('phone')}
                />
              </FormField>
              <FormField label="Tempat lahir" htmlFor="p-birth-place" error={errors.birth_place?.message}>
                <Input id="p-birth-place" aria-invalid={!!errors.birth_place} {...form.register('birth_place')} />
              </FormField>
              <FormField label="Tanggal lahir" htmlFor="p-birth-date" error={errors.birth_date?.message}>
                <Input id="p-birth-date" type="date" aria-invalid={!!errors.birth_date} {...form.register('birth_date')} />
              </FormField>
            </div>
            <FormField label="Alamat" htmlFor="p-address" error={errors.address?.message}>
              <Textarea id="p-address" rows={3} aria-invalid={!!errors.address} {...form.register('address')} />
            </FormField>
          </FieldGroup>
        </CardContent>
        <CardFooter className="mt-6 justify-end">
          <Button type="submit" disabled={!isDirty || save.isPending}>
            {save.isPending && <Spinner />}
            Simpan profil
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}

function PasswordForm() {
  const change = useMutation({ mutationFn: (v: PasswordValues) => api.put('/profile/password', v) })
  const form = useForm<PasswordValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', password: '', password_confirmation: '' },
  })
  const { errors } = form.formState

  return (
    <Card className="h-fit">
      <form
        noValidate
        onSubmit={form.handleSubmit((v) =>
          change.mutate(v, {
            onSuccess: () => {
              form.reset()
              toast.success('Kata sandi diubah', { description: 'Sesi lain di perangkat berbeda telah dikeluarkan.' })
            },
            onError: (err) => handleFormError(err, form.setError, ['current_password', 'password', 'password_confirmation']),
          }),
        )}
      >
        <CardHeader>
          <CardTitle>Ganti kata sandi</CardTitle>
          <CardDescription>Minimal 8 karakter, mengandung huruf dan angka.</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <FormField label="Kata sandi lama" htmlFor="pw-current" error={errors.current_password?.message}>
              <Input
                id="pw-current"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!errors.current_password}
                {...form.register('current_password')}
              />
            </FormField>
            <FormField label="Kata sandi baru" htmlFor="pw-new" error={errors.password?.message}>
              <Input
                id="pw-new"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.password}
                {...form.register('password')}
              />
            </FormField>
            <FormField label="Konfirmasi" htmlFor="pw-confirm" error={errors.password_confirmation?.message}>
              <Input
                id="pw-confirm"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.password_confirmation}
                {...form.register('password_confirmation')}
              />
            </FormField>
          </FieldGroup>
        </CardContent>
        <CardFooter className="mt-6 justify-end">
          <Button type="submit" disabled={change.isPending}>
            {change.isPending && <Spinner />}
            Ubah kata sandi
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}
