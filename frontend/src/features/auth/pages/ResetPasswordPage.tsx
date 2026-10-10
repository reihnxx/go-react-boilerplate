import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { FormField } from '@/components/form-field'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { isApiError } from '@/lib/api'
import { handleFormError } from '@/lib/errors'

import { resetPassword } from '../api'
import { resetSchema, type ResetValues } from '../schemas'

export function ResetPasswordPage() {
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const navigate = useNavigate()
  const form = useForm<ResetValues>({ resolver: zodResolver(resetSchema), defaultValues: { password: '', password_confirmation: '' } })
  const mutation = useMutation({ mutationFn: (v: ResetValues) => resetPassword({ token, ...v }) })
  const { errors } = form.formState
  const tokenError = isApiError(mutation.error) ? mutation.error.fields.token : undefined

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-xl">Buat kata sandi baru</CardTitle>
        <CardDescription>Minimal 8 karakter, mengandung huruf dan angka.</CardDescription>
      </CardHeader>
      <form
        noValidate
        onSubmit={form.handleSubmit((v) =>
          mutation.mutate(v, {
            onSuccess: (msg) => {
              toast.success(msg)
              navigate('/login', { replace: true })
            },
            onError: (err) => handleFormError(err, form.setError, ['password', 'password_confirmation', 'token']),
          }),
        )}
      >
        <CardContent>
          <FieldGroup>
            {(!token || tokenError) && (
              <Alert variant="destructive">
                <AlertDescription>
                  {tokenError ?? 'Tautan reset tidak lengkap.'}{' '}
                  <Link to="/forgot-password" className="underline">
                    Minta tautan baru
                  </Link>
                </AlertDescription>
              </Alert>
            )}
            <FormField label="Kata sandi baru" htmlFor="password" error={errors.password?.message}>
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
        <CardFooter className="mt-6">
          <Button type="submit" className="w-full" disabled={!token || mutation.isPending}>
            {mutation.isPending && <Spinner />}
            Simpan kata sandi
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}
