export type Status = 'ok' | 'expiring' | 'expired' | 'mismatch' | 'unreachable' | ''

export interface Overview {
  total_domains: number
  healthy_domains: number
  alert_domains: number
  agents_online: number
  agents_total: number
}

export interface DomainSummary {
  id: number
  host: string
  port: number
  protocol: string
  is_global: boolean
  remark: string
  healthy_count: number
  total_checks: number
  worst_status: Status
}

export interface DomainsResponse {
  domains: DomainSummary[]
}

export interface AgentResultRow {
  agent_id: string
  agent_display_name: string
  agent_online: boolean
  checked_at: string
  status: Status
  not_after: string | null
  issuer: string
  subject: string
  sans: string
  error_message: string
}

export interface DomainDetail {
  domain: {
    id: number
    host: string
    port: number
    protocol: string
    remark: string
  }
  results: AgentResultRow[]
}
