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

/** Multipart upload with optional progress (0–100). Do not set Content-Type manually. */
export function uploadFile(
  dir: string,
  file: File,
  opts?: {overwrite?: boolean; onProgress?: (pct: number) => void},
): Promise<{ok: boolean; files: string[]; path: string}> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/v1/files/upload')
    xhr.withCredentials = true
    if (csrf) xhr.setRequestHeader('X-CSRF-Token', csrf)
    xhr.upload.onprogress = e => {
      if (e.lengthComputable && opts?.onProgress) {
        opts.onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }
    xhr.onload = () => {
      let data: {error?: string; ok?: boolean; files?: string[]; path?: string} = {}
      try { data = JSON.parse(xhr.responseText) } catch { /* */ }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({ok: true, files: data.files || [file.name], path: data.path || dir})
      } else {
        reject(new ApiError(xhr.status, data.error || xhr.statusText || 'upload failed'))
      }
    }
    xhr.onerror = () => reject(new ApiError(0, 'network error'))
    const form = new FormData()
    form.append('path', dir)
    form.append('file', file, file.name)
    if (opts?.overwrite) form.append('overwrite', '1')
    xhr.send(form)
  })
}

export function downloadURL(path: string) {
  return `/api/v1/files/download?path=${encodeURIComponent(path)}`
}
