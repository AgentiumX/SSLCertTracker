import type { Overview, DomainsResponse, DomainDetail } from './types'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
  return res.json()
}

export const api = {
  overview: () => get<Overview>('/api/dashboard/overview'),
  domains: () => get<DomainsResponse>('/api/dashboard/domains'),
  domainDetail: (id: number | string) => get<DomainDetail>(`/api/dashboard/domains/${id}`),
}
