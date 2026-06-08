import type { ErrorEnvelope } from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const apiKeyStorageKey = 'tg-search.api-key'

let apiKey = readStoredAPIKey()

export function setAPIKey(key: string) {
  apiKey = key
  writeStoredAPIKey(key)
}

export function clearAPIKey() {
  apiKey = ''
  removeStoredAPIKey()
}

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: jsonHeaders()
  })
  return readResponse<T>(response)
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'include',
    headers: jsonHeaders(true),
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  return readResponse<T>(response)
}

export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'PUT',
    credentials: 'include',
    headers: jsonHeaders(true),
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  return readResponse<T>(response)
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'PATCH',
    credentials: 'include',
    headers: jsonHeaders(true),
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  return readResponse<T>(response)
}

export async function apiDelete<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    method: 'DELETE',
    credentials: 'include',
    headers: jsonHeaders()
  })
  return readResponse<T>(response)
}

function jsonHeaders(contentType = false) {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (contentType) {
    headers['Content-Type'] = 'application/json'
  }
  const key = currentAPIKey()
  if (key) {
    headers['X-API-Key'] = key
  }
  return headers
}

function currentAPIKey() {
  if (!apiKey) {
    apiKey = readStoredAPIKey()
  }
  return apiKey
}

function readStoredAPIKey() {
  try {
    return localStorage.getItem(apiKeyStorageKey) ?? ''
  } catch {
    return ''
  }
}

function writeStoredAPIKey(key: string) {
  try {
    if (key) {
      localStorage.setItem(apiKeyStorageKey, key)
    } else {
      localStorage.removeItem(apiKeyStorageKey)
    }
  } catch {
    // Storage can be unavailable in private or embedded browser contexts.
  }
}

function removeStoredAPIKey() {
  try {
    localStorage.removeItem(apiKeyStorageKey)
  } catch {
    // Storage can be unavailable in private or embedded browser contexts.
  }
}

async function readResponse<T>(response: Response): Promise<T> {
  const data = await response.json().catch(() => undefined)
  if (!response.ok) {
    const envelope = data as ErrorEnvelope | undefined
    throw new ApiError(
      response.status,
      envelope?.error?.code ?? 'http_error',
      envelope?.error?.message ?? `request failed with ${response.status}`
    )
  }
  return data as T
}
