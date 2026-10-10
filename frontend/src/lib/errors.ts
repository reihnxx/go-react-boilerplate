import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'
import { toast } from 'sonner'

import { isApiError } from '@/lib/api'

export function errorMessage(err: unknown): string {
  if (isApiError(err)) return err.message
  return 'Terjadi kesalahan yang tidak terduga.'
}

/** Shows an error toast for err. */
export function toastError(err: unknown, title = 'Gagal') {
  toast.error(title, { description: errorMessage(err) })
}

/**
 * Maps server-side field errors (422) onto the form. Field names the form
 * does not know are shown in a toast. Returns true when anything was mapped.
 */
export function applyServerErrors<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>,
  knownFields: readonly string[],
): boolean {
  if (!isApiError(err) || Object.keys(err.fields).length === 0) return false
  const unknown: string[] = []
  for (const [field, message] of Object.entries(err.fields)) {
    if (knownFields.includes(field)) setError(field as Path<T>, { type: 'server', message })
    else unknown.push(message)
  }
  if (unknown.length > 0) toast.error('Data tidak valid', { description: unknown.join(' ') })
  return true
}

/** Standard mutation error handler for forms. */
export function handleFormError<T extends FieldValues>(err: unknown, setError: UseFormSetError<T>, knownFields: readonly string[]) {
  if (!applyServerErrors(err, setError, knownFields)) toastError(err)
}
