import { z } from 'zod'

// Client-side checks mirror the server for faster feedback; the server is
// authoritative and its field errors are mapped back onto the forms.

export const passwordRule = z
  .string()
  .min(8, 'Kata sandi minimal 8 karakter.')
  .max(128, 'Kata sandi maksimal 128 karakter.')
  .refine((v) => /\p{L}/u.test(v) && /\p{N}/u.test(v), 'Kata sandi harus mengandung huruf dan angka.')

export const loginSchema = z.object({
  login: z.string().trim().min(1, 'Username atau email wajib diisi.'),
  password: z.string().min(1, 'Kata sandi wajib diisi.'),
})
export type LoginValues = z.infer<typeof loginSchema>

export const forgotSchema = z.object({
  email: z.string().trim().min(1, 'Email wajib diisi.').pipe(z.email('Format email tidak valid.')),
})
export type ForgotValues = z.infer<typeof forgotSchema>

export const resetSchema = z
  .object({ password: passwordRule, password_confirmation: z.string() })
  .refine((v) => v.password === v.password_confirmation, {
    path: ['password_confirmation'],
    message: 'Konfirmasi kata sandi tidak cocok.',
  })
export type ResetValues = z.infer<typeof resetSchema>

export const usernameRule = z
  .string()
  .trim()
  .min(3, 'Username minimal 3 karakter.')
  .max(50, 'Username maksimal 50 karakter.')
  .regex(/^[a-zA-Z0-9._-]+$/, 'Username hanya boleh berisi huruf, angka, titik, garis bawah, atau tanda hubung.')

export const registerSchema = z
  .object({
    name: z.string().trim().min(1, 'Nama wajib diisi.').max(255, 'Nama maksimal 255 karakter.'),
    username: usernameRule,
    email: z.string().trim().min(1, 'Email wajib diisi.').pipe(z.email('Format email tidak valid.')),
    password: passwordRule,
    password_confirmation: z.string(),
  })
  .refine((v) => v.password === v.password_confirmation, {
    path: ['password_confirmation'],
    message: 'Konfirmasi kata sandi tidak cocok.',
  })
export type RegisterValues = z.infer<typeof registerSchema>
