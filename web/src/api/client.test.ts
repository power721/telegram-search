import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiDelete, apiGet, apiPost, apiPut } from './client'

describe('api client', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('returns JSON for successful GET requests', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ service: 'ok' })
    } as Response)

    await expect(apiGet('/api/status')).resolves.toEqual({ service: 'ok' })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/status', {
      credentials: 'include',
      headers: { Accept: 'application/json' }
    })
  })

  it('throws ApiError for error envelopes', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({ error: { code: 'bad_request', message: 'invalid' } })
    } as Response)

    await expect(apiPost('/api/auth/login', { username: 'a' })).rejects.toBeInstanceOf(ApiError)
    await expect(apiPost('/api/auth/login', { username: 'a' })).rejects.toMatchObject({
      code: 'bad_request',
      message: 'invalid',
      status: 400
    })
  })

  it('sends DELETE requests with credentials', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ deleted: true })
    } as Response)

    await expect(apiDelete('/api/accounts/1')).resolves.toEqual({ deleted: true })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/accounts/1', {
      method: 'DELETE',
      credentials: 'include',
      headers: { Accept: 'application/json' }
    })
  })

  it('sends PUT requests with credentials and content type', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ updated: true })
    } as Response)

    await apiPut('/api/listen-rules', { message_types: ['link'] })

    expect(globalThis.fetch).toHaveBeenCalledWith('/api/listen-rules', {
      method: 'PUT',
      credentials: 'include',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ message_types: ['link'] })
    })
  })
})
