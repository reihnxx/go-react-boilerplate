import { api, isApiError, setCsrfToken } from '@/lib/api'
import type { AuthOptions, Envelope, Me } from '@/types/api'

function remember(me: Me): Me {
  setCsrfToken(me.csrf_token)
  return me
}

/** Returns the current user, or null when there is no valid session. */
export async function fetchMe(): Promise<Me | null> {
  try {
    const res = await api.get<Envelope<Me>>('/auth/me')
    return remember(res.data)
  } catch (err) {
    if (isApiError(err, 'UNAUTHENTICATED')) {
      setCsrfToken(null)
      return null
    }
    throw err
  }
}

export async function login(input: { login: string; password: string }): Promise<Me> {
  const res = await api.post<Envelope<Me>>('/auth/login', input)
  return remember(res.data)
}

export async function logout(): Promise<void> {
  await api.post('/auth/logout')
  setCsrfToken(null)
}

export async function setActiveRole(roleId: string): Promise<Me> {
  const res = await api.put<Envelope<Me>>('/auth/active-role', { role_id: roleId })
  return remember(res.data)
}

export async function stopImpersonation(): Promise<Me> {
  const res = await api.post<Envelope<Me>>('/auth/impersonation/stop')
  return remember(res.data)
}

export async function startImpersonation(userId: string): Promise<Me> {
  const res = await api.post<Envelope<Me>>(`/users/${userId}/impersonate`)
  return remember(res.data)
}

export async function fetchAuthOptions(): Promise<AuthOptions> {
  return (await api.get<Envelope<AuthOptions>>('/auth/options')).data
}

export async function forgotPassword(email: string): Promise<string> {
  return (await api.post<Envelope<{ message: string }>>('/auth/forgot-password', { email })).data.message
}

export async function resetPassword(input: { token: string; password: string; password_confirmation: string }): Promise<string> {
  return (await api.post<Envelope<{ message: string }>>('/auth/reset-password', input)).data.message
}

export async function register(input: {
  name: string
  username: string
  email: string
  password: string
  password_confirmation: string
}): Promise<string> {
  return (await api.post<Envelope<{ message: string }>>('/auth/register', input)).data.message
}
