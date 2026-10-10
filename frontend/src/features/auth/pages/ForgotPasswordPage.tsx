import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { MailCheck } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { Link } from 'react-router'

import { FormField } from '@/components/form-field'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { handleFormError } from '@/lib/errors'

import { forgotPassword } from '../api'
import { forgotSchema, type ForgotValues } from '../schemas'

export function ForgotPasswordPage() {
  const form = useForm<ForgotValues>({ resolver: zodResolver(forgotSchema), defaultValues: { email: '' } })
  const mutation = useMutation({ mutationFn: (v: ForgotValues) => forgotPassword(v.email) })
  const { errors } = form.formState

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-xl">Lupa kata sandi</CardTitle>
        <CardDescription>Masukkan email akun Anda. Kami akan mengirim tautan untuk membuat kata sandi baru.</CardDescription>
      </CardHeader>
      <form
        noValidate
        onSubmit={form.handleSubmit((v) => mutation.mutate(v, { onError: (err) => handleFormError(err, form.setError, ['email']) }))}
      >
        <CardContent className="space-y-4">
          {mutation.isSuccess ? (
            <Alert>
              <MailCheck />
              <AlertDescription>{mutation.data}</AlertDescription>
            </Alert>
          ) : (
            <FormField label="Email" htmlFor="email" error={errors.email?.message}>
              <Input id="email" type="email" autoComplete="email" autoFocus aria-invalid={!!errors.email} {...form.register('email')} />
            </FormField>
          )}
        </CardContent>
        <CardFooter className="mt-6 flex-col gap-3">
          {!mutation.isSuccess && (
            <Button type="submit" className="w-full" disabled={mutation.isPending}>
              {mutation.isPending && <Spinner />}
              Kirim tautan reset
            </Button>
          )}
          <Link to="/login" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
            Kembali ke halaman masuk
          </Link>
        </CardFooter>
      </form>
    </Card>
  )
}
