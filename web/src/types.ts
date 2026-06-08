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

export interface User {
  id: number
  username: string
}

export interface DomainAdmin {
  id: number
  host: string
  port: number
  protocol: string
  is_global: boolean
  remark: string
  created_at: string
}

export interface AgentAdmin {
  agent_id: string
  display_name: string
  hostname: string
  ip: string
  remark: string
  registered_at: string
  last_seen_at: string
  is_online: boolean
}

export interface Override {
  domain_id: number
  action: 'include' | 'exclude'
}

export interface AlertChannel {
  id: number
  name: string
  type: 'webhook' | 'dingtalk' | 'feishu' | 'wecom' | 'email'
  config?: string
  enabled: boolean
  created_at: string
}

export interface AlertChannelInput {
  name: string
  type: AlertChannel['type']
  config: string
  enabled: boolean
}
