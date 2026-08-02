import React, {useEffect, useMemo, useState} from 'react'
import {createRoot} from 'react-dom/client'
import {
  Activity,
  Box,
  Globe2,
  ServerCog,
  ListChecks,
  ShieldCheck,
  Settings2,
  LogOut,
  RefreshCw,
  Play,
  Square,
  RotateCw,
  Trash2,
  Terminal,
  LockKeyhole,
  Languages,
  BellRing,
  Cpu,
  HardDrive,
  Database,
  Plus,
  FileKey2,
} from 'lucide-react'
import {api, post, setCSRF} from './api'
import {I18n, Lang, translator, useI18n} from './i18n'
import type {AlertRule, Audit, Certificate, Container, Me, Service, Snapshot, Task, Website} from './types'
import './style.css'
import './alerts.css'

type Page = 'dashboard' | 'docker' | 'websites' | 'services' | 'tasks' | 'alerts' | 'panel' | 'security'

function App() {
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [lang, setLang] = useState<Lang>(() => (localStorage.lang || (navigator.language.startsWith('zh') ? 'zh' : 'en')) as Lang)

  useEffect(() => {
    api<Me>('/me')
      .then((v) => {
        setCSRF(v.csrf_token)
        setMe(v)
      })
      .catch(() => setMe(null))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    localStorage.lang = lang
  }, [lang])

  const value = useMemo(() => ({lang, setLang, t: translator(lang)}), [lang])

  if (loading) {
    return (
      <div className="splash">
        <div className="brandmark">
          <Activity />
        </div>
      </div>
    )
  }

  return (
    <I18n.Provider value={value}>
      {me ? <Shell me={me} setMe={setMe} /> : <Login setMe={setMe} />}
    </I18n.Provider>
  )
}

function Login({setMe}: {setMe: (m: Me) => void}) {
  const {t, lang, setLang} = useI18n()
  const [username, setUser] = useState('admin')
  const [password, setPass] = useState('')
  const [totp, setTotp] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const v = await post<Me>('/auth/login', {Username: username, Password: password, TOTP: totp})
      setCSRF(v.csrf_token)
      setMe(v)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="login">
      <form className="login-card" onSubmit={submit}>
        <button type="button" className="lang" onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')}>
          <Languages size={16} />
          {t('language')}
        </button>
        <div className="brand">
          <span className="brandmark">
            <Activity />
          </span>
          AnPanel
        </div>
        <h2>{t('loginTitle')}</h2>
        <p className="subtitle">{t('loginSubtitle')}</p>
        <label>
          {t('username')}
          <input value={username} onChange={(e) => setUser(e.target.value)} autoComplete="username" />
        </label>
        <label>
          {t('password')}
          <input type="password" value={password} onChange={(e) => setPass(e.target.value)} autoComplete="current-password" autoFocus />
        </label>
        <label>
          {t('totp')}
          <input inputMode="numeric" maxLength={6} value={totp} onChange={(e) => setTotp(e.target.value)} placeholder="可选" />
        </label>
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={busy}>
          {busy ? '…' : t('login')}
        </button>
      </form>
    </main>
  )
}

const nav: [Page, React.ElementType, 'dashboard' | 'docker' | 'websites' | 'services' | 'tasks' | 'alerts' | 'panel' | 'security'][] = [
  ['dashboard', Activity, 'dashboard'],
  ['docker', Box, 'docker'],
  ['websites', Globe2, 'websites'],
  ['services', ServerCog, 'services'],
  ['tasks', ListChecks, 'tasks'],
  ['alerts', BellRing, 'alerts'],
  ['panel', Settings2, 'panel'],
  ['security', ShieldCheck, 'security'],
]

function Shell({me, setMe}: {me: Me; setMe: (m: Me | null) => void}) {
  const {t, lang, setLang} = useI18n()
  const [page, setPage] = useState<Page>('dashboard')

  async function logout() {
    try {
      await post('/auth/logout', {})
    } catch {
      /* ignore */
    }
    setMe(null)
  }

  return (
    <div className="shell">
      <aside>
        <div className="brand">
          <span className="brandmark">
            <Activity />
          </span>
          <span>AnPanel</span>
        </div>
        <nav>
          {nav.map(([id, Icon, label]) => (
            <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)}>
              <Icon />
              <span>{t(label)}</span>
            </button>
          ))}
        </nav>
        <div className="aside-bottom">
          <button onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')}>
            <Languages />
            <span>{t('language')}</span>
          </button>
          <button onClick={logout}>
            <LogOut />
            <span>{t('logout')}</span>
          </button>
          <div className="user">
            <span>{me.username[0]?.toUpperCase()}</span>
            <div>
              <strong>{me.username}</strong>
              <small>{t('admin')}</small>
            </div>
          </div>
        </div>
      </aside>
      <main className="content">
        {me.must_change && <FirstLogin me={me} setMe={setMe} />}
        {page === 'dashboard' && <Dashboard goPanel={() => setPage('panel')} />}
        {page === 'docker' && <DockerPage />}
        {page === 'websites' && <Websites />}
        {page === 'services' && <Services />}
        {page === 'tasks' && <Tasks />}
        {page === 'alerts' && <Alerts />}
        {page === 'panel' && <PanelSettings />}
        {page === 'security' && <Security me={me} setMe={setMe} />}
      </main>
    </div>
  )
}

