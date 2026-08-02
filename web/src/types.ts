export type Snapshot = {
  time: string
  cpu_percent: number
  load1: number
  memory_total: number
  memory_used: number
  swap_total: number
  swap_used: number
  disk_total: number
  disk_used: number
  net_rx: number
  net_tx: number
  uptime: number
}

export type Service = {
  name: string
  version: string
  path: string
  status: string
  config_path: string
  installed: boolean
}

export type Container = {
  id: string
  names: string[]
  image: string
  state: string
  status: string
}

export type Website = {
  id: string
  server: string
  name: string
  domains: string[]
  listen: string[]
  proxy_target: string
  tls: boolean
  enabled: boolean
  source_path: string
  raw: string
}

export type Certificate = {
  domain: string
  issuer: string
  path: string
  key_path?: string
  expires_at: string
  source: string
  auto_renew: boolean
  days_left: number
}

export type Task = {
  id: string
  kind: string
  status: string
  summary: string
  log: string
  created_at: string
  updated_at: string
}

export type Audit = {
  id: number
  actor: string
  action: string
  resource: string
  detail: string
  remote_ip: string
  created_at: string
}

export type Me = {
  username: string
  must_change: boolean
  csrf_token: string
  totp_enabled: boolean
}

export type AlertRule = {
  id: number
  name: string
  metric: string
  operator: string
  threshold: number
  duration_seconds: number
  silence_seconds: number
  repeat_seconds: number
  enabled: boolean
}
