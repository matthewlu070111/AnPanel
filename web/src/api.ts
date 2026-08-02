let csrf = ''

export const setCSRF = (v: string) => {
  csrf = v
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  if (init.method && init.method !== 'GET') headers.set('X-CSRF-Token', csrf)
  const r = await fetch('/api/v1' + path, {...init, headers, credentials: 'same-origin'})
  const data = await r.json().catch(() => ({error: r.statusText || 'request failed'}))
  if (!r.ok) throw new ApiError(r.status, (data as {error?: string}).error || r.statusText)
  return data as T
}

export const post = <T>(path: string, body: unknown) =>
  api<T>(path, {method: 'POST', body: JSON.stringify(body)})