function FirstLogin({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [username, setUser] = useState(me.username)
  const [password, setPass] = useState('')
  const [error, setError] = useState('')

  async function save(e: React.FormEvent) {
    e.preventDefault()
    try {
      await post('/me/change', {Username: username, Password: password})
      setMe({...me, username, must_change: false})
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div className="modal-back">
      <form className="modal" onSubmit={save}>
        <LockKeyhole />
        <h2>{t('firstLogin')}</h2>
        <label>
          {t('newUsername')}
          <input value={username} onChange={(e) => setUser(e.target.value)} />
        </label>
        <label>
          {t('newPassword')}
          <input type="password" value={password} onChange={(e) => setPass(e.target.value)} />
        </label>
        {error && <div className="error">{error}</div>}
        <button className="primary">{t('save')}</button>
      </form>
    </div>
  )
}

type Overview = {
  snapshot: Snapshot
  services: Service[] | null
  containers: Container[] | null
  insecure_http: boolean
}

function Dashboard({goPanel}: {goPanel: () => void}) {
  const {t} = useI18n()
  const [data, setData] = useState<Overview | null>(null)
  const [history, setHistory] = useState<Snapshot[]>([])
  const [error, setError] = useState('')
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    setError('')
    Promise.all([
      api<Overview>('/overview'),
      api<Snapshot[]>('/metrics/history?hours=24').catch(() => [] as Snapshot[]),
    ])
      .then(([ov, hist]) => {
        if (cancelled) return
        setData({
          ...ov,
          services: Array.isArray(ov.services) ? ov.services : [],
          containers: Array.isArray(ov.containers) ? ov.containers : [],
        })
        setHistory(Array.isArray(hist) ? [...hist].reverse() : [])
      })
      .catch((e) => {
        if (!cancelled) setError((e as Error).message || t('loadFailed'))
      })

    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${protocol}://${location.host}/api/v1/ws/metrics`)
    ws.onmessage = (e) => {
      try {
        const m = JSON.parse(e.data) as Snapshot
        setData((v) => (v ? {...v, snapshot: m} : v))
        setHistory((v) => [...v.slice(-239), m])
      } catch {
        /* ignore bad frames */
      }
    }
    return () => {
      cancelled = true
      ws.close()
    }
  }, [tick, t])

  if (error && !data) {
    return (
      <>
        <PageHead title={t('dashboard')} hint={t('overviewHint')} />
        <div className="page-body">
          <div className="error banner">
            {t('loadFailed')}: {error}
            <button className="btn" style={{marginLeft: 12}} onClick={() => setTick((n) => n + 1)}>
              {t('retry')}
            </button>
          </div>
        </div>
      </>
    )
  }

  if (!data) {
    return (
      <>
        <PageHead title={t('dashboard')} hint={t('overviewHint')} />
        <div className="page-body">
          <Loading />
        </div>
      </>
    )
  }

  const m = data.snapshot || ({} as Snapshot)
  const mem = pct(m.memory_used, m.memory_total)
  const disk = pct(m.disk_used, m.disk_total)
  const containers = data.containers || []
  const services = data.services || []
  const running = containers.filter((c) => c.state === 'running').length

  return (
    <>
      <PageHead title={t('dashboard')} hint={t('overviewHint')} />
      <div className="page-body">
        {data.insecure_http && (
          <div className="warning">
            <LockKeyhole />
            <div>
              {t('insecure')}{' '}
              <button type="button" className="link" onClick={goPanel}>
                {t('panel')}
              </button>
            </div>
          </div>
        )}
        <section className="metrics">
          <Metric title={t('cpu')} value={`${num(m.cpu_percent)}%`} detail={`Load ${num(m.load1)}`} color="#20a53a" icon={Cpu} bar={m.cpu_percent} />
          <Metric title={t('memory')} value={`${num(mem)}%`} detail={`${bytes(m.memory_used)} / ${bytes(m.memory_total)}`} color="#409eff" icon={Database} bar={mem} />
          <Metric title={t('disk')} value={`${num(disk)}%`} detail={`${bytes(m.disk_used)} / ${bytes(m.disk_total)}`} color="#e6a23c" icon={HardDrive} bar={disk} />
          <Metric title={t('containers')} value={String(containers.length)} detail={`${running} ${t('running')}`} color="#9b59b6" icon={Box} bar={containers.length ? (running / containers.length) * 100 : 0} />
        </section>
        <section className="grid">
          <div className="panel wide">
            <PanelTitle title={t('performance')} />
            <Spark data={history.map((x) => x.cpu_percent || 0)} empty={t('collecting')} />
          </div>
          <div className="panel">
            <PanelTitle title={t('serviceHealth')} />
            <div className="service-list">
              {services.length === 0 && <div className="empty">{t('noData')}</div>}
              {services.map((s) => (
                <div key={s.name}>
                  <span className={`dot ${s.status === 'active' || s.status === 'available' ? 'ok' : ''}`} />
                  <div>
                    <strong>{s.name}</strong>
                    <small>{s.version || s.status}</small>
                  </div>
                  <em>{s.status}</em>
                </div>
              ))}
            </div>
          </div>
          <RecentTasks />
        </section>
      </div>
    </>
  )
}

