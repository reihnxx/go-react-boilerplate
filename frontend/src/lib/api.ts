// Thin fetch wrapper for the Go API.
//
// - Cookies carry the session (HttpOnly, same-origin); nothing auth-related is
//   stored in localStorage/sessionStorage.
// - The CSRF token lives only in memory and is sent on unsafe methods.
// - Errors are normalised into ApiError using the API's error envelope.

const BASE = import.meta.env.VITE_API_BASE ?? '/api/v1'

let csrfToken: string | null = null

export function setCsrfToken(token: string | null) {
  csrfToken = token
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly requestId?: string

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}, requestId?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
    this.requestId = requestId
  }
}

export function isApiError(err: unknown, code?: string): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code)
}

type Handlers = {
  onUnauthenticated?: () => void
  onRoleSelectionRequired?: () => void
}

let handlers: Handlers = {}

/** Registers app-level reactions to session loss / missing active role. */
export function setApiHandlers(h: Handlers) {
  handlers = h
}

export type QueryValue = string | number | boolean | null | undefined
type Query = Record<string, QueryValue>

export function buildQuery(query?: Query): string {
  if (!query) return ''
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    params.set(key, String(value))
  }
  const s = params.toString()
  return s ? `?${s}` : ''
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
}

const UNSAFE = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

/** Performs a request and returns the parsed JSON body (undefined for 204). */
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const method = opts.method ?? 'GET'
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (UNSAFE.has(method) && csrfToken) headers['X-CSRF-Token'] = csrfToken

  let res: Response
  try {
    res = await fetch(BASE + path + buildQuery(opts.query), {
      method,
      headers,
      credentials: 'same-origin',
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      signal: opts.signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server. Periksa koneksi Anda.')
  }

  if (res.status === 204) return undefined as T

  let payload: unknown = undefined
  const text = await res.text()
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = undefined
    }
  }

  if (!res.ok) {
    const e = (payload as { error?: { code?: string; message?: string; fields?: Record<string, string>; request_id?: string } } | undefined)
      ?.error
    const error = new ApiError(
      res.status,
      e?.code ?? 'HTTP_' + res.status,
      e?.message ?? 'Terjadi kesalahan. Silakan coba lagi.',
      e?.fields ?? {},
      e?.request_id,
    )
    if (error.code === 'UNAUTHENTICATED') handlers.onUnauthenticated?.()
    if (error.code === 'ROLE_SELECTION_REQUIRED') handlers.onRoleSelectionRequired?.()
    throw error
  }
  return payload as T
}

export const api = {
  get: <T>(path: string, query?: Query, signal?: AbortSignal) => request<T>(path, { query, signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body: body ?? {} }),
  put: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body: body ?? {} }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PATCH', body: body ?? {} }),
  delete: <T = void>(path: string) => request<T>(path, { method: 'DELETE' }),
}
