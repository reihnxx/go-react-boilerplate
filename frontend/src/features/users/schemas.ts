import { z } from 'zod'

import { passwordRule, usernameRule } from '@/features/auth/schemas'

const profileFields = {
  name: z.string().trim().min(1, 'Nama wajib diisi.').max(255, 'Nama maksimal 255 karakter.'),
  username: usernameRule,
  email: z.string().trim().min(1, 'Email wajib diisi.').pipe(z.email('Format email tidak valid.')),
}

const confirmPassword = <T extends { password: string; password_confirmation: string }>(v: T) => v.password === v.password_confirmation

export const createUserSchema = z
  .object({
    ...profileFields,
    password: passwordRule,
    password_confirmation: z.string(),
    is_active: z.boolean(),
    role_ids: z.array(z.string()),
  })
  .refine(confirmPassword, { path: ['password_confirmation'], message: 'Konfirmasi kata sandi tidak cocok.' })
export type CreateUserValues = z.infer<typeof createUserSchema>

export const updateUserSchema = z.object(profileFields)
export type UpdateUserValues = z.infer<typeof updateUserSchema>

export const setPasswordSchema = z
  .object({ password: passwordRule, password_confirmation: z.string() })
  .refine(confirmPassword, { path: ['password_confirmation'], message: 'Konfirmasi kata sandi tidak cocok.' })
export type SetPasswordValues = z.infer<typeof setPasswordSchema>
