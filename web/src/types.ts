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
  display_name?: string
  version: string
  path: string
  status: string
  config_path: string
  installed: boolean
  group?: string
  conflicts?: string[]
  install_methods?: string[]
  default_method?: string
  versions?: string[]
  can_install?: boolean
  can_update?: boolean
  block_reason?: string
  note?: string
  deploy?: string
  image?: string
  host_port?: string
  container_port?: string
  docker_name?: string
}

export type RewriteRule = {
  id: string
  name: string
  description: string
  nginx: string
  apache: string
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
  doc_root?: string
  tls: boolean
  has_http: boolean
  has_https: boolean
  enabled: boolean
  source_path: string
  raw?: string
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

export type FileEntry = {
  name: string
  path: string
  is_dir: boolean
  size: number
  mode: string
  mod_time: string
}

export type CronJob = {
  id: string
  schedule: string
  command: string
  raw: string
  enabled: boolean
}

export type SystemInfo = {
  version: string
  channel: string
  web_server?: string
  latest_stable?: string
  stable_url?: string
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
  must_set_entry?: boolean
  entry_path?: string
  decoy_mode?: string
  entry_url?: string
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
