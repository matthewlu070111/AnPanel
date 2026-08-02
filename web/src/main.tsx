import React, {useEffect, useMemo, useState} from 'react'
import {createRoot} from 'react-dom/client'
import {
  Activity, Box, Globe2, ServerCog, ListChecks, Settings2, LogOut, RefreshCw,
  Play, Square, RotateCw, Trash2, Terminal, LockKeyhole, Languages, BellRing,
  Cpu, HardDrive, Database, Plus, FileKey2, FolderOpen, FileText, ChevronUp,
  Pencil, Download,
} from 'lucide-react'
import {api, post, setCSRF} from './api'
import {I18n, Lang, translator, useI18n} from './i18n'
import type {
  AlertRule, Audit, Certificate, Container, CronJob, FileEntry, Me, RewriteRule, Service,
  Snapshot, SystemInfo, Task, Website,
} from './types'
import './style.css'
import './alerts.css'

type Page = 'dashboard' | 'docker' | 'websites' | 'files' | 'services' | 'tasks' | 'alerts' | 'settings'

function App() {
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [lang, setLang] = useState<Lang>(() => (localStorage.lang || (navigator.language.startsWith('zh') ? 'zh' : 'en')) as Lang)

  useEffect(() => {
    api<Me>('/me').then(v => { setCSRF(v.csrf_token); setMe(v) }).catch(() => setMe(null)).finally(() => setLoading(false))
  }, [])
  useEffect(() => { localStorage.lang = lang }, [lang])
  const value = useMemo(() => ({lang, setLang, t: translator(lang)}), [lang])
  if (loading) return <div className="splash"><div className="brandmark"><Activity /></div></div>
  return <I18n.Provider value={value}>{me ? <Shell me={me} setMe={setMe} /> : <Login setMe={setMe} />}</I18n.Provider>
}

function Login({setMe}: {setMe: (m: Me) => void}) {
  const {t, lang, setLang} = useI18n()
  const [username, setUser] = useState('admin'), [password, setPass] = useState(''), [totp, setTotp] = useState('')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  async function submit(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('')
    try {
      const v = await post<Me>('/auth/login', {Username: username, Password: password, TOTP: totp})
      setCSRF(v.csrf_token); setMe(v)
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  return (
    <main className="login">
      <form className="login-card" onSubmit={submit}>
        <button type="button" className="lang" onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')}><Languages size={16} />{t('language')}</button>
        <div className="brand"><span className="brandmark"><Activity /></span>AnPanel</div>
        <h2>{t('loginTitle')}</h2>
        <p className="subtitle">{t('loginSubtitle')}</p>
        <label>{t('username')}<input value={username} onChange={e => setUser(e.target.value)} autoComplete="username" /></label>
        <label>{t('password')}<input type="password" value={password} onChange={e => setPass(e.target.value)} autoComplete="current-password" autoFocus /></label>
        <label>{t('totp')}<input inputMode="numeric" maxLength={6} value={totp} onChange={e => setTotp(e.target.value)} /></label>
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={busy}>{busy ? '…' : t('login')}</button>
      </form>
    </main>
  )
}

const nav: [Page, React.ElementType, 'dashboard' | 'docker' | 'websites' | 'files' | 'services' | 'tasks' | 'alerts' | 'settings'][] = [
  ['dashboard', Activity, 'dashboard'],
  ['docker', Box, 'docker'],
  ['websites', Globe2, 'websites'],
  ['files', FolderOpen, 'files'],
  ['services', ServerCog, 'services'],
  ['tasks', ListChecks, 'tasks'],
  ['alerts', BellRing, 'alerts'],
  ['settings', Settings2, 'settings'],
]

function Shell({me, setMe}: {me: Me; setMe: (m: Me | null) => void}) {
  const {t, lang, setLang} = useI18n()
  const [page, setPage] = useState<Page>('dashboard')
  async function logout() { try { await post('/auth/logout', {}) } catch { /* */ } setMe(null) }
  return (
    <div className="shell">
      <aside>
        <div className="brand"><span className="brandmark"><Activity /></span><span>AnPanel</span></div>
        <nav>
          {nav.map(([id, Icon, label]) => (
            <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)}>
              <Icon /><span>{t(label)}</span>
            </button>
          ))}
        </nav>
        <div className="aside-bottom">
          <button onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')}><Languages /><span>{t('language')}</span></button>
          <button onClick={logout}><LogOut /><span>{t('logout')}</span></button>
          <div className="user"><span>{me.username[0]?.toUpperCase()}</span><div><strong>{me.username}</strong><small>{t('admin')}</small></div></div>
        </div>
      </aside>
      <main className="content">
        {me.must_change && <FirstLogin me={me} setMe={setMe} />}
        {page === 'dashboard' && <Dashboard goSettings={() => setPage('settings')} />}
        {page === 'docker' && <DockerPage />}
        {page === 'websites' && <Websites />}
        {page === 'files' && <FilesPage />}
        {page === 'services' && <Services />}
        {page === 'tasks' && <Tasks />}
        {page === 'alerts' && <Alerts />}
        {page === 'settings' && <Settings me={me} setMe={setMe} />}
      </main>
    </div>
  )
}

