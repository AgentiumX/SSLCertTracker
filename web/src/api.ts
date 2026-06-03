import type { Overview, DomainsResponse, DomainDetail, User } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error?.message) message = body.error.message
    } catch {
      // ignore parse errors, fall back to status text
    }
    throw new Error(message)
  }
  return res.json()
}

export const api = {
  overview: () => request<Overview>('/api/dashboard/overview'),
  domains: () => request<DomainsResponse>('/api/dashboard/domains'),
  domainDetail: (id: number | string) =>
    request<DomainDetail>(`/api/dashboard/domains/${id}`),
}

export const authApi = {
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () =>
    request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  me: () => request<{ user: User }>('/api/auth/me'),
}
