import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiError, json, mockApi } from '@/test/utils'

import { api, ApiError, buildQuery, isApiError, request, setApiHandlers, setCsrfToken } from './api'

afterEach(() => {
  setCsrfToken(null)
  setApiHandlers({})
})

describe('buildQuery', () => {
  it('skips empty values', () => {
    expect(buildQuery({ page: 2, q: '', sort: 'name', x: undefined, y: null, active: false })).toBe('?page=2&sort=name&active=false')
    expect(buildQuery({})).toBe('')
  })
})

describe('request', () => {
  it('sends the CSRF token on unsafe methods only', async () => {
    const { calls } = mockApi({ 'GET /things': () => json(200, { data: 1 }), 'POST /things': () => json(201, { data: 2 }) })
    setCsrfToken('secret-token')
    await api.get('/things')
    await api.post('/things', { a: 1 })
    expect(calls[0].headers.get('X-CSRF-Token')).toBeNull()
    expect(calls[1].headers.get('X-CSRF-Token')).toBe('secret-token')
    expect(calls[1].body).toEqual({ a: 1 })
  })

  it('uses same-origin credentials and never sends auth headers', async () => {
    const { fetchMock } = mockApi({ 'GET /x': () => json(200, { data: null }) })
    await api.get('/x')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(init.credentials).toBe('same-origin')
    expect(new Headers(init.headers).get('Authorization')).toBeNull()
  })

  it('parses the error envelope into ApiError', async () => {
    mockApi({ 'POST /users': () => apiError(422, 'VALIDATION_ERROR', 'Data tidak valid', { email: 'Email sudah digunakan.' }) })
    const err = (await api.post('/users', {}).catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(isApiError(err, 'VALIDATION_ERROR')).toBe(true)
    expect(err.status).toBe(422)
    expect(err.fields).toEqual({ email: 'Email sudah digunakan.' })
  })

  it('returns undefined for 204', async () => {
    mockApi({ 'DELETE /users/1': () => new Response(null, { status: 204 }) })
    await expect(api.delete('/users/1')).resolves.toBeUndefined()
  })

  it('notifies handlers on session loss and missing role', async () => {
    const onUnauthenticated = vi.fn()
    const onRoleSelectionRequired = vi.fn()
    setApiHandlers({ onUnauthenticated, onRoleSelectionRequired })
    mockApi({
      'GET /a': () => apiError(401, 'UNAUTHENTICATED'),
      'GET /b': () => apiError(403, 'ROLE_SELECTION_REQUIRED'),
      'POST /auth/login': () => apiError(401, 'INVALID_CREDENTIALS'),
    })
    await request('/a').catch(() => {})
    await request('/b').catch(() => {})
    await api.post('/auth/login', {}).catch(() => {})
    expect(onUnauthenticated).toHaveBeenCalledTimes(1) // not for INVALID_CREDENTIALS
    expect(onRoleSelectionRequired).toHaveBeenCalledTimes(1)
  })

  it('maps network failures to NETWORK_ERROR', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const err = (await api.get('/x').catch((e) => e)) as ApiError
    expect(isApiError(err, 'NETWORK_ERROR')).toBe(true)
  })

  it('tolerates non-JSON error bodies', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>Bad gateway</html>', { status: 502 })))
    const err = (await api.get('/x').catch((e) => e)) as ApiError
    expect(err.code).toBe('HTTP_502')
  })
})