function FirstLogin({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [username, setUser] = useState(me.username), [password, setPass] = useState(''), [error, setError] = useState('')
  async function save(e: React.FormEvent) {
    e.preventDefault()
    try { await post('/me/change', {Username: username, Password: password}); setMe({...me, username, must_change: false}) }
    catch (err) { setError((err as Error).message) }
  }
  return (
    <div className="modal-back">
      <form className="modal" onSubmit={save}>
        <LockKeyhole /><h2>{t('firstLogin')}</h2>
        <label>{t('newUsername')}<input value={username} onChange={e => setUser(e.target.value)} /></label>
        <label>{t('newPassword')}<input type="password" value={password} onChange={e => setPass(e.target.value)} /></label>
        {error && <div className="error">{error}</div>}
        <button className="primary">{t('save')}</button>
      </form>
    </div>
  )
}

/* —— Dashboard —— */
type Overview = {snapshot: Snapshot; services: Service[] | null; containers: Container[] | null; insecure_http: boolean}
function Dashboard({goSettings}: {goSettings: () => void}) {
  const {t} = useI18n()
  const [data, setData] = useState<Overview | null>(null)
  const [history, setHistory] = useState<Snapshot[]>([])
  const [error, setError] = useState(''), [tick, setTick] = useState(0)
  useEffect(() => {
    let cancelled = false
    Promise.all([api<Overview>('/overview'), api<Snapshot[]>('/metrics/history?hours=24').catch(() => [] as Snapshot[])])
      .then(([ov, hist]) => {
        if (cancelled) return
        setData({...ov, services: Array.isArray(ov.services) ? ov.services : [], containers: Array.isArray(ov.containers) ? ov.containers : []})
        setHistory(Array.isArray(hist) ? [...hist].reverse() : [])
      })
      .catch(e => { if (!cancelled) setError((e as Error).message) })
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/metrics`)
    ws.onmessage = e => {
      try {
        const m = JSON.parse(e.data) as Snapshot
        setData(v => v ? {...v, snapshot: m} : v)
        setHistory(v => [...v.slice(-239), m])
      } catch { /* */ }
    }
    return () => { cancelled = true; ws.close() }
  }, [tick])
  if (error && !data) return <><PageHead title={t('dashboard')} /><div className="page-body"><div className="error banner">{error} <button className="btn" onClick={() => setTick(n => n + 1)}>{t('retry')}</button></div></div></>
  if (!data) return <><PageHead title={t('dashboard')} /><div className="page-body"><Loading /></div></>
  const m = data.snapshot || ({} as Snapshot)
  const mem = pct(m.memory_used, m.memory_total), disk = pct(m.disk_used, m.disk_total)
  const containers = data.containers || [], services = data.services || []
  const running = containers.filter(c => c.state === 'running').length
  return (
    <>
      <PageHead title={t('dashboard')} hint={t('overviewHint')} />
      <div className="page-body">
        {data.insecure_http && <div className="warning"><LockKeyhole /><div>{t('insecure')} <button type="button" className="link" onClick={goSettings}>{t('settings')}</button></div></div>}
        <section className="metrics">
          <Metric title={t('cpu')} value={`${num(m.cpu_percent)}%`} detail={`Load ${num(m.load1)}`} color="#20a53a" icon={Cpu} bar={m.cpu_percent} />
          <Metric title={t('memory')} value={`${num(mem)}%`} detail={`${bytes(m.memory_used)} / ${bytes(m.memory_total)}`} color="#409eff" icon={Database} bar={mem} />
          <Metric title={t('disk')} value={`${num(disk)}%`} detail={`${bytes(m.disk_used)} / ${bytes(m.disk_total)}`} color="#e6a23c" icon={HardDrive} bar={disk} />
          <Metric title={t('containers')} value={String(containers.length)} detail={`${running} ${t('running')}`} color="#9b59b6" icon={Box} bar={containers.length ? (running / containers.length) * 100 : 0} />
        </section>
        <section className="grid">
          <div className="panel wide"><PanelTitle title={t('performance')} /><Spark data={history.map(x => x.cpu_percent || 0)} empty={t('collecting')} /></div>
          <div className="panel"><PanelTitle title={t('serviceHealth')} />
            <div className="service-list">
              {services.map(s => <div key={s.name}><span className={`dot ${s.status === 'active' || s.status === 'available' ? 'ok' : ''}`} /><div><strong>{s.name}</strong><small>{s.version || s.status}</small></div><em>{s.status}</em></div>)}
              {!services.length && <div className="empty">{t('noData')}</div>}
            </div>
          </div>
          <RecentTasks />
        </section>
      </div>
    </>
  )
}
function Metric({title, value, detail, color, icon: Icon, bar}: {title: string; value: string; detail: string; color: string; icon: React.ElementType; bar: number}) {
  return (
    <div className="metric" style={{'--accent': color} as React.CSSProperties}>
      <div className="metric-top"><span>{title}</span><Icon /></div>
      <strong>{value}</strong><small>{detail}</small>
      <i className="bar" style={{width: `${Math.max(0, Math.min(100, bar || 0))}%`}} />
    </div>
  )
}
function Spark({data, empty}: {data: number[]; empty: string}) {
  if (data.length < 2) return <div className="empty">{empty}</div>
  const max = Math.max(100, ...data), w = 800, h = 200
  const points = data.map((v, i) => `${(i / (data.length - 1)) * w},${h - (v / max) * h}`).join(' ')
  return (
    <svg className="chart" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <defs><linearGradient id="fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#20a53a" stopOpacity=".28" /><stop offset="1" stopColor="#20a53a" stopOpacity="0" /></linearGradient></defs>
      <polygon points={`0,${h} ${points} ${w},${h}`} fill="url(#fill)" />
      <polyline points={points} fill="none" stroke="#20a53a" strokeWidth="2.5" vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

/* —— Docker —— */
function DockerPage() {
  const {t} = useI18n()
  const [items, setItems] = useState<Container[]>([]), [terminal, setTerminal] = useState<Container | null>(null)
  const [error, setError] = useState(''), [deploy, setDeploy] = useState(false), [message, setMessage] = useState('')
  const load = () => api<Container[]>('/docker/containers').then(v => { setItems(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))
  useEffect(() => { void load() }, [])
  async function act(c: Container, verb: string) {
    if (verb === 'delete' && prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: `docker.container.${verb}`, resource: c.id, options: {}})
    setTimeout(load, 800)
  }
  return (
    <>
      <PageHead title={t('docker')} action={<div className="toolbar">
        <button className="primary" onClick={() => setDeploy(true)}><Plus size={16} />{t('deployDocker')}</button>
        <button className="btn" onClick={load}><RefreshCw />{t('refresh')}</button>
      </div>} />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        <div className="panel table-panel">
          <table>
            <thead><tr><th>{t('containerCol')}</th><th>{t('image')}</th><th>{t('status')}</th><th>{t('idCol')}</th><th /></tr></thead>
            <tbody>
              {items.map(c => (
                <tr key={c.id}>
                  <td><div className="resource"><span className={`cube ${c.state === 'running' ? 'online' : ''}`}><Box /></span><strong>{c.names?.[0]?.replace('/', '') || c.id.slice(0, 12)}</strong></div></td>
                  <td>{c.image}</td>
                  <td><span className={`pill ${c.state === 'running' ? 'green' : ''}`}>{c.status}</span></td>
                  <td><code>{c.id.slice(0, 12)}</code></td>
                  <td className="actions">
                    {c.state === 'running' && <button title={t('terminal')} onClick={() => setTerminal(c)}><Terminal /></button>}
                    {c.state === 'running' ? <button title={t('stop')} onClick={() => act(c, 'stop')}><Square /></button> : <button title={t('start')} onClick={() => act(c, 'start')}><Play /></button>}
                    <button title={t('restart')} onClick={() => act(c, 'restart')}><RotateCw /></button>
                    <button className="danger" title={t('remove')} onClick={() => act(c, 'delete')}><Trash2 /></button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!items.length && !error && <div className="empty">{t('noData')}</div>}
        </div>
        {terminal && <ContainerTerminal container={terminal} onClose={() => setTerminal(null)} />}
        {deploy && <DeployWizard onClose={() => setDeploy(false)} onDone={() => { setDeploy(false); setMessage(t('deployTask')); setTimeout(load, 1500) }} />}
      </div>
    </>
  )
}

function DeployWizard({onClose, onDone}: {onClose: () => void; onDone: () => void}) {
  const {t} = useI18n()
  const [image, setImage] = useState('nginx:alpine'), [name, setName] = useState('')
  const [hostPort, setHostPort] = useState('8080'), [containerPort, setContainerPort] = useState('80')
  const [env, setEnv] = useState(''), [domain, setDomain] = useState(''), [enableSSL, setEnableSSL] = useState(false)
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  async function submit() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'docker.deploy', resource: image, options: {
        image, name, host_port: hostPort, container_port: containerPort, env, domain,
        enable_ssl: enableSSL ? 'true' : 'false', tool: 'certbot',
      }})
      onDone()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  return (
    <div className="modal-back"><div className="modal wizard">
      <button className="close" onClick={onClose}>×</button>
      <h2>{t('deployTitle')}</h2>
      <p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('deployHint')}</p>
      <label>{t('image')}<input value={image} onChange={e => setImage(e.target.value)} /></label>
      <label>{t('containerName')}<input value={name} onChange={e => setName(e.target.value)} placeholder="auto" /></label>
      <div className="split" style={{gap: 12}}>
        <label>{t('hostPort')}<input value={hostPort} onChange={e => setHostPort(e.target.value)} /></label>
        <label>{t('containerPort')}<input value={containerPort} onChange={e => setContainerPort(e.target.value)} /></label>
      </div>
      <label>{t('envVars')}<input value={env} onChange={e => setEnv(e.target.value)} placeholder={t('envHint')} /></label>
      <label>{t('bindDomainOpt')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="app.example.com" /></label>
      {domain && <label className="check-row"><input type="checkbox" checked={enableSSL} onChange={e => setEnableSSL(e.target.checked)} />{t('enableSSL')}</label>}
      {error && <div className="error">{error}</div>}
      <div className="card-actions">
        <button className="btn" onClick={onClose}>{t('cancel')}</button>
        <button className="primary" disabled={busy || !image.trim()} onClick={submit}>{busy ? '…' : t('deploy')}</button>
      </div>
    </div></div>
  )
}

function ContainerTerminal({container, onClose}: {container: Container; onClose: () => void}) {
  const [output, setOutput] = useState(''), [command, setCommand] = useState(''), [socket, setSocket] = useState<WebSocket | null>(null)
  useEffect(() => {
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/docker/terminal?id=${encodeURIComponent(container.id)}`)
    ws.binaryType = 'arraybuffer'
    ws.onmessage = e => { if (typeof e.data === 'string') setOutput(v => v + e.data); else setOutput(v => v + new TextDecoder().decode(e.data)) }
    ws.onclose = () => setOutput(v => v + '\n[connection closed]\n')
    setSocket(ws)
    return () => ws.close()
  }, [container.id])
  function send(e: React.FormEvent) {
    e.preventDefault()
    if (socket?.readyState === WebSocket.OPEN) { socket.send(command + '\n'); setOutput(v => v + '$ ' + command + '\n'); setCommand('') }
  }
  return (
    <div className="modal-back"><div className="modal terminal-modal">
      <button className="close" onClick={onClose}>×</button>
      <h2>{container.names?.[0]?.replace('/', '') || container.id.slice(0, 12)}</h2>
      <pre>{output || 'Connecting…'}</pre>
      <form onSubmit={send}><span>$</span><input autoFocus value={command} onChange={e => setCommand(e.target.value)} autoComplete="off" /></form>
    </div></div>
  )
}