function Metric({
  title,
  value,
  detail,
  color,
  icon: Icon,
  bar,
}: {
  title: string
  value: string
  detail: string
  color: string
  icon: React.ElementType
  bar: number
}) {
  const width = Math.max(0, Math.min(100, bar || 0))
  return (
    <div className="metric" style={{'--accent': color} as React.CSSProperties}>
      <div className="metric-top">
        <span>{title}</span>
        <Icon />
      </div>
      <strong>{value}</strong>
      <small>{detail}</small>
      <i className="bar" style={{width: `${width}%`}} />
    </div>
  )
}

function Spark({data, empty}: {data: number[]; empty: string}) {
  if (data.length < 2) return <div className="empty">{empty}</div>
  const max = Math.max(100, ...data)
  const w = 800
  const h = 200
  const points = data.map((v, i) => `${(i / (data.length - 1)) * w},${h - (v / max) * h}`).join(' ')
  return (
    <svg className="chart" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <defs>
        <linearGradient id="fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#20a53a" stopOpacity=".28" />
          <stop offset="1" stopColor="#20a53a" stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={`0,${h} ${points} ${w},${h}`} fill="url(#fill)" />
      <polyline points={points} fill="none" stroke="#20a53a" strokeWidth="2.5" vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

function DockerPage() {
  const {t} = useI18n()
  const [items, setItems] = useState<Container[]>([])
  const [terminal, setTerminal] = useState<Container | null>(null)
  const [error, setError] = useState('')

  const load = () =>
    api<Container[]>('/docker/containers')
      .then((v) => {
        setItems(Array.isArray(v) ? v : [])
        setError('')
      })
      .catch((e) => setError(e.message))

  useEffect(() => {
    void load()
  }, [])

  async function act(c: Container, verb: string) {
    if (verb === 'delete' && prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: `docker.container.${verb}`, resource: c.id, options: {}})
    setTimeout(load, 800)
  }

  return (
    <>
      <PageHead
        title={t('docker')}
        action={
          <button className="btn" onClick={load}>
            <RefreshCw />
            {t('refresh')}
          </button>
        }
      />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        <div className="panel table-panel">
          <table>
            <thead>
              <tr>
                <th>{t('containerCol')}</th>
                <th>{t('image')}</th>
                <th>{t('status')}</th>
                <th>{t('idCol')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr key={c.id}>
                  <td>
                    <div className="resource">
                      <span className={`cube ${c.state === 'running' ? 'online' : ''}`}>
                        <Box />
                      </span>
                      <strong>{c.names?.[0]?.replace('/', '') || c.id.slice(0, 12)}</strong>
                    </div>
                  </td>
                  <td>{c.image}</td>
                  <td>
                    <span className={`pill ${c.state === 'running' ? 'green' : ''}`}>{c.status}</span>
                  </td>
                  <td>
                    <code>{c.id.slice(0, 12)}</code>
                  </td>
                  <td className="actions">
                    {c.state === 'running' && (
                      <button title={t('terminal')} onClick={() => setTerminal(c)}>
                        <Terminal />
                      </button>
                    )}
                    {c.state === 'running' ? (
                      <button title={t('stop')} onClick={() => act(c, 'stop')}>
                        <Square />
                      </button>
                    ) : (
                      <button title={t('start')} onClick={() => act(c, 'start')}>
                        <Play />
                      </button>
                    )}
                    <button title={t('restart')} onClick={() => act(c, 'restart')}>
                      <RotateCw />
                    </button>
                    <button className="danger" title={t('remove')} onClick={() => act(c, 'delete')}>
                      <Trash2 />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!items.length && !error && <div className="empty">{t('noData')}</div>}
        </div>
        {terminal && <ContainerTerminal container={terminal} onClose={() => setTerminal(null)} />}
      </div>
    </>
  )
}

function ContainerTerminal({container, onClose}: {container: Container; onClose: () => void}) {
  const [output, setOutput] = useState('')
  const [command, setCommand] = useState('')
  const [socket, setSocket] = useState<WebSocket | null>(null)

  useEffect(() => {
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${protocol}://${location.host}/api/v1/ws/docker/terminal?id=${encodeURIComponent(container.id)}`)
    ws.binaryType = 'arraybuffer'
    ws.onmessage = (e) => {
      if (typeof e.data === 'string') setOutput((v) => v + e.data)
      else setOutput((v) => v + new TextDecoder().decode(e.data))
    }
    ws.onclose = () => setOutput((v) => v + '\n[connection closed]\n')
    setSocket(ws)
    return () => ws.close()
  }, [container.id])

  function send(e: React.FormEvent) {
    e.preventDefault()
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(command + '\n')
      setOutput((v) => v + '$ ' + command + '\n')
      setCommand('')
    }
  }

  return (
    <div className="modal-back">
      <div className="modal terminal-modal">
        <button className="close" onClick={onClose}>
          ×
        </button>
        <h2>{container.names?.[0]?.replace('/', '') || container.id.slice(0, 12)}</h2>
        <pre>{output || 'Connecting to /bin/sh...'}</pre>
        <form onSubmit={send}>
          <span>$</span>
          <input autoFocus value={command} onChange={(e) => setCommand(e.target.value)} autoComplete="off" />
        </form>
      </div>
    </div>
  )
}

