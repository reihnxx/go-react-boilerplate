import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router'
import { toast } from 'sonner'

import { FormField } from '@/components/form-field'
import { LoadingState } from '@/components/states'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { handleFormError } from '@/lib/errors'
import { NotFoundPage } from '@/pages/NotFoundPage'

import { register as registerAccount } from '../api'
import { useAuthOptions } from '../hooks'
import { registerSchema, type RegisterValues } from '../schemas'

const FIELDS = ['name', 'username', 'email', 'password', 'password_confirmation'] as const

export function RegisterPage() {
  const options = useAuthOptions()
  const navigate = useNavigate()
  const form = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: '', username: '', email: '', password: '', password_confirmation: '' },
  })
  const mutation = useMutation({ mutationFn: registerAccount })
  const { errors } = form.formState

  if (options.isPending) return <LoadingState />
  if (!options.data?.registration_enabled) return <NotFoundPage />

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-xl">Daftar akun</CardTitle>
        <CardDescription>Akun baru mendapat otoritas standar.</CardDescription>
      </CardHeader>
      <form
        noValidate
        onSubmit={form.handleSubmit((v) =>
          mutation.mutate(v, {
            onSuccess: (msg) => {
              toast.success(msg)
              navigate('/login', { replace: true })
            },
            onError: (err) => handleFormError(err, form.setError, FIELDS),
          }),
        )}
      >
        <CardContent>
          <FieldGroup>
            <FormField label="Nama lengkap" htmlFor="name" error={errors.name?.message}>
              <Input id="name" autoComplete="name" aria-invalid={!!errors.name} {...form.register('name')} />
            </FormField>
            <FormField label="Username" htmlFor="username" error={errors.username?.message}>
              <Input id="username" autoComplete="username" aria-invalid={!!errors.username} {...form.register('username')} />
            </FormField>
            <FormField label="Email" htmlFor="email" error={errors.email?.message}>
              <Input id="email" type="email" autoComplete="email" aria-invalid={!!errors.email} {...form.register('email')} />
            </FormField>
            <FormField label="Kata sandi" htmlFor="password" error={errors.password?.message}>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.password}
                {...form.register('password')}
              />
            </FormField>
            <FormField label="Konfirmasi kata sandi" htmlFor="password_confirmation" error={errors.password_confirmation?.message}>
              <Input
                id="password_confirmation"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.password_confirmation}
                {...form.register('password_confirmation')}
              />
            </FormField>
          </FieldGroup>
        </CardContent>
        <CardFooter className="mt-6 flex-col gap-3">
          <Button type="submit" className="w-full" disabled={mutation.isPending}>
            {mutation.isPending && <Spinner />}
            Daftar
          </Button>
          <Link to="/login" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
            Sudah punya akun? Masuk
          </Link>
        </CardFooter>
      </form>
    </Card>
  )
}