/* —— Websites (BT-style) —— */
function Websites() {
  const {t} = useI18n()
  const [tab, setTab] = useState<'sites' | 'certs'>('sites')
  const [items, setItems] = useState<Website[]>([]), [certs, setCerts] = useState<Certificate[]>([])
  const [edit, setEdit] = useState<{path: string; content: string; title: string} | null>(null)
  const [wizard, setWizard] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  const loadSites = () => api<Website[]>('/websites').then(v => { setItems(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))
  const loadCerts = () => api<Certificate[]>('/certificates').then(v => { setCerts(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))
  const load = () => { void loadSites(); void loadCerts() }
  useEffect(() => { load() }, [])

  async function openConfig(s: Website) {
    try {
      const r = await api<{path: string; content: string}>(`/websites/config?path=${encodeURIComponent(s.source_path)}`)
      setEdit({path: r.path, content: r.content, title: s.domains?.[0] || s.name})
    } catch (e) { setError((e as Error).message) }
  }
  async function applyConfig() {
    if (!edit) return
    await post('/actions', {kind: 'web.apply', resource: edit.path, options: {content: edit.content}})
    setEdit(null); setMessage(t('success')); setTimeout(loadSites, 1200)
  }
  async function issueSSL(site: Website) {
    const domain = site.domains?.[0]; if (!domain) return
    await post('/actions', {kind: 'cert.issue', resource: domain, options: {tool: 'certbot'}})
    setMessage(t('issueTask'))
  }
  async function setRewrite(site: Website) {
    const domain = site.domains?.[0]; if (!domain) return
    try {
      const rules = await api<RewriteRule[]>('/rewrite-rules')
      const ids = rules.map(r => r.id).join(', ')
      const pick = prompt(`${t('rewrite')} (${ids})`, 'none')
      if (!pick) return
      await post('/actions', {kind: 'web.site.rewrite', resource: domain, options: {rewrite: pick, server: site.server}})
      setMessage(t('success'))
    } catch (e) { setError((e as Error).message) }
  }
  async function deleteSite(site: Website) {
    const domain = site.domains?.[0]; if (!domain) return
    if (!site.source_path.includes('anpanel-site-')) { setError(t('onlyManaged')); return }
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: 'web.site.delete', resource: domain, options: {}})
    setMessage(t('success')); setTimeout(loadSites, 1200)
  }
  async function renew(domain = '', force = false) {
    await post('/actions', {kind: 'cert.renew', resource: domain, options: {force: force ? 'true' : 'false'}})
    setMessage(t('renewTask')); setTimeout(loadCerts, 2000)
  }
  function protoLabel(s: Website) {
    if (s.has_http && s.has_https) return t('bothHttpHttps')
    if (s.has_https || s.tls) return t('tlsOn')
    return t('tlsOff')
  }

  return (
    <>
      <PageHead title={t('websites')} action={<div className="toolbar">
        {tab === 'sites' && <button className="primary" onClick={() => setWizard(true)}><Plus size={16} />{t('addSite')}</button>}
        {tab === 'certs' && <button className="btn" onClick={() => renew('', false)}><RefreshCw />{t('renewAll')}</button>}
        <button className="btn" onClick={load}><RefreshCw />{t('refresh')}</button>
      </div>} />
      <div className="page-body">
        <div className="tabs">
          <button className={tab === 'sites' ? 'active' : ''} onClick={() => setTab('sites')}>{t('siteList')}</button>
          <button className={tab === 'certs' ? 'active' : ''} onClick={() => setTab('certs')}>{t('sslCerts')}</button>
        </div>
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}

        {tab === 'sites' && (
          <>
            <div className="panel table-panel">
              <table>
                <thead><tr><th>{t('domainLabel')}</th><th>{t('status')}</th><th>{t('proxy')}</th><th>{t('actions')}</th></tr></thead>
                <tbody>
                  {items.map(s => (
                    <tr key={s.id}>
                      <td>
                        <strong>{s.domains?.join(' ') || s.name}</strong>
                        <div style={{fontSize: 12, color: 'var(--muted)', marginTop: 2}}>{s.listen?.join(', ')}</div>
                      </td>
                      <td><span className={`pill ${(s.has_https || s.tls) ? 'green' : ''}`}>{protoLabel(s)}</span></td>
                      <td style={{maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                        {s.proxy_target || s.doc_root || t('staticSite')}
                      </td>
                      <td>
                        <div className="toolbar" style={{justifyContent: 'flex-end'}}>
                          {s.domains?.[0] && !(s.has_https || s.tls) && <button className="btn" onClick={() => issueSSL(s)}><FileKey2 size={14} />{t('issueSSL')}</button>}
                          {s.source_path.includes('anpanel-site-') && <button className="btn" onClick={() => setRewrite(s)}>{t('setRewrite')}</button>}
                          <button className="btn" onClick={() => openConfig(s)}>{t('advancedConfig')}</button>
                          {s.source_path.includes('anpanel-site-') && <button className="btn" onClick={() => deleteSite(s)}>{t('deleteSite')}</button>}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!items.length && !error && <div className="empty">{t('noData')}</div>}
            </div>
          </>
        )}

        {tab === 'certs' && (
          <div className="panel table-panel">
            <table>
              <thead><tr><th>{t('certDomain')}</th><th>{t('issuer')}</th><th>{t('expires')}</th><th>{t('daysLeft')}</th><th>{t('certSource')}</th><th>{t('status')}</th><th /></tr></thead>
              <tbody>
                {certs.map(c => {
                  const status = c.days_left < 0 ? 'expired' : c.days_left <= 14 ? 'expiring' : 'valid'
                  const dayClass = status === 'expired' ? 'days-bad' : status === 'expiring' ? 'days-warn' : 'days-ok'
                  return (
                    <tr key={c.domain + c.path}>
                      <td><strong>{c.domain}</strong></td>
                      <td>{c.issuer || '-'}</td>
                      <td>{c.expires_at ? new Date(c.expires_at).toLocaleString() : '-'}</td>
                      <td className={dayClass}>{c.days_left}</td>
                      <td><span className="pill">{c.source}</span></td>
                      <td><span className={`pill ${status === 'valid' ? 'green' : ''}`}>{status === 'expired' ? t('expired') : status === 'expiring' ? t('expiring') : t('valid')}</span></td>
                      <td className="actions">
                        <button title={t('renew')} onClick={() => renew(c.domain, false)}><RefreshCw /></button>
                        <button title={t('renewForce')} onClick={() => renew(c.domain, true)}><RotateCw /></button>
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
          <div className="modal-back"><div className="modal editor">
            <button className="close" onClick={() => setEdit(null)}>×</button>
            <h2>{edit.title}</h2>
            <small>{t('source')}: {edit.path}</small>
            <textarea spellCheck={false} value={edit.content} onChange={e => setEdit({...edit, content: e.target.value})} />
            <button className="primary" onClick={applyConfig}>{t('apply')}</button>
          </div></div>
        )}
        {wizard && <SiteWizard onClose={() => setWizard(false)} onCreated={() => { setWizard(false); setMessage(t('siteCreated')); setTimeout(load, 1500) }} />}
      </div>
    </>
  )
}

function SiteWizard({onClose, onCreated}: {onClose: () => void; onCreated: () => void}) {
  const {t} = useI18n()
  const [step, setStep] = useState(0)
  const [siteType, setSiteType] = useState<'proxy' | 'static'>('proxy')
  const [domain, setDomain] = useState(''), [root, setRoot] = useState(''), [proxyPass, setProxyPass] = useState('http://127.0.0.1:3000')
  const [rewrite, setRewrite] = useState('none'), [rules, setRules] = useState<RewriteRule[]>([])
  const [enableSSL, setEnableSSL] = useState(false), [tool, setTool] = useState('certbot'), [email, setEmail] = useState('')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  useEffect(() => { api<RewriteRule[]>('/rewrite-rules').then(v => setRules(Array.isArray(v) ? v : [])).catch(() => {}) }, [])
  async function submit() {
    setBusy(true); setError('')
    try {
      const options: Record<string, string> = {
        domain: domain.trim(), site_type: siteType, rewrite,
        enable_ssl: enableSSL ? 'true' : 'false', tool, email,
      }
      if (siteType === 'static') options.root = root.trim()
      else options.proxy_pass = proxyPass.trim()
      await post('/actions', {kind: 'web.site.create', resource: domain.trim(), options})
      onCreated()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  const canNext = step === 0 || (step === 1 && domain.trim().includes('.') && (siteType === 'proxy' ? proxyPass.trim().startsWith('http') : true))
  return (
    <div className="modal-back"><div className="modal wizard">
      <button className="close" onClick={onClose}>×</button>
      <h2>{t('siteWizard')}</h2>
      <div className="wizard-steps">
        <span className={step === 0 ? 'active' : step > 0 ? 'done' : ''}>{siteType === 'proxy' ? t('siteTypeProxy') : t('siteTypeStatic')}</span>
        <span className={step === 1 ? 'active' : step > 1 ? 'done' : ''}>{t('domainLabel')}</span>
        <span className={step === 2 ? 'active' : ''}>{t('rewrite')} / SSL</span>
      </div>
      {step === 0 && (
        <div className="type-cards">
          <button type="button" className={`type-card ${siteType === 'proxy' ? 'selected' : ''}`} onClick={() => setSiteType('proxy')}><strong>{t('siteTypeProxy')}</strong><span>{t('siteTypeProxyHint')}</span></button>
          <button type="button" className={`type-card ${siteType === 'static' ? 'selected' : ''}`} onClick={() => setSiteType('static')}><strong>{t('siteTypeStatic')}</strong><span>{t('siteTypeStaticHint')}</span></button>
        </div>
      )}
      {step === 1 && (
        <>
          <label>{t('domainLabel')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="example.com" autoFocus /></label>
          <small style={{color: 'var(--muted)'}}>{t('domainHintSite')}</small>
          {siteType === 'proxy' ? (
            <label>{t('proxyPass')}<input value={proxyPass} onChange={e => setProxyPass(e.target.value)} /><small style={{color: 'var(--muted)', fontWeight: 400}}>{t('proxyPassHint')}</small></label>
          ) : (
            <label>{t('docRoot')}<input value={root} onChange={e => setRoot(e.target.value)} placeholder={`/var/www/${domain || 'example.com'}`} /><small style={{color: 'var(--muted)', fontWeight: 400}}>{t('docRootHint')}</small></label>
          )}
        </>
      )}
      {step === 2 && (
        <>
          {siteType === 'static' && (
            <label>{t('rewrite')}
              <select value={rewrite} onChange={e => setRewrite(e.target.value)}>
                {(rules.length ? rules : [{id: 'none', name: 'none', description: '', nginx: '', apache: ''}]).map(r => (
                  <option key={r.id} value={r.id}>{r.name}{r.description ? ` — ${r.description}` : ''}</option>
                ))}
              </select>
              <small style={{color: 'var(--muted)', fontWeight: 400}}>{t('rewriteHint')}</small>
            </label>
          )}
          <label className="check-row"><input type="checkbox" checked={enableSSL} onChange={e => setEnableSSL(e.target.checked)} />{t('enableSSL')}</label>
          {enableSSL && (
            <>
              <label>{t('acmeTool')}<select value={tool} onChange={e => setTool(e.target.value)}><option value="certbot">certbot</option><option value="acme.sh">acme.sh</option></select></label>
              <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>
              <div className="steps"><ol><li>{t('step1')}</li><li>{t('step2')}</li><li>{t('step3')}</li></ol></div>
            </>
          )}
        </>
      )}
      {error && <div className="error">{error}</div>}
      <div className="card-actions">
        <button type="button" className="btn" onClick={onClose}>{t('cancel')}</button>
        {step > 0 && <button type="button" className="btn" onClick={() => setStep(s => s - 1)}>{t('prev')}</button>}
        {step < 2 && <button type="button" className="primary" disabled={!canNext} onClick={() => setStep(s => s + 1)}>{t('next')}</button>}
        {step === 2 && <button type="button" className="primary" disabled={busy || !domain.trim()} onClick={submit}>{busy ? '…' : t('createSite')}</button>}
      </div>
    </div></div>
  )
}

/* —— Files —— */
function FilesPage() {
  const {t} = useI18n()
  const [path, setPath] = useState('/var/www')
  const [items, setItems] = useState<FileEntry[]>([])
  const [error, setError] = useState(''), [message, setMessage] = useState('')
  const [edit, setEdit] = useState<{path: string; content: string} | null>(null)

  const load = (p = path) => api<FileEntry[]>(`/files?path=${encodeURIComponent(p)}`)
    .then(v => { setItems(Array.isArray(v) ? v : []); setPath(p); setError('') })
    .catch(e => setError(e.message))

  useEffect(() => { void load('/var/www') }, [])

  function parentOf(p: string) {
    const roots = ['/var/www', '/www', '/srv', '/opt', '/home', '/etc/anpanel/compose', '/var/lib/anpanel']
    const norm = p.replace(/\\/g, '/').replace(/\/$/, '') || '/var/www'
    if (roots.includes(norm)) return norm
    const parts = norm.split('/').filter(Boolean)
    if (parts.length <= 1) return '/var/www'
    const parent = '/' + parts.slice(0, -1).join('/')
    // stay within an allowed root
    for (const root of roots) {
      if (parent === root || parent.startsWith(root + '/')) return parent
    }
    return roots.find(r => norm.startsWith(r + '/') || norm === r) || '/var/www'
  }

  async function openFile(f: FileEntry) {
    if (f.is_dir) { void load(f.path); return }
    try {
      const r = await api<{path: string; content: string}>(`/files/content?path=${encodeURIComponent(f.path)}`)
      setEdit({path: r.path, content: r.content})
    } catch (e) { setError((e as Error).message) }
  }

  async function saveFile() {
    if (!edit) return
    await post('/actions', {kind: 'files.write', resource: edit.path, options: {content: edit.content}})
    setEdit(null); setMessage(t('success')); void load()
  }

  async function newFolder() {
    const name = prompt(t('newFolder'))
    if (!name) return
    await post('/actions', {kind: 'files.mkdir', resource: `${path.replace(/\/$/, '')}/${name}`, options: {}})
    setTimeout(() => load(), 500)
  }

  async function newFile() {
    const name = prompt(t('newFile'))
    if (!name) return
    const fp = `${path.replace(/\/$/, '')}/${name}`
    await post('/actions', {kind: 'files.write', resource: fp, options: {content: ''}})
    setTimeout(() => load(), 500)
  }

  async function remove(f: FileEntry) {
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: 'files.delete', resource: f.path, options: {}})
    setTimeout(() => load(), 500)
  }

  async function rename(f: FileEntry) {
    const name = prompt(t('rename'), f.name)
    if (!name || name === f.name) return
    const to = `${path.replace(/\/$/, '')}/${name}`
    await post('/actions', {kind: 'files.rename', resource: f.path, options: {to}})
    setTimeout(() => load(), 500)
  }

  return (
    <>
      <PageHead title={t('filesTitle')} action={<div className="toolbar">
        <button className="btn" onClick={() => load(parentOf(path))}><ChevronUp />{t('parentDir')}</button>
        <button className="btn" onClick={newFolder}><Plus size={14} />{t('newFolder')}</button>
        <button className="btn" onClick={newFile}><FileText size={14} />{t('newFile')}</button>
        <button className="btn" onClick={() => load()}><RefreshCw />{t('refresh')}</button>
      </div>} />
      <div className="page-body">
        <div className="panel" style={{marginBottom: 12, padding: '10px 14px', fontFamily: 'ui-monospace,monospace', fontSize: 13}}>{t('path')}: {path}</div>
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        <div className="panel table-panel">
          <table>
            <thead><tr><th>{t('path')}</th><th>{t('size')}</th><th>{t('modified')}</th><th /></tr></thead>
            <tbody>
              {items.map(f => (
                <tr key={f.path}>
                  <td>
                    <button className="btn" style={{border: 0, background: 'transparent', padding: 0, color: 'var(--text)'}} onClick={() => openFile(f)}>
                      {f.is_dir ? <FolderOpen size={16} style={{verticalAlign: 'middle', marginRight: 6, color: 'var(--green)'}} /> : <FileText size={16} style={{verticalAlign: 'middle', marginRight: 6, color: 'var(--muted)'}} />}
                      {f.name}
                    </button>
                  </td>
                  <td>{f.is_dir ? '-' : bytes(f.size)}</td>
                  <td>{f.mod_time ? new Date(f.mod_time).toLocaleString() : '-'}</td>
                  <td className="actions">
                    {!f.is_dir && <button title={t('edit')} onClick={() => openFile(f)}><Pencil /></button>}
                    <button title={t('rename')} onClick={() => rename(f)}><FileText /></button>
                    <button className="danger" title={t('remove')} onClick={() => remove(f)}><Trash2 /></button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!items.length && !error && <div className="empty">{t('noData')}</div>}
        </div>
        {edit && (
          <div className="modal-back"><div className="modal editor">
            <button className="close" onClick={() => setEdit(null)}>×</button>
            <h2>{edit.path}</h2>
            <small>{t('uploadHint')}</small>
            <textarea spellCheck={false} value={edit.content} onChange={e => setEdit({...edit, content: e.target.value})} />
            <button className="primary" onClick={saveFile}>{t('save')}</button>
          </div></div>
        )}
      </div>
    </>
  )
}

/* —— Services —— */
function Services() {
  const {t} = useI18n()
  const [items, setItems] = useState<Service[]>([]), [error, setError] = useState(''), [message, setMessage] = useState('')
  const [installDlg, setInstallDlg] = useState<Service | null>(null)
  const load = () => api<Service[]>('/services').then(v => { setItems(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))
  useEffect(() => { void load() }, [])
  async function act(s: Service, verb: string) {
    let resource = s.name
    if (s.name === 'apache') resource = s.path?.includes('apache2') ? 'apache2' : 'httpd'
    if (s.name === 'php') resource = 'php-fpm'
    await post('/actions', {kind: `service.${verb}`, resource, options: {}}); setTimeout(load, 600)
  }
  async function doUpdate(s: Service) {
    try {
      await post('/actions', {kind: 'package.update', resource: s.name, options: {method: s.default_method || 'source'}})
      setMessage(t('updateQueued'))
    } catch (e) { setError((e as Error).message) }
  }
  function groupLabel(g?: string) {
    if (g === 'web') return t('groupWeb')
    if (g === 'ssl') return t('groupSSL')
    if (g === 'runtime') return t('groupRuntime')
    if (g === 'container') return t('groupContainer')
    return g || ''
  }
  const groups = ['web', 'ssl', 'runtime', 'container']
  return (
    <>
      <PageHead title={t('services')} action={<button className="btn" onClick={load}><RefreshCw />{t('refresh')}</button>} />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        {groups.map(g => {
          const list = items.filter(s => (s.group || '') === g)
          if (!list.length) return null
          return (
            <div key={g} style={{marginBottom: 18}}>
              <h3 style={{margin: '0 0 10px', fontSize: 14, color: 'var(--muted)'}}>{groupLabel(g)}</h3>
              <div className="service-cards">
                {list.map(s => (
                  <div className="panel service-card" key={s.name}>
                    <div className="resource">
                      <span className="cube"><ServerCog /></span>
                      <div>
                        <h3>{s.display_name || s.name}</h3>
                        <small>{s.version || s.path || t('missing')}</small>
                      </div>
                    </div>
                    <span className={`pill ${s.status === 'active' || s.status === 'available' ? 'green' : ''}`}>{s.status}</span>
                    {s.note && <p style={{gridColumn: '1/-1', margin: 0, fontSize: 12, color: 'var(--muted)'}}>{s.note}</p>}
                    {s.block_reason && <p style={{gridColumn: '1/-1', margin: 0, fontSize: 12, color: 'var(--danger)'}}>{s.block_reason}</p>}
                    <div className="card-actions">
                      {s.installed && ['nginx', 'apache', 'docker', 'php'].includes(s.name) && (
                        <>
                          <button className="btn" onClick={() => act(s, s.status === 'active' ? 'stop' : 'start')}>{s.status === 'active' ? t('stop') : t('start')}</button>
                          <button className="btn" onClick={() => act(s, 'restart')}>{t('restart')}</button>
                        </>
                      )}
                      {s.can_update && <button className="btn" onClick={() => doUpdate(s)}>{t('updateSoft')}</button>}
                      {s.can_install && <button className="primary" onClick={() => setInstallDlg(s)}>{t('installSoft')}</button>}
                      {!s.installed && !s.can_install && s.block_reason && (
                        <button className="btn" disabled title={s.block_reason}>{t('conflictBlocked')}</button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
        {installDlg && (
          <InstallDialog
            service={installDlg}
            onClose={() => setInstallDlg(null)}
            onDone={() => { setInstallDlg(null); setMessage(t('installQueued')); setTimeout(load, 1500) }}
          />
        )}
      </div>
    </>
  )
}

function InstallDialog({service, onClose, onDone}: {service: Service; onClose: () => void; onDone: () => void}) {
  const {t} = useI18n()
  const methods = service.install_methods?.length ? service.install_methods : ['source']
  const [method, setMethod] = useState(service.default_method || methods[0] || 'source')
  const [version, setVersion] = useState(service.versions?.[service.versions.length - 2] || service.versions?.[0] || '8.3')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  function methodLabel(m: string) {
    if (m === 'source') return t('methodSource')
    if (m === 'package') return t('methodPackage')
    if (m === 'script') return t('methodScript')
    return m
  }
  async function submit() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'package.install', resource: service.name, options: {method, version}})
      onDone()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  return (
    <div className="modal-back"><div className="modal">
      <button className="close" onClick={onClose}>×</button>
      <h2>{t('installSoft')} {service.display_name || service.name}</h2>
      {service.conflicts?.length ? <p style={{margin: 0, fontSize: 13, color: 'var(--muted)'}}>互斥：{service.conflicts.join(', ')}</p> : null}
      <label>{t('installMethod')}
        <select value={method} onChange={e => setMethod(e.target.value)}>
          {methods.map(m => <option key={m} value={m}>{methodLabel(m)}</option>)}
        </select>
      </label>
      {service.name === 'php' && service.versions?.length && (
        <label>{t('phpVersion')}
          <select value={version} onChange={e => setVersion(e.target.value)}>
            {service.versions.map(v => <option key={v} value={v}>{v}</option>)}
          </select>
        </label>
      )}
      {method === 'source' && <p style={{margin: 0, fontSize: 12, color: 'var(--muted)'}}>编译安装可能需要数分钟到数十分钟，进度请在「计划任务」查看。</p>}
      {error && <div className="error">{error}</div>}
      <div className="card-actions">
        <button className="btn" onClick={onClose}>{t('cancel')}</button>
        <button className="primary" disabled={busy} onClick={submit}>{busy ? '…' : t('installSoft')}</button>
      </div>
    </div></div>
  )
}

/* —— Tasks + Crontab —— */
function Tasks() {
  const {t} = useI18n()
  const [tab, setTab] = useState<'tasks' | 'cron'>('tasks')
  const [tasks, setTasks] = useState<Task[]>([]), [audits, setAudits] = useState<Audit[]>([]), [crons, setCrons] = useState<CronJob[]>([])
  const [schedule, setSchedule] = useState('0 3 * * *'), [command, setCommand] = useState(''), [message, setMessage] = useState(''), [error, setError] = useState('')

  const loadTasks = () => {
    api<Task[]>('/tasks').then(v => setTasks(Array.isArray(v) ? v : [])).catch(() => {})
    api<Audit[]>('/audits').then(v => setAudits(Array.isArray(v) ? v : [])).catch(() => {})
  }
  const loadCron = () => api<CronJob[]>('/crontab').then(v => { setCrons(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))

  useEffect(() => {
    loadTasks(); loadCron()
    const i = setInterval(loadTasks, 3000)
    return () => clearInterval(i)
  }, [])

  async function addCron() {
    try {
      await post('/actions', {kind: 'crontab.add', resource: 'root', options: {schedule, command}})
      setMessage(t('cronAdded')); setCommand(''); setTimeout(loadCron, 800)
    } catch (e) { setError((e as Error).message) }
  }
  async function removeCron(id: string) {
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    await post('/actions', {kind: 'crontab.remove', resource: id, options: {}})
    setMessage(t('cronRemoved')); setTimeout(loadCron, 800)
  }

  return (
    <>
      <PageHead title={t('tasks')} action={<button className="btn" onClick={() => { loadTasks(); loadCron() }}><RefreshCw />{t('refresh')}</button>} />
      <div className="page-body">
        <div className="tabs">
          <button className={tab === 'tasks' ? 'active' : ''} onClick={() => setTab('tasks')}>{t('taskQueue')}</button>
          <button className={tab === 'cron' ? 'active' : ''} onClick={() => setTab('cron')}>{t('crontabTitle')}</button>
        </div>
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        {error && <div className="error banner">{error}</div>}

        {tab === 'tasks' && (
          <section className="split">
            <div className="panel"><PanelTitle title={t('tasksTitle')} />
              <div className="timeline">
                {tasks.map(x => <div key={x.id}><span className={`task-dot ${x.status}`} /><div><strong>{x.summary}</strong><small>{new Date(x.created_at).toLocaleString()} / {x.status}</small>{x.log && <pre>{x.log}</pre>}</div></div>)}
                {!tasks.length && <div className="empty">{t('noData')}</div>}
              </div>
            </div>
            <div className="panel"><PanelTitle title={t('auditTitle')} />
              <div className="timeline">
                {audits.map(x => <div key={x.id}><span className="task-dot succeeded" /><div><strong>{x.action}</strong><small>{x.actor} / {x.resource}</small><p>{x.detail}</p></div></div>)}
                {!audits.length && <div className="empty">{t('noData')}</div>}
              </div>
            </div>
          </section>
        )}

        {tab === 'cron' && (
          <div className="stack">
            <div className="panel alert-form">
              <PanelTitle title={t('addCron')} />
              <p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('cronHint')}</p>
              <label>{t('cronSchedule')}<input value={schedule} onChange={e => setSchedule(e.target.value)} placeholder="0 3 * * *" /></label>
              <label>{t('cronCommand')}<input value={command} onChange={e => setCommand(e.target.value)} placeholder="/usr/bin/example" /></label>
              <button className="primary" disabled={!command.trim()} onClick={addCron}>{t('addCron')}</button>
            </div>
            <div className="panel table-panel">
              <table>
                <thead><tr><th>ID</th><th>{t('cronSchedule')}</th><th>{t('cronCommand')}</th><th /></tr></thead>
                <tbody>
                  {crons.map(c => (
                    <tr key={c.id}>
                      <td>{c.id}</td>
                      <td><code>{c.schedule}</code></td>
                      <td><code>{c.command}</code></td>
                      <td className="actions"><button className="danger" onClick={() => removeCron(c.id)}><Trash2 /></button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!crons.length && <div className="empty">{t('noData')}</div>}
            </div>
          </div>
        )}
      </div>
    </>
  )
}

function RecentTasks() {
  const {t} = useI18n()
  const [items, setItems] = useState<Task[]>([])
  useEffect(() => { api<Task[]>('/tasks').then(v => setItems(Array.isArray(v) ? v : [])).catch(() => setItems([])) }, [])
  return (
    <div className="panel"><PanelTitle title={t('recentTasks')} />
      <div className="timeline compact">
        {items.slice(0, 5).map(x => <div key={x.id}><span className={`task-dot ${x.status}`} /><div><strong>{x.kind}</strong><small>{x.status}</small></div></div>)}
        {!items.length && <div className="empty">{t('noData')}</div>}
      </div>
    </div>
  )
}

/* —— Alerts —— */
function Alerts() {
  const {t} = useI18n()
  const [rules, setRules] = useState<AlertRule[]>([])
  const [name, setName] = useState('High CPU'), [metric, setMetric] = useState('cpu')
  const [threshold, setThreshold] = useState(90), [duration, setDuration] = useState(300)
  const [webhook, setWebhook] = useState(''), [message, setMessage] = useState('')
  const load = () => api<AlertRule[]>('/alerts/rules').then(v => setRules(Array.isArray(v) ? v : [])).catch(() => {})
  useEffect(() => { void load() }, [])
  async function save() {
    await post('/alerts/rules/update', {operation: 'save', rule: {id: 0, name, metric, operator: 'gt', threshold, duration_seconds: duration, silence_seconds: 300, repeat_seconds: 3600, enabled: true}})
    await load()
  }
  async function remove(rule: AlertRule) { await post('/alerts/rules/update', {operation: 'delete', rule}); await load() }
  async function saveNotify() {
    await post('/actions', {kind: 'notification.configure', resource: 'notifications', options: {json: JSON.stringify({webhook_url: webhook})}})
    setMessage(t('notifyTask'))
  }
  return (
    <>
      <PageHead title={t('alerts')} />
      <div className="page-body">
        <section className="split">
          <div className="panel alert-form">
            <PanelTitle title={t('newRule')} />
            <label>{t('ruleName')}<input value={name} onChange={e => setName(e.target.value)} /></label>
            <label>{t('metric')}<select value={metric} onChange={e => setMetric(e.target.value)}><option value="cpu">CPU %</option><option value="memory">Memory %</option><option value="disk">Disk %</option><option value="load">Load</option></select></label>
            <label>{t('threshold')}<input type="number" value={threshold} onChange={e => setThreshold(+e.target.value)} /></label>
            <label>{t('durationSec')}<input type="number" value={duration} onChange={e => setDuration(+e.target.value)} /></label>
            <button className="primary" onClick={save}>{t('save')}</button>
          </div>
          <div className="panel alert-form">
            <PanelTitle title={t('notify')} />
            <label>Webhook URL<input value={webhook} onChange={e => setWebhook(e.target.value)} placeholder="https://example.com/hook" /></label>
            <button className="primary" onClick={saveNotify}>{t('save')}</button>
            {message && <div className="success">{message}</div>}
          </div>
        </section>
        <div className="panel table-panel alert-table">
          <table>
            <thead><tr><th>{t('ruleName')}</th><th>{t('metric')}</th><th>{t('threshold')}</th><th>{t('durationSec')}</th><th /></tr></thead>
            <tbody>
              {rules.map(r => <tr key={r.id}><td>{r.name}</td><td>{r.metric}</td><td>{r.operator} {r.threshold}</td><td>{r.duration_seconds}s</td><td className="actions"><button className="danger" onClick={() => remove(r)}><Trash2 /></button></td></tr>)}
            </tbody>
          </table>
          {!rules.length && <div className="empty">{t('noData')}</div>}
        </div>
      </div>
    </>
  )
}

/* —— Settings (panel domain wizard + security + update) —— */
function Settings({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [tab, setTab] = useState<'panel' | 'security' | 'update'>('panel')
  return (
    <>
      <PageHead title={t('settings')} />
      <div className="page-body">
        <div className="tabs">
          <button className={tab === 'panel' ? 'active' : ''} onClick={() => setTab('panel')}>{t('settingsTabPanel')}</button>
          <button className={tab === 'security' ? 'active' : ''} onClick={() => setTab('security')}>{t('settingsTabSecurity')}</button>
          <button className={tab === 'update' ? 'active' : ''} onClick={() => setTab('update')}>{t('settingsTabUpdate')}</button>
        </div>
        {tab === 'panel' && <PanelDomainWizard />}
        {tab === 'security' && <SecurityBlock me={me} setMe={setMe} />}
        {tab === 'update' && <UpdateBlock />}
      </div>
    </>
  )
}

function PanelDomainWizard() {
  const {t} = useI18n()
  const [step, setStep] = useState(0)
  const [domain, setDomain] = useState(''), [tool, setTool] = useState('certbot'), [email, setEmail] = useState('')
  const [message, setMessage] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)

  async function bind() {
    setBusy(true); setError(''); setMessage('')
    try {
      await post('/actions', {kind: 'panel.bind_domain', resource: domain, options: {domain, tool, email}})
      setMessage(t('domainTask')); setStep(2)
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  async function unbind() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'panel.unbind_domain', resource: 'panel', options: {}})
      setMessage(t('unbindTask'))
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  return (
    <div className="panel domain-card" style={{display: 'block'}}>
      <div style={{display: 'flex', gap: 12, alignItems: 'flex-start', marginBottom: 12}}>
        <Globe2 style={{color: 'var(--green)', width: 28, height: 28, flex: 'none'}} />
        <div><h2 style={{margin: '0 0 4px'}}>{t('domainHttps')}</h2><p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('domainHint')}</p></div>
      </div>
      <div className="wizard-steps">
        <span className={step === 0 ? 'active' : step > 0 ? 'done' : ''}>{t('wizardDomain')}</span>
        <span className={step === 1 ? 'active' : step > 1 ? 'done' : ''}>{t('wizardSSL')}</span>
        <span className={step === 2 ? 'active' : ''}>{t('wizardDone')}</span>
      </div>
      {step === 0 && (
        <>
          <div className="steps" style={{marginBottom: 14}}><h4>{t('domainSteps')}</h4><ol><li>{t('step1')}</li><li>{t('step2')}</li><li>{t('step3')}</li></ol></div>
          <label>{t('domainLabel')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder={t('domainPlaceholder')} /></label>
          <div className="card-actions" style={{marginTop: 12}}>
            <button className="primary" disabled={!domain.trim().includes('.')} onClick={() => setStep(1)}>{t('next')}</button>
            <button className="btn" disabled={busy} onClick={unbind}>{t('unbindDomain')}</button>
          </div>
        </>
      )}
      {step === 1 && (
        <>
          <label>{t('acmeTool')}<select value={tool} onChange={e => setTool(e.target.value)}><option value="certbot">certbot</option><option value="acme.sh">acme.sh</option></select></label>
          <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>
          <div className="card-actions" style={{marginTop: 12}}>
            <button className="btn" onClick={() => setStep(0)}>{t('prev')}</button>
            <button className="primary" disabled={busy} onClick={bind}>{busy ? '…' : t('startBind')}</button>
          </div>
        </>
      )}
      {step === 2 && (
        <div className="success">{message || t('domainTask')}</div>
      )}
      {error && <div className="error banner" style={{marginTop: 12}}>{error}</div>}
      {message && step !== 2 && <div className="success" style={{marginTop: 12}}>{message}</div>}
    </div>
  )
}

function SecurityBlock({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [secret, setSecret] = useState<{secret: string; uri: string} | null>(null)
  const [code, setCode] = useState(''), [message, setMessage] = useState(''), [error, setError] = useState('')
  async function setup() { try { setSecret(await post('/me/totp/setup', {})) } catch (e) { setError((e as Error).message) } }
  async function enable() {
    try {
      await post('/me/totp/enable', {Code: code})
      setMe({...me, totp_enabled: true}); setMessage(t('totpEnabled')); setSecret(null)
    } catch (e) { setError((e as Error).message) }
  }
  return (
    <div className="panel security-card">
      <LockKeyhole />
      <div><h2>{t('totpTitle')}</h2><p>{me.totp_enabled ? t('totpOn') : t('totpOff')}</p></div>
      {!me.totp_enabled && !secret && <button className="primary" onClick={setup}>{t('setupTOTP')}</button>}
      {secret && (
        <div className="totp-setup">
          <code>{secret.secret}</code>
          <p>{t('totpSecretHint')}</p>
          <input value={code} onChange={e => setCode(e.target.value)} maxLength={6} />
          <button className="primary" onClick={enable}>{t('totpVerify')}</button>
        </div>
      )}
      {message && <div className="success">{message}</div>}
      {error && <div className="error">{error}</div>}
    </div>
  )
}

function UpdateBlock() {
  const {t} = useI18n()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [channel, setChannel] = useState<'stable' | 'prerelease'>('stable')
  const [message, setMessage] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)

  const load = () => api<SystemInfo>('/system').then(v => {
    setInfo(v)
    if (v.channel === 'prerelease' || v.channel === 'stable') setChannel(v.channel)
  }).catch(e => setError(e.message))

  useEffect(() => { void load() }, [])

  async function doUpdate() {
    if (!confirm(channel === 'prerelease' ? 'Update to prerelease?' : 'Update to stable?')) return
    setBusy(true); setError(''); setMessage('')
    try {
      await post('/actions', {kind: 'panel.self_update', resource: channel, options: {channel}})
      setMessage(t('updateTask'))
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  return (
    <div className="panel" style={{maxWidth: 720}}>
      <div style={{display: 'flex', gap: 12, alignItems: 'flex-start', marginBottom: 16}}>
        <Download style={{color: 'var(--green)', width: 28, height: 28}} />
        <div><h2 style={{margin: '0 0 4px', fontSize: 16}}>{t('updateTitle')}</h2><p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('updateHint')}</p></div>
      </div>
      {info && (
        <div style={{display: 'grid', gap: 10, marginBottom: 16, fontSize: 13}}>
          <div><strong>{t('currentVersion')}:</strong> {info.version}</div>
          <div><strong>{t('webServerUsed')}:</strong> {info.web_server || '-'}</div>
          <div><strong>{t('latestStable')}:</strong> {info.latest_stable || '-'}</div>
          <div><strong>{t('latestPre')}:</strong> {info.latest_prerelease || '-'}</div>
        </div>
      )}
      <label>{t('channel')}
        <select value={channel} onChange={e => setChannel(e.target.value as 'stable' | 'prerelease')}>
          <option value="stable">{t('channelStable')}</option>
          <option value="prerelease">{t('channelPre')}</option>
        </select>
      </label>
      <div className="card-actions" style={{marginTop: 14}}>
        <button className="btn" onClick={load}><RefreshCw size={14} />{t('checkUpdate')}</button>
        <button className="primary" disabled={busy} onClick={doUpdate}>{busy ? '…' : t('doUpdate')}</button>
      </div>
      {message && <div className="success" style={{marginTop: 12}}>{message}</div>}
      {error && <div className="error banner" style={{marginTop: 12}}>{error}</div>}
    </div>
  )
}

/* —— shared —— */
function PageHead({title, hint, action}: {title: string; hint?: string; action?: React.ReactNode}) {
  return <header className="page-head"><div><h1>{title}</h1>{hint && <p className="hint">{hint}</p>}</div>{action && <div className="actions-row">{action}</div>}</header>
}
function PanelTitle({title}: {title: string}) {
  return <div className="panel-title"><h3>{title}</h3><span>LIVE</span></div>
}
function Loading() {
  const {t} = useI18n()
  return <div className="loading"><RefreshCw /> {t('loading')}</div>
}
function pct(a = 0, b = 0) { return b ? (a / b) * 100 : 0 }
function num(v = 0) { return Number.isFinite(v) ? v.toFixed(1) : '0.0' }
function bytes(v = 0) {
  if (!v) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(v) / Math.log(1024)), 4)
  return `${(v / 1024 ** i).toFixed(1)} ${u[i]}`
}

createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>)