function Websites() {
  const {t} = useI18n()
  const [tab, setTab] = useState<'sites' | 'certs'>('sites')
  const [items, setItems] = useState<Website[]>([])
  const [certs, setCerts] = useState<Certificate[]>([])
  const [edit, setEdit] = useState<Website | null>(null)
  const [wizard, setWizard] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  const loadSites = () =>
    api<Website[]>('/websites')
      .then((v) => {
        setItems(Array.isArray(v) ? v : [])
        setError('')
      })
      .catch((e) => setError(e.message))

  const loadCerts = () =>
    api<Certificate[]>('/certificates')
      .then((v) => {
        setCerts(Array.isArray(v) ? v : [])
        setError('')
      })
      .catch((e) => setError(e.message))

  const load = () => {
    void loadSites()
    void loadCerts()
  }

  useEffect(() => {
    load()
  }, [])

  async function apply() {
    if (!edit) return
    await post('/actions', {kind: 'web.apply', resource: edit.source_path, options: {content: edit.raw}})
    setEdit(null)
    setMessage(t('success'))
    setTimeout(loadSites, 1200)
  }

  async function issueSSL(site: Website) {
    const domain = site.domains?.[0]
    if (!domain) return
    await post('/actions', {
      kind: 'cert.issue',
      resource: domain,
      options: {server: site.server || 'nginx', tool: 'certbot'},
    })
    setMessage(t('issueTask'))
  }

  async function deleteSite(site: Website) {
    const domain = site.domains?.[0]
    if (!domain) return
    if (!site.source_path.includes('anpanel-site-')) {
      setError(t('onlyManaged'))
      return
    }
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: 'web.site.delete', resource: domain, options: {server: site.server || 'nginx'}})
    setMessage(t('success'))
    setTimeout(loadSites, 1200)
  }

  async function renew(domain = '', force = false) {
    await post('/actions', {
      kind: 'cert.renew',
      resource: domain,
      options: {force: force ? 'true' : 'false'},
    })
    setMessage(t('renewTask'))
    setTimeout(loadCerts, 2000)
  }

  return (
    <>
      <PageHead
        title={t('websites')}
        action={
          <div className="toolbar">
            {tab === 'sites' && (
              <button className="primary" onClick={() => setWizard(true)}>
                <Plus size={16} />
                {t('addSite')}
              </button>
            )}
            {tab === 'certs' && (
              <>
                <button className="btn" onClick={() => renew('', false)}>
                  <RefreshCw />
                  {t('renewAll')}
                </button>
              </>
            )}
            <button className="btn" onClick={load}>
              <RefreshCw />
              {t('refresh')}
            </button>
          </div>
        }
      />
      <div className="page-body">
        <div className="tabs">
          <button className={tab === 'sites' ? 'active' : ''} onClick={() => setTab('sites')}>
            {t('siteList')}
          </button>
          <button className={tab === 'certs' ? 'active' : ''} onClick={() => setTab('certs')}>
            {t('sslCerts')}
          </button>
        </div>
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}

        {tab === 'sites' && (
          <>
            <div className="site-grid">
              {items.map((s) => (
                <div className="site-card" key={s.id} style={{cursor: 'default'}}>
                  <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                    <span className={`server-tag ${s.server}`}>{s.server}</span>
                    <span className={`pill ${s.tls ? 'green' : ''}`}>{s.tls ? t('tlsOn') : t('tlsOff')}</span>
                  </div>
                  <h3>{s.domains?.join(', ') || s.name}</h3>
                  <p>
                    <Globe2 />
                    {s.listen?.join(', ') || '-'}
                  </p>
                  <p>
                    <Terminal />
                    {s.proxy_target || t('staticSite')}
                  </p>
                  <small>{s.source_path}</small>
                  <div className="meta-row card-actions" style={{marginTop: 12}}>
                    <button className="btn" onClick={() => setEdit(s)}>
                      {t('editConfig')}
                    </button>
                    {s.domains?.[0] && !s.tls && (
                      <button className="btn" onClick={() => issueSSL(s)}>
                        <FileKey2 size={14} />
                        {t('issueSSL')}
                      </button>
                    )}
                    {s.source_path.includes('anpanel-site-') && (
                      <button className="btn" onClick={() => deleteSite(s)}>
                        {t('deleteSite')}
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
            {!items.length && !error && <div className="panel empty">{t('noData')}</div>}
          </>
        )}

        {tab === 'certs' && (
          <div className="panel table-panel">
            <table>
              <thead>
                <tr>
                  <th>{t('certDomain')}</th>
                  <th>{t('issuer')}</th>
                  <th>{t('expires')}</th>
                  <th>{t('daysLeft')}</th>
                  <th>{t('certSource')}</th>
                  <th>{t('autoRenew')}</th>
                  <th>{t('status')}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {certs.map((c) => {
                  const status = c.days_left < 0 ? 'expired' : c.days_left <= 14 ? 'expiring' : 'valid'
                  const dayClass = status === 'expired' ? 'days-bad' : status === 'expiring' ? 'days-warn' : 'days-ok'
                  return (
                    <tr key={c.domain + c.path}>
                      <td>
                        <strong>{c.domain}</strong>
                      </td>
                      <td>{c.issuer || '-'}</td>
                      <td>{c.expires_at ? new Date(c.expires_at).toLocaleString() : '-'}</td>
                      <td className={dayClass}>{c.days_left}</td>
                      <td>
                        <span className="pill">{c.source}</span>
                      </td>
                      <td>{c.auto_renew ? t('yes') : t('no')}</td>
                      <td>
                        <span className={`pill ${status === 'valid' ? 'green' : ''}`}>
                          {status === 'expired' ? t('expired') : status === 'expiring' ? t('expiring') : t('valid')}
                        </span>
                      </td>
                      <td className="actions">
                        <button title={t('renew')} onClick={() => renew(c.domain, false)}>
                          <RefreshCw />
                        </button>
                        <button title={t('renewForce')} onClick={() => renew(c.domain, true)}>
                          <RotateCw />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
            {!certs.length && !error && <div className="empty">{t('noData')}</div>}
          </div>
        )}

        {edit && (
          <div className="modal-back">
            <div className="modal editor">
              <button className="close" onClick={() => setEdit(null)}>
                ×
              </button>
              <h2>{edit.domains?.[0] || edit.name}</h2>
              <small>
                {t('source')}: {edit.source_path}
              </small>
              <textarea spellCheck={false} value={edit.raw} onChange={(e) => setEdit({...edit, raw: e.target.value})} />
              <button className="primary" onClick={apply}>
                {t('apply')}
              </button>
            </div>
          </div>
        )}

        {wizard && (
          <SiteWizard
            onClose={() => setWizard(false)}
            onCreated={() => {
              setWizard(false)
              setMessage(t('siteCreated'))
              setTimeout(load, 1500)
            }}
          />
        )}
      </div>
    </>
  )
}

function SiteWizard({onClose, onCreated}: {onClose: () => void; onCreated: () => void}) {
  const {t} = useI18n()
  const [step, setStep] = useState(0)
  const [siteType, setSiteType] = useState<'proxy' | 'static'>('proxy')
  const [domain, setDomain] = useState('')
  const [server, setServer] = useState('nginx')
  const [root, setRoot] = useState('')
  const [proxyPass, setProxyPass] = useState('http://127.0.0.1:3000')
  const [enableSSL, setEnableSSL] = useState(false)
  const [tool, setTool] = useState('certbot')
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    setBusy(true)
    setError('')
    try {
      const options: Record<string, string> = {
        domain: domain.trim(),
        server,
        site_type: siteType,
        enable_ssl: enableSSL ? 'true' : 'false',
        tool,
        email,
      }
      if (siteType === 'static') {
        options.root = root.trim()
      } else {
        options.proxy_pass = proxyPass.trim()
      }
      await post('/actions', {kind: 'web.site.create', resource: domain.trim(), options})
      onCreated()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const canNext =
    step === 0 ||
    (step === 1 && domain.trim().includes('.') && (siteType === 'proxy' ? proxyPass.trim().startsWith('http') : true))

  return (
    <div className="modal-back">
      <div className="modal wizard">
        <button className="close" onClick={onClose}>
          ×
        </button>
        <h2>{t('siteWizard')}</h2>
        <div className="wizard-steps">
          <span className={step === 0 ? 'active' : step > 0 ? 'done' : ''}>{t('wizardStepType')}</span>
          <span className={step === 1 ? 'active' : step > 1 ? 'done' : ''}>{t('wizardStepBasic')}</span>
          <span className={step === 2 ? 'active' : ''}>{t('wizardStepSSL')}</span>
        </div>

        {step === 0 && (
          <div className="type-cards">
            <button type="button" className={`type-card ${siteType === 'proxy' ? 'selected' : ''}`} onClick={() => setSiteType('proxy')}>
              <strong>{t('siteTypeProxy')}</strong>
              <span>{t('siteTypeProxyHint')}</span>
            </button>
            <button type="button" className={`type-card ${siteType === 'static' ? 'selected' : ''}`} onClick={() => setSiteType('static')}>
              <strong>{t('siteTypeStatic')}</strong>
              <span>{t('siteTypeStaticHint')}</span>
            </button>
          </div>
        )}

        {step === 1 && (
          <>
            <label>
              {t('domainLabel')}
              <input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="example.com" autoFocus />
            </label>
            <small style={{color: 'var(--muted)'}}>{t('domainHintSite')}</small>
            <label>
              {t('webServer')}
              <select value={server} onChange={(e) => setServer(e.target.value)}>
                <option value="nginx">Nginx</option>
                <option value="apache">Apache</option>
              </select>
            </label>
            {siteType === 'proxy' ? (
              <label>
                {t('proxyPass')}
                <input value={proxyPass} onChange={(e) => setProxyPass(e.target.value)} placeholder="http://127.0.0.1:3000" />
                <small style={{color: 'var(--muted)', fontWeight: 400}}>{t('proxyPassHint')}</small>
              </label>
            ) : (
              <label>
                {t('docRoot')}
                <input value={root} onChange={(e) => setRoot(e.target.value)} placeholder={`/var/www/${domain || 'example.com'}`} />
                <small style={{color: 'var(--muted)', fontWeight: 400}}>{t('docRootHint')}</small>
              </label>
            )}
          </>
        )}

        {step === 2 && (
          <>
            <label className="check-row">
              <input type="checkbox" checked={enableSSL} onChange={(e) => setEnableSSL(e.target.checked)} />
              {t('enableSSL')}
            </label>
            {enableSSL && (
              <>
                <label>
                  {t('acmeTool')}
                  <select value={tool} onChange={(e) => setTool(e.target.value)}>
                    <option value="certbot">certbot</option>
                    <option value="acme.sh">acme.sh</option>
                  </select>
                </label>
                <label>
                  {t('email')}
                  <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin@example.com" />
                </label>
                <div className="steps" style={{marginTop: 8}}>
                  <ol>
                    <li>{t('step1')}</li>
                    <li>{t('step2')}</li>
                    <li>{t('step3')}</li>
                  </ol>
                </div>
              </>
            )}
          </>
        )}

        {error && <div className="error">{error}</div>}
        <div className="card-actions">
          <button type="button" className="btn" onClick={onClose}>
            {t('cancel')}
          </button>
          {step > 0 && (
            <button type="button" className="btn" onClick={() => setStep((s) => s - 1)}>
              {t('prev')}
            </button>
          )}
          {step < 2 && (
            <button type="button" className="primary" disabled={!canNext} onClick={() => setStep((s) => s + 1)}>
              {t('next')}
            </button>
          )}
          {step === 2 && (
            <button type="button" className="primary" disabled={busy || !domain.trim()} onClick={submit}>
              {busy ? '…' : t('createSite')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function Services() {
  const {t} = useI18n()
  const [items, setItems] = useState<Service[]>([])
  const [error, setError] = useState('')

  const load = () =>
    api<Service[]>('/services')
      .then((v) => {
        setItems(Array.isArray(v) ? v : [])
        setError('')
      })
      .catch((e) => setError(e.message))

  useEffect(() => {
    void load()
  }, [])

  async function act(s: Service, verb: string) {
    const resource = s.name === 'apache' ? (s.path.includes('apache2') ? 'apache2' : 'httpd') : s.name
    await post('/actions', {kind: `service.${verb}`, resource, options: {}})
    setTimeout(load, 600)
  }

  async function install(s: Service) {
    await post('/actions', {kind: 'package.install', resource: s.name, options: {}})
  }

  return (
    <>
      <PageHead
        title={t('services')}
        action={
          <button className="btn" onClick={load}>
            <RefreshCw />
            {t('refresh')}
          </button>
        }
      />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        <div className="service-cards">
          {items.map((s) => (
            <div className="panel service-card" key={s.name}>
              <div className="resource">
                <span className="cube">
                  <ServerCog />
                </span>
                <div>
                  <h3>{s.name}</h3>
                  <small>{s.path || t('missing')}</small>
                </div>
              </div>
              <span className={`pill ${s.status === 'active' || s.status === 'available' ? 'green' : ''}`}>{s.status}</span>
              {s.installed && ['nginx', 'apache', 'docker'].includes(s.name) && (
                <div className="card-actions">
                  <button className="btn" onClick={() => act(s, s.status === 'active' ? 'stop' : 'start')}>
                    {s.status === 'active' ? t('stop') : t('start')}
                  </button>
                  <button className="btn" onClick={() => act(s, 'restart')}>
                    {t('restart')}
                  </button>
                </div>
              )}
              {!s.installed && ['nginx', 'apache', 'docker', 'certbot'].includes(s.name) && (
                <div className="card-actions">
                  <button className="btn" onClick={() => install(s)}>
                    {t('install')}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
        {!items.length && !error && <div className="panel empty">{t('noData')}</div>}
      </div>
    </>
  )
}

function Tasks() {
  const {t} = useI18n()
  const [tasks, setTasks] = useState<Task[]>([])
  const [audits, setAudits] = useState<Audit[]>([])

  const load = () => {
    api<Task[]>('/tasks')
      .then((v) => setTasks(Array.isArray(v) ? v : []))
      .catch(() => {})
    api<Audit[]>('/audits')
      .then((v) => setAudits(Array.isArray(v) ? v : []))
      .catch(() => {})
  }

  useEffect(() => {
    load()
    const i = setInterval(load, 3000)
    return () => clearInterval(i)
  }, [])

  return (
    <>
      <PageHead
        title={t('tasks')}
        action={
          <button className="btn" onClick={load}>
            <RefreshCw />
            {t('refresh')}
          </button>
        }
      />
      <div className="page-body">
        <section className="split">
          <div className="panel">
            <PanelTitle title={t('tasksTitle')} />
            <div className="timeline">
              {tasks.map((x) => (
                <div key={x.id}>
                  <span className={`task-dot ${x.status}`} />
                  <div>
                    <strong>{x.summary}</strong>
                    <small>
                      {new Date(x.created_at).toLocaleString()} / {x.status}
                    </small>
                    {x.log && <pre>{x.log}</pre>}
                  </div>
                </div>
              ))}
              {!tasks.length && <div className="empty">{t('noData')}</div>}
            </div>
          </div>
          <div className="panel">
            <PanelTitle title={t('auditTitle')} />
            <div className="timeline">
              {audits.map((x) => (
                <div key={x.id}>
                  <span className="task-dot succeeded" />
                  <div>
                    <strong>{x.action}</strong>
                    <small>
                      {x.actor} / {x.resource}
                    </small>
                    <p>{x.detail}</p>
                  </div>
                </div>
              ))}
              {!audits.length && <div className="empty">{t('noData')}</div>}
            </div>
          </div>
        </section>
      </div>
    </>
  )
}

function RecentTasks() {
  const {t} = useI18n()
  const [items, setItems] = useState<Task[]>([])

  useEffect(() => {
    api<Task[]>('/tasks')
      .then((v) => setItems(Array.isArray(v) ? v : []))
      .catch(() => setItems([]))
  }, [])

  return (
    <div className="panel">
      <PanelTitle title={t('recentTasks')} />
      <div className="timeline compact">
        {items.slice(0, 5).map((x) => (
          <div key={x.id}>
            <span className={`task-dot ${x.status}`} />
            <div>
              <strong>{x.kind}</strong>
              <small>{x.status}</small>
            </div>
          </div>
        ))}
        {!items.length && <div className="empty">{t('noData')}</div>}
      </div>
    </div>
  )
}

function Alerts() {
  const {t} = useI18n()
  const [rules, setRules] = useState<AlertRule[]>([])
  const [name, setName] = useState('High CPU')
  const [metric, setMetric] = useState('cpu')
  const [threshold, setThreshold] = useState(90)
  const [duration, setDuration] = useState(300)
  const [webhook, setWebhook] = useState('')
  const [message, setMessage] = useState('')

  const load = () =>
    api<AlertRule[]>('/alerts/rules')
      .then((v) => setRules(Array.isArray(v) ? v : []))
      .catch(() => {})

  useEffect(() => {
    void load()
  }, [])

  async function save() {
    await post('/alerts/rules/update', {
      operation: 'save',
      rule: {
        id: 0,
        name,
        metric,
        operator: 'gt',
        threshold,
        duration_seconds: duration,
        silence_seconds: 300,
        repeat_seconds: 3600,
        enabled: true,
      },
    })
    await load()
  }

  async function remove(rule: AlertRule) {
    await post('/alerts/rules/update', {operation: 'delete', rule})
    await load()
  }

  async function saveNotify() {
    await post('/actions', {
      kind: 'notification.configure',
      resource: 'notifications',
      options: {json: JSON.stringify({webhook_url: webhook})},
    })
    setMessage(t('notifyTask'))
  }

  return (
    <>
      <PageHead title={t('alerts')} />
      <div className="page-body">
        <section className="split">
          <div className="panel alert-form">
            <PanelTitle title={t('newRule')} />
            <label>
              {t('ruleName')}
              <input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              {t('metric')}
              <select value={metric} onChange={(e) => setMetric(e.target.value)}>
                <option value="cpu">CPU %</option>
                <option value="memory">Memory %</option>
                <option value="disk">Disk %</option>
                <option value="load">Load</option>
              </select>
            </label>
            <label>
              {t('threshold')}
              <input type="number" value={threshold} onChange={(e) => setThreshold(+e.target.value)} />
            </label>
            <label>
              {t('durationSec')}
              <input type="number" value={duration} onChange={(e) => setDuration(+e.target.value)} />
            </label>
            <button className="primary" onClick={save}>
              {t('save')}
            </button>
          </div>
          <div className="panel alert-form">
            <PanelTitle title={t('notify')} />
            <label>
              Webhook URL
              <input value={webhook} onChange={(e) => setWebhook(e.target.value)} placeholder="https://example.com/hook" />
            </label>
            <button className="primary" onClick={saveNotify}>
              {t('save')}
            </button>
            {message && <div className="success">{message}</div>}
          </div>
        </section>
        <div className="panel table-panel alert-table">
          <table>
            <thead>
              <tr>
                <th>{t('ruleName')}</th>
                <th>{t('metric')}</th>
                <th>{t('threshold')}</th>
                <th>{t('durationSec')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id}>
                  <td>{r.name}</td>
                  <td>{r.metric}</td>
                  <td>
                    {r.operator} {r.threshold}
                  </td>
                  <td>{r.duration_seconds}s</td>
                  <td className="actions">
                    <button className="danger" onClick={() => remove(r)}>
                      <Trash2 />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!rules.length && <div className="empty">{t('noData')}</div>}
        </div>
      </div>
    </>
  )
}

function PanelSettings() {
  const {t} = useI18n()
  const [domain, setDomain] = useState('')
  const [server, setServer] = useState('nginx')
  const [tool, setTool] = useState('certbot')
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function bind() {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await post('/actions', {kind: 'panel.bind_domain', resource: domain, options: {domain, server, tool, email}})
      setMessage(t('domainTask'))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function unbind() {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await post('/actions', {kind: 'panel.unbind_domain', resource: 'panel', options: {server}})
      setMessage(t('unbindTask'))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <PageHead title={t('panel')} hint={t('domainHttps')} />
      <div className="page-body stack">
        <div className="panel domain-card">
          <Globe2 />
          <div>
            <h2>{t('domainHttps')}</h2>
            <p>{t('domainHint')}</p>
          </div>
          <div className="steps">
            <h4>{t('domainSteps')}</h4>
            <ol>
              <li>{t('step1')}</li>
              <li>{t('step2')}</li>
              <li>{t('step3')}</li>
              <li>{t('step4')}</li>
            </ol>
          </div>
          <div className="domain-form">
            <label>
              Domain
              <input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder={t('domainPlaceholder')} />
            </label>
            <label>
              {t('webServer')}
              <select value={server} onChange={(e) => setServer(e.target.value)}>
                <option value="nginx">Nginx</option>
                <option value="apache">Apache</option>
              </select>
            </label>
            <label>
              {t('acmeTool')}
              <select value={tool} onChange={(e) => setTool(e.target.value)}>
                <option value="certbot">certbot</option>
                <option value="acme.sh">acme.sh</option>
              </select>
            </label>
            <label>
              {t('email')}
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin@example.com" />
            </label>
            <div className="card-actions">
              <button className="primary" disabled={busy || !domain.trim()} onClick={bind}>
                {t('bindDomain')}
              </button>
              <button className="btn" disabled={busy} onClick={unbind}>
                {t('unbindDomain')}
              </button>
            </div>
          </div>
          {message && <div className="success">{message}</div>}
          {error && <div className="error banner">{error}</div>}
        </div>
      </div>
    </>
  )
}

function Security({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [secret, setSecret] = useState<{secret: string; uri: string} | null>(null)
  const [code, setCode] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function setup() {
    setError('')
    try {
      setSecret(await post('/me/totp/setup', {}))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function enable() {
    setError('')
    try {
      await post('/me/totp/enable', {Code: code})
      setMe({...me, totp_enabled: true})
      setMessage(t('totpEnabled'))
      setSecret(null)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <>
      <PageHead title={t('security')} />
      <div className="page-body">
        <div className="panel security-card">
          <ShieldCheck />
          <div>
            <h2>{t('totpTitle')}</h2>
            <p>{me.totp_enabled ? t('totpOn') : t('totpOff')}</p>
          </div>
          {!me.totp_enabled && !secret && (
            <button className="primary" onClick={setup}>
              {t('setupTOTP')}
            </button>
          )}
          {secret && (
            <div className="totp-setup">
              <code>{secret.secret}</code>
              <p>{t('totpSecretHint')}</p>
              <input value={code} onChange={(e) => setCode(e.target.value)} maxLength={6} />
              <button className="primary" onClick={enable}>
                {t('totpVerify')}
              </button>
            </div>
          )}
          {message && <div className="success">{message}</div>}
          {error && <div className="error">{error}</div>}
        </div>
      </div>
    </>
  )
}

function PageHead({title, hint, action}: {title: string; hint?: string; action?: React.ReactNode}) {
  return (
    <header className="page-head">
      <div>
        <h1>{title}</h1>
        {hint && <p className="hint">{hint}</p>}
      </div>
      {action && <div className="actions-row">{action}</div>}
    </header>
  )
}

function PanelTitle({title}: {title: string}) {
  return (
    <div className="panel-title">
      <h3>{title}</h3>
      <span>LIVE</span>
    </div>
  )
}

function Loading() {
  const {t} = useI18n()
  return (
    <div className="loading">
      <RefreshCw /> {t('loading')}
    </div>
  )
}

function pct(a = 0, b = 0) {
  return b ? (a / b) * 100 : 0
}
function num(v = 0) {
  return Number.isFinite(v) ? v.toFixed(1) : '0.0'
}
function bytes(v = 0) {
  if (!v) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(v) / Math.log(1024)), 4)
  return `${(v / 1024 ** i).toFixed(1)} ${u[i]}`
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
