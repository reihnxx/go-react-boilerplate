import { zodResolver } from '@hookform/resolvers/zod'
import { Eye, EyeOff, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate, useSearchParams } from 'react-router'

import { FormField } from '@/components/form-field'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { isApiError } from '@/lib/api'
import { applyServerErrors, errorMessage } from '@/lib/errors'

import { safeNext } from '../guards'
import { useAuthOptions, useLogin } from '../hooks'
import { loginSchema, type LoginValues } from '../schemas'

export function LoginPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const login = useLogin()
  const options = useAuthOptions()
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { login: '', password: '' } })
  const { errors } = form.formState
  const [showPassword, setShowPassword] = useState(false)

  const onSubmit = form.handleSubmit((values) =>
    login.mutate(values, {
      onSuccess: (me) => {
        const next = safeNext(params.get('next'))
        navigate(me.requires_role_selection ? `/choose-role${next ? `?next=${encodeURIComponent(next)}` : ''}` : (next ?? '/'), {
          replace: true,
        })
      },
      onError: (err) => {
        applyServerErrors(err, form.setError, ['login', 'password'])
      },
    }),
  )

  // Field errors are shown inline; anything else (wrong credentials, throttling, network) as an alert.
  const hasFieldErrors = isApiError(login.error) && Object.keys(login.error.fields).length > 0
  const serverError = login.error && !hasFieldErrors ? errorMessage(login.error) : null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-xl">Masuk</CardTitle>
        <CardDescription>Gunakan username atau email dan kata sandi Anda.</CardDescription>
      </CardHeader>
      <form onSubmit={onSubmit} noValidate>
        <CardContent>
          <FieldGroup>
            {serverError && (
              <Alert variant="destructive">
                <TriangleAlert />
                <AlertDescription>{serverError}</AlertDescription>
              </Alert>
            )}
            <FormField label="Username atau email" htmlFor="login" error={errors.login?.message}>
              <Input id="login" autoComplete="username" autoFocus aria-invalid={!!errors.login} {...form.register('login')} />
            </FormField>
            <FormField label="Kata sandi" htmlFor="password" error={errors.password?.message}>
              <div className="relative">
                <Input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  aria-invalid={!!errors.password}
                  className="pr-10"
                  {...form.register('password')}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((prev) => !prev)}
                  className="absolute inset-y-0 right-0 flex items-center justify-center px-3 text-muted-foreground transition-colors hover:text-foreground"
                  aria-label={showPassword ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
                  aria-pressed={showPassword}
                >
                  {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              </div>
            </FormField>
            <div className="-mt-2 text-right text-sm">
              <Link to="/forgot-password" className="text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
                Lupa kata sandi?
              </Link>
            </div>
          </FieldGroup>
        </CardContent>
        <CardFooter className="mt-6 flex-col gap-3">
          <Button type="submit" className="w-full" disabled={login.isPending}>
            {login.isPending && <Spinner />}
            Masuk
          </Button>
          {options.data?.registration_enabled && (
            <p className="text-sm text-muted-foreground">
              Belum punya akun?{' '}
              <Link to="/register" className="text-foreground underline-offset-4 hover:underline">
                Daftar
              </Link>
            </p>
          )}
        </CardFooter>
      </form>
    </Card>
  )
}
