import type { ReactNode } from 'react'

import { Field, FieldDescription, FieldError, FieldLabel } from '@/components/ui/field'

interface FormFieldProps {
  label: ReactNode
  htmlFor: string
  error?: string
  description?: ReactNode
  required?: boolean
  children: ReactNode
  className?: string
}

/**
 * Label + control + error. Pass the input as children and wire it with
 * react-hook-form's register(); set aria-invalid on the control yourself.
 */
export function FormField({ label, htmlFor, error, description, required, children, className }: FormFieldProps) {
  return (
    <Field data-invalid={!!error || undefined} className={className}>
      <FieldLabel htmlFor={htmlFor}>
        {label}
        {required && <span className="text-destructive">*</span>}
      </FieldLabel>
      {children}
      {description && !error && <FieldDescription>{description}</FieldDescription>}
      {error && <FieldError id={`${htmlFor}-error`}>{error}</FieldError>}
    </Field>
  )
}
