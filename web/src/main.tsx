import React, {useEffect, useMemo, useRef, useState} from 'react'
import {createRoot} from 'react-dom/client'
import {Terminal as XTerm} from '@xterm/xterm'
import {FitAddon} from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import {
  Activity, Box, Globe2, ServerCog, ListChecks, Settings2, LogOut, RefreshCw,
  Play, Square, RotateCw, Trash2, Terminal, LockKeyhole, Languages, BellRing,
  Cpu, HardDrive, Database, Plus, FileKey2, FolderOpen, FileText, ChevronUp,
  Pencil, Download, Search, Home, ChevronRight, ShieldCheck, CheckCircle2,
  KeyRound, Link2, Network,
} from 'lucide-react'
import {api, post, setCSRF} from './api'
import {I18n, Lang, translator, useI18n} from './i18n'
import type {
  AlertRule, Audit, Certificate, Container, CronJob, FileEntry, Me, RewriteRule, Service,
  Snapshot, SystemInfo, Task, Website,
} from './types'
import './style.css'
import './alerts.css'

type Page = 'dashboard' | 'docker' | 'ssh' | 'websites' | 'files' | 'services' | 'tasks' | 'alerts' | 'settings'

const pagePaths: Record<Page, string> = {dashboard: '/', docker: '/docker', ssh: '/ssh', websites: '/website', files: '/files', services: '/apps', tasks: '/tasks', alerts: '/alerts', settings: '/settings/general'}
function pageFromPath(path: string): Page {
  if (path === '/settings' || path.startsWith('/settings/')) return 'settings'
  return (Object.entries(pagePaths).find(([, value]) => value === path)?.[0] as Page) || 'dashboard'
}

function cached<T>(key: string, fallback: T): T {
  try { return JSON.parse(sessionStorage.getItem(key) || '') as T } catch { return fallback }
}
function cache(key: string, value: unknown) {
  try { sessionStorage.setItem(key, JSON.stringify(value)) } catch { /* storage unavailable */ }
}

function BrandMark() {
  return <span className="brandmark"><img src="/favicon.svg" alt="" /></span>
}

function App() {
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [lang, setLang] = useState<Lang>(() => (localStorage.lang || (navigator.language.startsWith('zh') ? 'zh' : 'en')) as Lang)

  useEffect(() => {
    api<Me>('/me').then(v => { setCSRF(v.csrf_token); setMe(v) }).catch(() => setMe(null)).finally(() => setLoading(false))
  }, [])
  useEffect(() => { localStorage.lang = lang }, [lang])
  const value = useMemo(() => ({lang, setLang, t: translator(lang)}), [lang])
  if (loading) return <div className="splash"><BrandMark /></div>
  return <I18n.Provider value={value}>{me ? <Shell me={me} setMe={setMe} /> : <Login setMe={setMe} />}</I18n.Provider>
}

function Login({setMe}: {setMe: (m: Me) => void}) {
  const {t, lang, setLang} = useI18n()
  const [username, setUser] = useState('admin'), [password, setPass] = useState(''), [totp, setTotp] = useState('')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const [totpEnabled, setTotpEnabled] = useState(false)
  useEffect(() => { api<{totp_enabled: boolean}>('/auth/config').then(v => setTotpEnabled(v.totp_enabled)).catch(() => {}) }, [])
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
        <div className="brand"><BrandMark />AnPanel</div>
        <h2>{t('loginTitle')}</h2>
        <p className="subtitle">{t('loginSubtitle')}</p>
        <label>{t('username')}<input value={username} onChange={e => setUser(e.target.value)} autoComplete="username" /></label>
        <label>{t('password')}<input type="password" value={password} onChange={e => setPass(e.target.value)} autoComplete="current-password" autoFocus /></label>
        {totpEnabled && <label>{t('totp')}<input inputMode="numeric" maxLength={6} value={totp} onChange={e => setTotp(e.target.value)} /></label>}
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={busy}>{busy ? '…' : t('login')}</button>
      </form>
    </main>
  )
}

const nav: [Page, React.ElementType, 'dashboard' | 'docker' | 'ssh' | 'websites' | 'files' | 'services' | 'tasks' | 'alerts' | 'settings'][] = [
  ['dashboard', Activity, 'dashboard'],
  ['docker', Box, 'docker'],
  ['ssh', KeyRound, 'ssh'],
  ['websites', Globe2, 'websites'],
  ['files', FolderOpen, 'files'],
  ['services', ServerCog, 'services'],
  ['tasks', ListChecks, 'tasks'],
  ['alerts', BellRing, 'alerts'],
  ['settings', Settings2, 'settings'],
]

function Shell({me, setMe}: {me: Me; setMe: (m: Me | null) => void}) {
  const {t, lang, setLang} = useI18n()
  const [page, setPage] = useState<Page>(() => pageFromPath(location.pathname))
  useEffect(() => { const pop = () => setPage(pageFromPath(location.pathname)); addEventListener('popstate', pop); return () => removeEventListener('popstate', pop) }, [])
  function navigate(next: Page) { if (next !== page) history.pushState({}, '', pagePaths[next]); setPage(next) }
  async function logout() { try { await post('/auth/logout', {}) } catch { /* */ } setMe(null) }
  return (
    <div className="shell">
      <aside>
        <div className="brand"><BrandMark /><span>AnPanel</span></div>
        <nav>
          {nav.map(([id, Icon, label]) => (
            <button key={id} className={page === id ? 'active' : ''} onClick={() => navigate(id)}>
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
        {!me.must_change && me.must_set_entry && <ForceEntrySetup me={me} setMe={setMe} />}
        {page === 'dashboard' && <Dashboard goSettings={() => navigate('settings')} />}
        {page === 'docker' && <DockerPage />}
        {page === 'ssh' && <HostSSH onClose={() => navigate('dashboard')} />}
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

function entryPathHint(path: string, t: (k: any) => string): string {
  const p = path.trim().replace(/^\/+|\/+$/g, '')
  if (!p) return t('entryErrEmpty')
  if (p.length < 4) return t('entryErrShort')
  if (p.length > 64) return t('entryErrLong')
  if (!/^[a-zA-Z0-9_-]+$/.test(p)) return t('entryErrCharset')
  if (['api', 'assets', 'static', 'favicon.ico', 'robots.txt'].includes(p.toLowerCase())) return `${t('entryErrReserved')}：${p.toLowerCase()}`
  return ''
}

function ForceEntrySetup({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const [path, setPath] = useState(() => Math.random().toString(36).slice(2, 12))
  const [decoy, setDecoy] = useState<'404' | 'dino'>('404')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const pathError = entryPathHint(path, t)
  async function save(e: React.FormEvent) {
    e.preventDefault()
    if (pathError) { setError(pathError); return }
    setBusy(true); setError('')
    try {
      const v = await post<Me & {ok?: boolean}>('/settings/entry', {path, decoy_mode: decoy})
      setMe({...me, must_set_entry: false, entry_path: v.entry_path || path, decoy_mode: v.decoy_mode || decoy, entry_url: v.entry_url || ('/' + path)})
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  return (
    <div className="modal-back">
      <form className="modal entry-force-modal" onSubmit={save}>
        <LockKeyhole />
        <h2>{t('entryForceTitle')}</h2>
        <p className="form-hint">{t('entryForceHint')}</p>
        <label>{t('entryPath')}
          <input value={path} onChange={e => { setPath(e.target.value); setError('') }} maxLength={64} required />
          <small className={pathError ? 'field-error' : ''}>{pathError || t('entryPathHint')}</small>
        </label>
        <label>{t('decoyMode')}
          <select value={decoy} onChange={e => setDecoy(e.target.value as '404' | 'dino')}>
            <option value="404">{t('decoy404')}</option>
            <option value="dino">{t('decoyDino')}</option>
          </select>
        </label>
        {!pathError && path && <div className="entry-preview">{t('entryPreview')}: <code>{location.origin}/{path.replace(/^\/+|\/+$/g, '')}</code></div>}
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={busy || !!pathError}>{busy ? '…' : t('saveEntry')}</button>
      </form>
    </div>
  )
}

/* —— Dashboard —— */
type Overview = {snapshot: Snapshot; services: Service[] | null; containers: Container[] | null; insecure_http: boolean}
function Dashboard({goSettings}: {goSettings: () => void}) {
  const {t} = useI18n()
  const [data, setData] = useState<Overview | null>(() => cached<Overview | null>('overview', null))
  const [history, setHistory] = useState<Snapshot[]>(() => cached<Snapshot[]>('metrics24h', []))
  const [error, setError] = useState(''), [tick, setTick] = useState(0)
  useEffect(() => {
    let cancelled = false
    Promise.all([api<Overview>('/overview'), api<Snapshot[]>('/metrics/history?hours=24').catch(() => [] as Snapshot[])])
      .then(([ov, hist]) => {
        if (cancelled) return
        const next = {...ov, services: Array.isArray(ov.services) ? ov.services : [], containers: Array.isArray(ov.containers) ? ov.containers : []}
        const points = Array.isArray(hist) ? [...hist].reverse() : []
        setData(next); setHistory(points); cache('overview', next); cache('metrics24h', points)
      })
      .catch(e => { if (!cancelled) setError((e as Error).message) })
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/metrics`)
    ws.onmessage = e => {
      try {
        const m = JSON.parse(e.data) as Snapshot
        setData(v => { const next = v ? {...v, snapshot: m} : v; if (next) cache('overview', next); return next })
        setHistory(v => {
          const last = v[v.length - 1]
          const sameMinute = last && Math.floor(new Date(last.time).getTime() / 60000) === Math.floor(new Date(m.time).getTime() / 60000)
          const next = (sameMinute ? [...v.slice(0, -1), m] : [...v, m]).filter(x => new Date(x.time).getTime() >= Date.now() - 24 * 60 * 60 * 1000).slice(-1441)
          cache('metrics24h', next); return next
        })
      } catch { /* */ }
    }
    return () => { cancelled = true; ws.close() }
  }, [tick])
  if (error && !data) return <><PageHead title={t('dashboard')} /><div className="page-body"><div className="error banner">{error} <button className="btn" onClick={() => setTick(n => n + 1)}>{t('retry')}</button></div></div></>
  if (!data) return <><PageHead title={t('dashboard')} /><div className="page-body"><Loading /></div></>
  const m = data.snapshot || ({} as Snapshot)
  const mem = pct(m.memory_used, m.memory_total), disk = pct(m.disk_used, m.disk_total)
  const containers = data.containers || [], services = (data.services || []).filter(s => s.installed)
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
          <div className="panel wide"><PanelTitle title={t('performance')} /><Spark data={history} empty={t('collecting')} /></div>
          <div className="panel"><PanelTitle title={t('serviceHealth')} />
            <div className="service-list">
              {services.filter(s => s.name !== 'compose').map(s => <div key={s.name}><span className={`dot ${statusOk(s.status) ? 'ok' : ''}`} /><div><strong>{s.display_name || s.name}</strong><small>{s.version || s.path}</small></div><em>{statusLabel(s.status, t)}</em></div>)}
              {!services.length && <div className="empty">{t('noData')}</div>}
            </div>
          </div>
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
function Spark({data, empty}: {data: Snapshot[]; empty: string}) {
  const {t} = useI18n()
  const [hover, setHover] = useState<number | null>(null)
  if (data.length < 2) return <div className="empty">{empty}</div>
  const values = data.map(x => x.cpu_percent || 0), max = Math.max(100, ...values), w = 800, h = 200
  const end = Date.now(), start = end - 24 * 60 * 60 * 1000
  const xAt = (item: Snapshot) => Math.max(0, Math.min(w, ((new Date(item.time).getTime() - start) / (end - start)) * w))
  const points = data.map((item, i) => `${xAt(item)},${h - (values[i] / max) * h}`).join(' ')
  const active = hover == null ? null : data[hover]
  const x = hover == null ? 0 : xAt(data[hover])
  return (
    <div className="chart-wrap">
      <svg className="chart" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" onMouseLeave={() => setHover(null)} onMouseMove={e => { const r = e.currentTarget.getBoundingClientRect(); const target = start + ((e.clientX - r.left) / r.width) * (end - start); let best = 0; data.forEach((item, i) => { if (Math.abs(new Date(item.time).getTime() - target) < Math.abs(new Date(data[best].time).getTime() - target)) best = i }); setHover(best) }}>
        <defs><linearGradient id="fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#20a53a" stopOpacity=".28" /><stop offset="1" stopColor="#20a53a" stopOpacity="0" /></linearGradient></defs>
        <polygon points={`0,${h} ${points} ${w},${h}`} fill="url(#fill)" />
        <polyline points={points} fill="none" stroke="#20a53a" strokeWidth="2.5" vectorEffect="non-scaling-stroke" />
        {active && <><line x1={x} x2={x} y1="0" y2={h} className="chart-guide" vectorEffect="non-scaling-stroke" /><circle cx={x} cy={h - ((active.cpu_percent || 0) / max) * h} r="5" className="chart-point" vectorEffect="non-scaling-stroke" /></>}
      </svg>
      <div className="chart-axis"><span>{new Date(start).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'})}</span><span>-12h</span><span>{t('now')}</span></div>
      {active && <div className="chart-tip" style={{left: `${Math.max(10, Math.min(90, (x / w) * 100))}%`}}><strong>{new Date(active.time).toLocaleString()}</strong><span>CPU {num(active.cpu_percent)}%</span><span>{t('memory')} {num(pct(active.memory_used, active.memory_total))}%</span><span>Load {num(active.load1)}</span></div>}
    </div>
  )
}

/* —— Docker —— */
function DockerPage() {
  const {t} = useI18n()
  const [items, setItems] = useState<Container[]>(() => cached<Container[]>('containers', [])), [terminal, setTerminal] = useState<Container | null>(null)
  const [error, setError] = useState(''), [customMessage, setCustomMessage] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Container | null>(null)
  const [customProject, setCustomProject] = useState(false)
  const load = () => api<Container[]>('/docker/containers').then(v => { const next = Array.isArray(v) ? v : []; setItems(next); cache('containers', next); setError('') }).catch(e => setError(e.message))
  useEffect(() => { void load() }, [])
  async function act(c: Container, verb: string) {
    if (verb === 'delete') { setPendingDelete(c); return }
    await post('/actions', {kind: `docker.container.${verb}`, resource: c.id, options: {}})
    setTimeout(load, 800)
  }
  async function confirmDeleteContainer() {
    if (!pendingDelete) return
    const c = pendingDelete
    setPendingDelete(null)
    await post('/actions', {kind: 'docker.container.delete', resource: c.id, options: {}})
    setTimeout(load, 800)
  }
  return (
    <>
      <PageHead title={t('docker')} action={<><button className="primary" onClick={() => setCustomProject(true)}><Plus />{t('createDockerProject')}</button><button className="btn" onClick={load}><RefreshCw />{t('refresh')}</button></>} />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        {customMessage && <div className="success" style={{marginBottom: 12}}>{customMessage}</div>}
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
        {pendingDelete && (
          <ConfirmModal
            title={t('remove')}
            message={`${t('confirmDeleteContainer')}\n${pendingDelete.names?.[0]?.replace('/', '') || pendingDelete.id.slice(0, 12)}`}
            confirmLabel={t('remove')}
            danger
            onCancel={() => setPendingDelete(null)}
            onConfirm={() => void confirmDeleteContainer()}
          />
        )}
        {customProject && <CustomDockerDialog onClose={() => setCustomProject(false)} onQueued={() => { setCustomProject(false); setCustomMessage(t('dockerProjectQueued')); setTimeout(load, 1200) }} />}
      </div>
    </>
  )
}

function ContainerTerminal({container, onClose}: {container: Container; onClose: () => void}) {
  const {t} = useI18n()
  const [connected, setConnected] = useState(false)
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!host.current) return
    const terminal = new XTerm({cursorBlink: true, convertEol: true, fontFamily: '"Cascadia Mono", Consolas, monospace', fontSize: 13, scrollback: 5000, theme: {background: '#15171c', foreground: '#d4d4d4', cursor: '#67c77a', selectionBackground: '#3b82f655'}})
    const fit = new FitAddon()
    terminal.loadAddon(fit); terminal.open(host.current); fit.fit()
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/docker/terminal?id=${encodeURIComponent(container.id)}`)
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => { setConnected(true); terminal.focus() }
    ws.onmessage = e => terminal.write(typeof e.data === 'string' ? e.data : new Uint8Array(e.data))
    ws.onclose = () => { setConnected(false); terminal.writeln('\r\n[connection closed]') }
    const input = terminal.onData(data => { if (ws.readyState === WebSocket.OPEN) ws.send(data) })
    const resize = new ResizeObserver(() => fit.fit()); resize.observe(host.current)
    return () => { resize.disconnect(); input.dispose(); ws.close(); terminal.dispose() }
  }, [container.id])
  return (
    <div className="modal-back"><div className="modal terminal-modal">
      <div className="terminal-head">
        <span className="terminal-dots" title={t('logout')}>
          <button type="button" className="dot-close" aria-label="close" onClick={onClose} />
          <i className="dot-min" />
          <i className="dot-max" />
        </span>
        <strong>{container.names?.[0]?.replace('/', '') || container.id.slice(0, 12)}</strong>
        <em className={connected ? 'online' : ''}>{connected ? t('connected') : t('connecting')}</em>
      </div>
      <div ref={host} className="terminal-screen" />
      <small className="terminal-hint">{t('terminalHint')}</small>
    </div></div>
  )
}

function HostSSH({onClose}: {onClose: () => void}) {
  const {t} = useI18n()
  const [connected, setConnected] = useState(false)
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!host.current) return
    const terminal = new XTerm({cursorBlink: true, convertEol: true, fontFamily: '"Cascadia Mono", Consolas, monospace', fontSize: 13, scrollback: 10000, theme: {background: '#15171c', foreground: '#d4d4d4', cursor: '#67c77a', selectionBackground: '#3b82f655'}})
    const fit = new FitAddon()
    terminal.loadAddon(fit); terminal.open(host.current); fit.fit()
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/host/terminal`)
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => { setConnected(true); terminal.focus() }
    ws.onmessage = e => terminal.write(typeof e.data === 'string' ? e.data : new Uint8Array(e.data))
    ws.onclose = () => { setConnected(false); terminal.writeln(`\r\n[${t('connectionClosed')}]`) }
    const input = terminal.onData(data => { if (ws.readyState === WebSocket.OPEN) ws.send(data) })
    const resize = new ResizeObserver(() => fit.fit()); resize.observe(host.current)
    return () => { resize.disconnect(); input.dispose(); ws.close(); terminal.dispose() }
  }, [t])
  return <>
    <PageHead title={t('ssh')} hint={t('sshHint')} />
    <div className="page-body ssh-page">
      <div className="terminal-modal host-terminal">
        <div className="terminal-head host-terminal-head"><strong>root@localhost</strong><em className={connected ? 'online' : ''}>{connected ? t('connected') : t('connecting')}</em><button className="host-terminal-close" onClick={onClose}>{t('close')}</button></div>
        <div ref={host} className="terminal-screen" />
      </div>
    </div>
  </>
}

/* —— Websites (BT-style) —— */
function Websites() {
  const {t} = useI18n()
  const [tab, setTab] = useState<'sites' | 'certs'>('sites')
  const [items, setItems] = useState<Website[]>(() => cached<Website[]>('websites', [])), [certs, setCerts] = useState<Certificate[]>(() => cached<Certificate[]>('certificates', []))
  const [settingsSite, setSettingsSite] = useState<Website | null>(null)
  const [wizard, setWizard] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  const loadSites = () => api<Website[]>('/websites').then(v => { const next = Array.isArray(v) ? v : []; setItems(next); cache('websites', next); setError('') }).catch(e => setError(e.message))
  const loadCerts = () => api<Certificate[]>('/certificates').then(v => { const next = Array.isArray(v) ? v : []; setCerts(next); cache('certificates', next); setError('') }).catch(e => setError(e.message))
  const load = () => { void loadSites(); void loadCerts() }
  useEffect(() => { load() }, [])

  async function renew(domain = '', force = false) {
    await post('/actions', {kind: 'cert.renew', resource: domain, options: {force: force ? 'true' : 'false'}})
    setMessage(t('renewTask')); setTimeout(loadCerts, 2000)
  }
  async function deleteCert(cert: Certificate) {
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    try {
      await post('/actions', {kind: 'cert.delete', resource: cert.domain, options: {source: cert.source}})
      setMessage(t('certDeleted')); setTimeout(loadCerts, 800)
    } catch (e) { setError((e as Error).message) }
  }
  function protoLabel(s: Website) {
    if (s.has_http && s.has_https) return t('bothHttpHttps')
    if (s.has_https || s.tls) return t('tlsOn')
    return t('tlsOff')
  }

  return (
    <>
      <PageHead title={t('websites')} action={<div className="toolbar">
        {tab === 'sites' && <button className="primary add-site-btn" onClick={() => setWizard(true)}><Plus size={16} />{t('addSite')}</button>}
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
          <div className="panel table-panel website-table">
            <table className="site-table">
              <thead><tr><th>{t('domainLabel')}</th><th>{t('siteType')}</th><th>{t('siteDirectory')}</th><th>{t('status')}</th><th>SSL</th><th>{t('actions')}</th></tr></thead>
              <tbody>
                {items.map(s => (
                  <tr key={s.id} className="site-row" onClick={() => setSettingsSite(s)}>
                    <td>
                      <strong className="site-domain"><span className={`dot ${s.enabled ? 'ok' : ''}`} />{s.domains?.join(' ') || s.name}</strong>
                      <div style={{fontSize: 12, color: 'var(--muted)', marginTop: 2}}>{s.listen?.join(', ') || t('clickToSettings')}</div>
                    </td>
                    <td><span className={`server-tag ${s.server === 'apache' ? 'apache' : ''}`}>{s.server}</span><small className="site-kind">{s.proxy_target ? t('siteTypeProxy') : t('siteTypeStatic')}</small></td>
                    <td style={{maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>{s.proxy_target || s.doc_root || t('staticSite')}</td>
                    <td><span className={`pill ${s.enabled ? 'green' : ''}`}>{s.enabled ? t('active') : t('inactive')}</span></td>
                    <td><span className={`pill ${(s.has_https || s.tls) ? 'green' : ''}`}>{protoLabel(s)}</span></td>
                    <td onClick={e => e.stopPropagation()}>
                      <div className="site-ops">
                        <button className="primary" onClick={() => setSettingsSite(s)}>{t('siteSettings')}</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!items.length && !error && <div className="empty">{t('noData')}</div>}
          </div>
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
                        <button className="danger" title={t('remove')} onClick={() => deleteCert(c)}><Trash2 /></button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
            {!certs.length && !error && <div className="empty">{t('noData')}</div>}
          </div>
        )}

        {settingsSite && (
          <SiteSettings
            site={settingsSite}
            onClose={() => setSettingsSite(null)}
            onChanged={() => { setMessage(t('settingsSaved')); setTimeout(load, 1200) }}
            onDeleted={() => { setSettingsSite(null); setMessage(t('success')); setTimeout(loadSites, 1200) }}
          />
        )}
        {wizard && <SiteWizard onClose={() => setWizard(false)} onCreated={() => { setWizard(false); setMessage(t('siteCreated')); setTimeout(load, 1500) }} />}
      </div>
    </>
  )
}

type SiteTab = 'basic' | 'proxy' | 'rewrite' | 'ssl' | 'config' | 'danger'

function SiteSettings({site, onClose, onChanged, onDeleted}: {
  site: Website
  onClose: () => void
  onChanged: () => void
  onDeleted: () => void
}) {
  const {t} = useI18n()
  const managed = site.source_path.includes('anpanel-site-')
  const domain = site.domains?.[0] || site.name
  const [tab, setTab] = useState<SiteTab>('basic')
  const [siteType, setSiteType] = useState<'static' | 'proxy'>(site.proxy_target ? 'proxy' : 'static')
  const [root, setRoot] = useState(site.doc_root || `/var/www/${domain}`)
  const [proxyPass, setProxyPass] = useState(site.proxy_target || 'http://127.0.0.1:3000')
  const [rewrite, setRewrite] = useState('none')
  const [rules, setRules] = useState<RewriteRule[]>([])
  const [config, setConfig] = useState('')
  const [tool, setTool] = useState('certbot')
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api<RewriteRule[]>('/rewrite-rules').then(v => setRules(Array.isArray(v) ? v : [])).catch(() => {})
    api<{content: string}>(`/websites/config?path=${encodeURIComponent(site.source_path)}`)
      .then(r => setConfig(r.content || ''))
      .catch(e => setError(e.message))
  }, [site.source_path])

  const preview = rules.find(r => r.id === rewrite)
  const previewText = site.server === 'apache' ? preview?.apache : preview?.nginx

  async function saveBasic(nextType: 'static' | 'proxy' = siteType) {
    if (!managed) { setError(t('managedOnly')); return }
    setBusy(true); setError(''); setMessage('')
    try {
      setSiteType(nextType)
      await post('/actions', {
        kind: 'web.site.configure',
        resource: domain,
        options: {
          domain, server: site.server, site_type: nextType, rewrite,
          root: nextType === 'static' ? root : '',
          proxy_pass: nextType === 'proxy' ? proxyPass : '',
        },
      })
      setMessage(t('settingsSaved')); onChanged()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  async function saveRewrite() {
    if (!managed) { setError(t('managedOnly')); return }
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'web.site.rewrite', resource: domain, options: {rewrite, server: site.server}})
      setMessage(t('settingsSaved')); onChanged()
      const r = await api<{content: string}>(`/websites/config?path=${encodeURIComponent(site.source_path)}`)
      setConfig(r.content || '')
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  async function saveConfig() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'web.apply', resource: site.source_path, options: {content: config}})
      setMessage(t('settingsSaved')); onChanged()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  async function issueSSL() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'cert.issue', resource: domain, options: {tool, email, server: site.server}})
      setMessage(t('issueTask')); onChanged()
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }

  async function removeSite() {
    if (!managed) { setError(t('onlyManaged')); return }
    if (prompt(t('confirmDelete')) !== 'DELETE') return
    setBusy(true)
    try {
      await post('/actions', {kind: 'web.site.delete', resource: domain, options: {server: site.server}})
      onDeleted()
    } catch (e) { setError((e as Error).message); setBusy(false) }
  }

  const tabs: {id: SiteTab; label: string}[] = [
    {id: 'basic', label: t('tabBasic')},
    {id: 'proxy', label: t('tabProxy')},
    {id: 'rewrite', label: t('tabRewrite')},
    {id: 'ssl', label: t('tabSSL')},
    {id: 'config', label: t('tabConfig')},
    {id: 'danger', label: t('tabDanger')},
  ]

  return (
    <div className="modal-back site-settings-back">
      <div className="site-settings-panel">
        <header className="site-settings-head">
          <div>
            <h2>{t('siteSettingsTitle')}</h2>
            <p>{domain} · {site.server} · {site.source_path}</p>
          </div>
          <button className="close" onClick={onClose}>×</button>
        </header>
        <div className="site-settings-body">
          <nav className="site-settings-nav">
            {tabs.map(x => (
              <button key={x.id} className={tab === x.id ? 'active' : ''} onClick={() => setTab(x.id)}>{x.label}</button>
            ))}
          </nav>
          <div className="site-settings-main">
            {!managed && tab !== 'config' && tab !== 'ssl' && (
              <div className="warning" style={{marginBottom: 12}}>{t('managedOnly')}</div>
            )}
            {error && <div className="error banner">{error}</div>}
            {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}

            {tab === 'basic' && (
              <div className="site-settings-form">
                <label>{t('domainLabel')}<input value={domain} disabled /></label>
                <label>{t('listenPorts')}<input value={site.listen?.join(', ') || '-'} disabled /></label>
                <label>{t('siteEnabled')}<input value={site.enabled ? t('active') : t('inactive')} disabled /></label>
                <label>{t('siteType')}
                  <select value={siteType} disabled={!managed} onChange={e => setSiteType(e.target.value as 'static' | 'proxy')}>
                    <option value="static">{t('siteTypeStatic')}</option>
                    <option value="proxy">{t('siteTypeProxy')}</option>
                  </select>
                </label>
                {siteType === 'static' ? (
                  <label className="full">{t('docRoot')}<input value={root} disabled={!managed} onChange={e => setRoot(e.target.value)} /><small>{t('docRootHint')}</small></label>
                ) : (
                  <label className="full">{t('proxyPass')}<input value={proxyPass} disabled={!managed} onChange={e => setProxyPass(e.target.value)} /><small>{t('proxyPassHint')}</small></label>
                )}
                {managed && (
                  <div className="card-actions">
                    <button className="primary" disabled={busy} onClick={() => void saveBasic()}>{busy ? '…' : t('saveSettings')}</button>
                  </div>
                )}
              </div>
            )}

            {tab === 'proxy' && (
              <div className="site-settings-form">
                <p className="form-hint">{t('siteTypeProxyHint')}</p>
                <label className="full">{t('proxyPass')}<input value={proxyPass} disabled={!managed} onChange={e => setProxyPass(e.target.value)} placeholder="http://127.0.0.1:3000" /></label>
                {managed && (
                  <div className="card-actions">
                    <button className="primary" disabled={busy} onClick={() => void saveBasic('proxy')}>{busy ? '…' : t('saveSettings')}</button>
                  </div>
                )}
              </div>
            )}

            {tab === 'rewrite' && (
              <div className="site-settings-form">
                <p className="form-hint">{t('rewriteHint')}</p>
                <label className="full">{t('rewrite')}
                  <select value={rewrite} disabled={!managed} onChange={e => setRewrite(e.target.value)}>
                    {(rules.length ? rules : [{id: 'none', name: 'none', description: '', nginx: '', apache: ''} as RewriteRule]).map(r => (
                      <option key={r.id} value={r.id}>{r.name}{r.description ? ` — ${r.description}` : ''}</option>
                    ))}
                  </select>
                </label>
                {previewText && (
                  <label className="full">{t('rewritePreview')}<textarea className="rewrite-preview" readOnly value={previewText} rows={10} /></label>
                )}
                {managed && (
                  <div className="card-actions">
                    <button className="primary" disabled={busy} onClick={saveRewrite}>{busy ? '…' : t('saveSettings')}</button>
                  </div>
                )}
              </div>
            )}

            {tab === 'ssl' && (
              <div className="site-settings-form">
                <div className={`ssl-status-card ${(site.has_https || site.tls) ? 'ok' : ''}`}>
                  <FileKey2 size={22} />
                  <div>
                    <strong>{t('sslStatus')}</strong>
                    <p>{(site.has_https || site.tls) ? t('sslOk') : t('sslNone')}</p>
                  </div>
                  <span className={`pill ${(site.has_https || site.tls) ? 'green' : ''}`}>{(site.has_https || site.tls) ? t('tlsOn') : t('tlsOff')}</span>
                </div>
                <label>{t('acmeTool')}<select value={tool} onChange={e => setTool(e.target.value)}><option value="certbot">certbot</option><option value="acme.sh">acme.sh</option></select></label>
                <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>
                <div className="steps"><ol><li>{t('step1')}</li><li>{t('step2')}</li><li>{t('step3')}</li></ol></div>
                <div className="card-actions">
                  <button className="primary" disabled={busy || !domain} onClick={issueSSL}>{busy ? '…' : t('issueSSL')}</button>
                </div>
              </div>
            )}

            {tab === 'config' && (
              <div className="site-settings-form config-tab">
                <small>{t('source')}: {site.source_path}</small>
                <textarea className="config-editor" spellCheck={false} value={config} onChange={e => setConfig(e.target.value)} />
                <div className="card-actions">
                  <button className="primary" disabled={busy} onClick={saveConfig}>{busy ? '…' : t('apply')}</button>
                </div>
              </div>
            )}

            {tab === 'danger' && (
              <div className="site-settings-form">
                <div className="danger-box">
                  <h3>{t('deleteSite')}</h3>
                  <p>{t('onlyManaged')}</p>
                  <button className="primary" style={{background: 'var(--danger)', borderColor: 'var(--danger)'}} disabled={busy || !managed} onClick={removeSite}>
                    {t('deleteSite')}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function SiteWizard({onClose, onCreated}: {onClose: () => void; onCreated: () => void}) {
  const {t} = useI18n()
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
  const valid = domain.trim().includes('.') && (siteType === 'static' || proxyPass.trim().startsWith('http'))
  return (
    <div className="modal-back"><div className="modal wizard site-create-modal">
      <button className="close" onClick={onClose}>×</button>
      <h2>{t('siteWizard')}</h2>
      <p className="form-hint">{t('siteCreateHint')}</p>
      <div className="site-type-tabs"><button className={siteType === 'proxy' ? 'active' : ''} onClick={() => setSiteType('proxy')}>{t('siteTypeProxy')}</button><button className={siteType === 'static' ? 'active' : ''} onClick={() => setSiteType('static')}>{t('siteTypeStatic')}</button></div>
      <div className="site-form-grid">
        <label className="full">{t('domainLabel')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="example.com" autoFocus /><small>{t('domainHintSite')}</small></label>
        {siteType === 'proxy' ? <label className="full">{t('proxyPass')}<input value={proxyPass} onChange={e => setProxyPass(e.target.value)} /><small>{t('proxyPassHint')}</small></label> : <label className="full">{t('docRoot')}<input value={root} onChange={e => setRoot(e.target.value)} placeholder={`/var/www/${domain || 'example.com'}`} /><small>{t('docRootHint')}</small></label>}
        {siteType === 'static' && <label>{t('rewrite')}<select value={rewrite} onChange={e => setRewrite(e.target.value)}>{(rules.length ? rules : [{id: 'none', name: 'none', description: '', nginx: '', apache: ''}]).map(r => <option key={r.id} value={r.id}>{r.name}</option>)}</select></label>}
        <label className="ssl-switch"><input type="checkbox" checked={enableSSL} onChange={e => setEnableSSL(e.target.checked)} /><span><strong>{t('enableSSL')}</strong><small>{t('sslCreateHint')}</small></span></label>
        {enableSSL && <><label>{t('acmeTool')}<select value={tool} onChange={e => setTool(e.target.value)}><option value="certbot">certbot</option><option value="acme.sh">acme.sh</option></select></label><label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label></>}
      </div>
      {error && <div className="error">{error}</div>}
      <div className="card-actions modal-actions">
        <button type="button" className="btn" onClick={onClose}>{t('cancel')}</button>
        <button type="button" className="primary" disabled={busy || !valid} onClick={submit}>{busy ? '…' : t('createSite')}</button>
      </div>
    </div></div>
  )
}

/* —— Files —— */
function FilesPage() {
  const {t} = useI18n()
  const [path, setPath] = useState('/'), [address, setAddress] = useState('/')
  const [items, setItems] = useState<FileEntry[]>([])
  const [error, setError] = useState(''), [message, setMessage] = useState('')
  const [edit, setEdit] = useState<{path: string; content: string} | null>(null)

  const load = (p = path) => api<FileEntry[]>(`/files?path=${encodeURIComponent(p)}`)
    .then(v => { setItems(Array.isArray(v) ? v : []); setPath(p); setAddress(p); setError('') })
    .catch(e => setError(e.message))

  useEffect(() => { void load('/') }, [])

  function parentOf(p: string) {
    const norm = p.replace(/\\/g, '/').replace(/\/$/, '') || '/'
    if (norm === '/') return '/'
    const parts = norm.split('/').filter(Boolean)
    return parts.length <= 1 ? '/' : '/' + parts.slice(0, -1).join('/')
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

  const crumbs = path.split('/').filter(Boolean)

  return (
    <>
      <PageHead title={t('filesTitle')} action={<div className="toolbar">
        <button className="btn" onClick={newFolder}><Plus size={14} />{t('newFolder')}</button>
        <button className="btn" onClick={newFile}><FileText size={14} />{t('newFile')}</button>
        <button className="btn" onClick={() => load()}><RefreshCw />{t('refresh')}</button>
      </div>} />
      <div className="page-body">
        <div className="file-browser-bar">
          <button className="icon-btn" title={t('parentDir')} disabled={path === '/'} onClick={() => load(parentOf(path))}><ChevronUp /></button>
          <div className="breadcrumbs">
            <button onClick={() => load('/')}><Home /></button>
            {crumbs.map((part, i) => <React.Fragment key={`${part}-${i}`}><ChevronRight /><button onClick={() => load('/' + crumbs.slice(0, i + 1).join('/'))}>{part}</button></React.Fragment>)}
          </div>
          <form onSubmit={e => { e.preventDefault(); void load(address.trim() || '/') }}><input aria-label={t('path')} value={address} onChange={e => setAddress(e.target.value)} /></form>
        </div>
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        <div className="panel table-panel file-panel">
          <table className="file-table">
            <thead><tr><th>{t('path')}</th><th>{t('size')}</th><th>{t('permissions')}</th><th>{t('modified')}</th><th /></tr></thead>
            <tbody>
              {items.map(f => (
                <tr key={f.path}>
                  <td>
                    <button className="file-name" onClick={() => openFile(f)}>
                      <span className={f.is_dir ? 'folder' : ''}>{f.is_dir ? <FolderOpen /> : <FileText />}</span>
                      {f.name}
                    </button>
                  </td>
                  <td>{f.is_dir ? '-' : bytes(f.size)}</td>
                  <td><code className="file-mode">{f.mode}</code></td>
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

/* —— App store —— */
function Services() {
  const {t} = useI18n()
  const [items, setItems] = useState<Service[]>(() => cached<Service[]>('services', [])), [error, setError] = useState(''), [message, setMessage] = useState('')
  const [installDlg, setInstallDlg] = useState<Service | null>(null)
  const [progress, setProgress] = useState<{title: string; taskId: string} | null>(null)
  const [bindDlg, setBindDlg] = useState<Service | null>(null)
  const [query, setQuery] = useState('')
  const load = () => api<Service[]>('/services').then(v => { const next = Array.isArray(v) ? v : []; setItems(next); cache('services', next); setError('') }).catch(e => setError(e.message))
  useEffect(() => { void load() }, [])
  async function act(s: Service, verb: string) {
    let resource = s.name
    if (s.name === 'apache') resource = s.path?.includes('apache2') ? 'apache2' : 'httpd'
    if (s.name === 'php') resource = 'php-fpm'
    await post('/actions', {kind: `service.${verb}`, resource, options: {}}); setTimeout(load, 600)
  }
  async function doUpdate(s: Service) {
    try {
      if (s.deploy === 'docker') {
        const r = await post<{task_id: string}>('/actions', {kind: 'package.update', resource: s.name, options: {deploy: 'docker', version: s.versions?.[s.versions.length - 2] || s.versions?.[0] || '', host_port: s.host_port || ''}})
        setProgress({title: `${t('updateSoft')} ${s.display_name || s.name}`, taskId: r.task_id})
      } else {
        const method = s.name === 'docker' ? '' : (s.default_method || 'source')
        await post('/actions', {kind: 'package.update', resource: s.name, options: method ? {method} : {}})
        setMessage(t('updateQueued'))
      }
    } catch (e) { setError((e as Error).message) }
  }
  function openInstall(s: Service) {
    // Always open dialog; conflict is shown inside for confirmation UX.
    setInstallDlg(s)
  }
  const q = query.trim().toLowerCase()
  const systemApps = items
    .filter(s => s.name !== 'compose')
    .filter(s => !q || `${s.name} ${s.display_name || ''}`.toLowerCase().includes(q))
    .sort((a, b) => Number(b.name === 'docker') - Number(a.name === 'docker'))
  return (
    <>
      <PageHead title={t('services')} hint={t('appStoreHint')} action={<button className="btn" onClick={load}><RefreshCw />{t('refresh')}</button>} />
      <div className="page-body">
        {error && <div className="error banner">{error}</div>}
        {message && <div className="success" style={{marginBottom: 12}}>{message}</div>}
        <div className="market-tools">
          <div className="market-search"><Search /><input value={query} onChange={e => setQuery(e.target.value)} placeholder={t('searchApps')} /></div>
        </div>
        <section className="market-section">
          <h2>{t('systemApps')}<small>{t('systemAppsHint')}</small></h2>
          <div className="app-grid">
            {systemApps.map(s => (
                  <div className={`panel service-card ${s.name === 'docker' ? 'docker-app' : ''}`} key={s.name}>
                    <div className="resource">
                      <span className="cube"><ServerCog /></span>
                      <div>
                        <h3>{s.display_name || s.name}</h3>
                        <small>{s.version || s.path || t('missing')}</small>
                      </div>
                    </div>
                    <div style={{display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'flex-end'}}>
                      {s.deploy === 'docker' && <span className="pill">{t('deployDockerBadge')}</span>}
                      <span className={`pill ${statusOk(s.status) ? 'green' : ''}`}>{statusLabel(s.status, t)}</span>
                    </div>
                    {s.name === 'docker' && <p className="docker-app-hint">{s.note || t('dockerInstallHint')}</p>}
                    {s.name !== 'docker' && s.note && <p style={{gridColumn: '1/-1', margin: 0, fontSize: 12, color: 'var(--muted)'}}>{s.note}</p>}
                    <div className="card-actions">
                      {s.installed && s.deploy !== 'docker' && ['nginx', 'apache', 'docker'].includes(s.name) && (
                        <>
                          <button className="btn" onClick={() => act(s, (s.status === 'active' || s.status === 'running') ? 'stop' : 'start')}>{(s.status === 'active' || s.status === 'running') ? t('stop') : t('start')}</button>
                          <button className="btn" onClick={() => act(s, 'restart')}>{t('restart')}</button>
                        </>
                      )}
                      {s.can_update && <button className="btn" onClick={() => doUpdate(s)}>{t('updateSoft')}</button>}
                      {s.installed && s.deploy === 'docker' && s.host_port && s.name !== 'php' && <button className="btn" onClick={() => setBindDlg(s)}><Link2 />{t('bindWeb')}</button>}
                      {!s.installed && (s.can_install || s.block_reason) && (
                        <button className="primary" onClick={() => openInstall(s)}>{s.deploy === 'docker' ? t('deployDockerApp') : t('installSoft')}</button>
                      )}
                    </div>
                  </div>
            ))}
          </div>
        </section>
        {!systemApps.length && <div className="empty">{t('noData')}</div>}
        {installDlg && (
          <InstallDialog
            service={installDlg}
            onClose={() => setInstallDlg(null)}
            onQueued={(taskId, title) => {
              setInstallDlg(null)
              if (taskId) setProgress({title, taskId})
              else { setMessage(t('installQueued')); setTimeout(load, 1500) }
            }}
          />
        )}
        {bindDlg && <DockerWebDialog service={bindDlg} onClose={() => setBindDlg(null)} onQueued={() => { setBindDlg(null); setMessage(t('webBindQueued')) }} />}
        {progress && (
          <TaskProgressModal
            title={progress.title}
            taskId={progress.taskId}
            onClose={() => { setProgress(null); void load() }}
          />
        )}
      </div>
    </>
  )
}

function queueWebBinding(domain: string, hostPort: string, ssl: boolean, email: string) {
  return post('/actions', {kind: 'web.site.create', resource: domain, options: {domain, site_type: 'proxy', proxy_pass: `http://127.0.0.1:${hostPort}`, enable_ssl: ssl ? 'true' : 'false', email, tool: 'certbot'}})
}

function DockerWebDialog({service, onClose, onQueued}: {service: Service; onClose: () => void; onQueued: () => void}) {
  const {t} = useI18n()
  const [domain, setDomain] = useState(''), [ssl, setSSL] = useState(true), [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function submit() {
    setBusy(true); setError('')
    try {
      await queueWebBinding(domain, service.host_port || '', ssl, email)
      onQueued()
    } catch (e) { setError((e as Error).message); setBusy(false) }
  }
  return <div className="modal-back"><div className="modal">
    <button className="close" onClick={onClose}>×</button>
    <h2>{t('bindWeb')} · {service.display_name || service.name}</h2>
    <p className="form-hint">{t('bindWebHint')} <code>127.0.0.1:{service.host_port}</code></p>
    <label>{t('domainLabel')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="app.example.com" /></label>
    <label className="check-row"><input type="checkbox" checked={ssl} onChange={e => setSSL(e.target.checked)} />{t('enableSSL')}</label>
    {ssl && <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>}
    {error && <div className="error">{error}</div>}
    <div className="card-actions"><button className="btn" onClick={onClose}>{t('cancel')}</button><button className="primary" disabled={busy || !domain.includes('.')} onClick={submit}>{busy ? '…' : t('bindWeb')}</button></div>
  </div></div>
}

function CustomDockerDialog({onClose, onQueued}: {onClose: () => void; onQueued: () => void}) {
  const {t} = useI18n()
  const [image, setImage] = useState(''), [name, setName] = useState(''), [hostPort, setHostPort] = useState('8080'), [containerPort, setContainerPort] = useState('80')
  const [domain, setDomain] = useState(''), [ssl, setSSL] = useState(true), [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function submit() {
    setBusy(true); setError('')
    try {
      await post('/actions', {kind: 'package.install', resource: 'custom', options: {deploy: 'docker', image, container_name: name, host_port: hostPort, container_port: containerPort}})
      if (domain) await queueWebBinding(domain, hostPort, ssl, email)
      onQueued()
    } catch (e) { setError((e as Error).message); setBusy(false) }
  }
  return <div className="modal-back"><div className="modal">
    <button className="close" onClick={onClose}>×</button><h2>{t('createDockerProject')}</h2>
    <label>{t('dockerImage')}<input value={image} onChange={e => setImage(e.target.value)} placeholder="nginx:alpine" /></label>
    <label>{t('containerName')}<input value={name} onChange={e => setName(e.target.value)} placeholder="my-app" /></label>
    <div className="port-fields"><label>{t('hostPort')}<input inputMode="numeric" value={hostPort} onChange={e => setHostPort(e.target.value)} /></label><label>{t('containerPort')}<input inputMode="numeric" value={containerPort} onChange={e => setContainerPort(e.target.value)} /></label></div>
    <label>{t('bindDomainOptional')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="app.example.com" /></label>
    {domain && <label className="check-row"><input type="checkbox" checked={ssl} onChange={e => setSSL(e.target.checked)} />{t('enableSSL')}</label>}
    {domain && ssl && <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>}
    {error && <div className="error">{error}</div>}
    <div className="card-actions"><button className="btn" onClick={onClose}>{t('cancel')}</button><button className="primary" disabled={busy || !image || !name || !hostPort || !containerPort} onClick={submit}>{busy ? '…' : t('create')}</button></div>
  </div></div>
}

function InstallDialog({service, onClose, onQueued}: {service: Service; onClose: () => void; onQueued: (taskId: string, title: string) => void}) {
  const {t} = useI18n()
  const methods = service.install_methods?.length ? service.install_methods : ['source']
  const [method, setMethod] = useState(service.default_method || methods[0] || 'source')
  const [version, setVersion] = useState(service.versions?.[service.versions.length - 2] || service.versions?.[0] || '8.3')
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const isDocker = service.deploy === 'docker'
  const blocked = !!service.block_reason
  const [hostPort, setHostPort] = useState(service.host_port || '')
  const [domain, setDomain] = useState(''), [ssl, setSSL] = useState(true), [email, setEmail] = useState('')
  const webBindable = isDocker && service.name !== 'php'
  function methodLabel(m: string) {
    if (m === 'source') return t('methodSource')
    if (m === 'package') return t('methodPackage')
    if (m === 'script') return t('methodScript')
    if (m === 'snap') return t('methodSnap')
    return m
  }
  async function submit() {
    if (blocked) return
    setBusy(true); setError('')
    try {
      let r: {task_id: string}
      if (isDocker) {
        r = await post('/actions', {kind: 'package.install', resource: service.name, options: {deploy: 'docker', version, host_port: hostPort}})
        if (webBindable && domain) await queueWebBinding(domain, hostPort, ssl, email)
      } else {
        r = await post('/actions', {kind: 'package.install', resource: service.name, options: {method, version}})
      }
      onQueued(r.task_id || '', `${isDocker ? t('deployDockerApp') : t('installSoft')} ${service.display_name || service.name}`)
    } catch (e) { setError((e as Error).message); setBusy(false) }
  }
  return (
    <div className="modal-back"><div className="modal">
      <button className="close" onClick={onClose}>×</button>
      <h2>{isDocker ? t('deployDockerApp') : t('installSoft')} {service.display_name || service.name}</h2>
      {blocked && (
        <div className="error banner" style={{margin: 0}}>
          <strong>{t('conflictTitle')}</strong>
          <p style={{margin: '6px 0 0'}}>{service.block_reason}</p>
        </div>
      )}
      {!blocked && isDocker && <p style={{margin: 0, fontSize: 13, color: 'var(--muted)'}}>{t('deployDockerNote')} · <code>{service.image}</code></p>}
      {!blocked && !isDocker && (
        <label>{t('installMethod')}
          <select value={method} onChange={e => setMethod(e.target.value)}>
            {methods.map(m => <option key={m} value={m}>{methodLabel(m)}</option>)}
          </select>
        </label>
      )}
      {!blocked && service.versions?.length && (
        <label>{service.name === 'php' ? t('phpVersion') : t('version')}
          <select value={version} onChange={e => setVersion(e.target.value)}>
            {service.versions.map(v => <option key={v} value={v}>{v}</option>)}
          </select>
        </label>
      )}
      {!blocked && isDocker && (
        <label>{t('hostPort')}<input value={hostPort} onChange={e => setHostPort(e.target.value)} placeholder={service.host_port || '8080'} /></label>
      )}
      {!blocked && webBindable && <>
        <label>{t('bindDomainOptional')}<input value={domain} onChange={e => setDomain(e.target.value)} placeholder="app.example.com" /></label>
        {domain && <label className="check-row"><input type="checkbox" checked={ssl} onChange={e => setSSL(e.target.checked)} />{t('enableSSL')}</label>}
        {domain && ssl && <label>{t('email')}<input type="email" value={email} onChange={e => setEmail(e.target.value)} /></label>}
      </>}
      {!blocked && !isDocker && method === 'source' && <p style={{margin: 0, fontSize: 12, color: 'var(--muted)'}}>编译安装可能需要数分钟到数十分钟，进度请在本窗口或「计划任务」查看。</p>}
      {error && <div className="error">{error}</div>}
      <div className="card-actions">
        <button className="btn" onClick={onClose}>{t('cancel')}</button>
        {!blocked && <button className="primary" disabled={busy} onClick={submit}>{busy ? '…' : (isDocker ? t('startDeploy') : t('installSoft'))}</button>}
      </div>
    </div></div>
  )
}

function TaskProgressModal({title, taskId, onClose}: {title: string; taskId: string; onClose: () => void}) {
  const {t} = useI18n()
  const [task, setTask] = useState<Task | null>(null)
  const done = task && (task.status === 'succeeded' || task.status === 'failed' || task.status === 'rolled_back')
  useEffect(() => {
    let stop = false
    const tick = async () => {
      try {
        const list = await api<Task[]>('/tasks?limit=100')
        const hit = (Array.isArray(list) ? list : []).find(x => x.id === taskId)
        if (hit && !stop) setTask(hit)
        if (hit && (hit.status === 'succeeded' || hit.status === 'failed' || hit.status === 'rolled_back')) return
      } catch { /* ignore */ }
      if (!stop) setTimeout(tick, 1000)
    }
    void tick()
    return () => { stop = true }
  }, [taskId])
  const log = task?.log || t('deployWaiting')
  return (
    <div className="modal-back">
      <div className="modal deploy-progress-modal">
        <button className="close" onClick={onClose}>×</button>
        <h2>{title}</h2>
        <div className="deploy-status">
          <span className={`task-dot ${task?.status || 'running'}`} />
          <strong>{task ? taskStatusLabel(task.status, t) : t('statusRunningTask')}</strong>
          <small>{taskId.slice(0, 8)}…</small>
        </div>
        <pre className="deploy-log">{log}</pre>
        {done && task?.status === 'succeeded' && (
          <div className="success" style={{margin: 0}}>{t('deployDoneHint')}</div>
        )}
        {done && task?.status !== 'succeeded' && (
          <div className="error banner" style={{margin: 0}}>{t('deployFailedHint')}</div>
        )}
        <div className="card-actions">
          <button className="primary" onClick={onClose}>{done ? t('close') : t('runInBackground')}</button>
        </div>
      </div>
    </div>
  )
}

/* —— Tasks + Crontab —— */
function Tasks() {
  const {t, lang} = useI18n()
  const [tab, setTab] = useState<'tasks' | 'cron'>('tasks')
  const [tasks, setTasks] = useState<Task[]>([]), [audits, setAudits] = useState<Audit[]>([]), [crons, setCrons] = useState<CronJob[]>([])
  const [taskDetail, setTaskDetail] = useState<Task | null>(null)
  const [schedule, setSchedule] = useState('0 3 * * *'), [command, setCommand] = useState(''), [message, setMessage] = useState(''), [error, setError] = useState('')
  const [cronMode, setCronMode] = useState<'simple' | 'advanced'>('simple')
  const [cronUnit, setCronUnit] = useState<'minutes' | 'hours' | 'daily'>('minutes')
  const [cronEvery, setCronEvery] = useState(5), [cronTime, setCronTime] = useState('03:00')
  const simpleSchedule = cronUnit === 'minutes'
    ? `*/${Math.max(1, Math.min(59, cronEvery || 1))} * * * *`
    : cronUnit === 'hours'
      ? `0 */${Math.max(1, Math.min(23, cronEvery || 1))} * * *`
      : `${Number(cronTime.split(':')[1] || 0)} ${Number(cronTime.split(':')[0] || 0)} * * *`
  const cronSchedule = cronMode === 'simple' ? simpleSchedule : schedule

  const loadTasks = () => {
    api<Task[]>('/tasks?limit=10').then(v => setTasks(Array.isArray(v) ? v.slice(0, 10) : [])).catch(() => {})
    api<Audit[]>('/audits').then(v => setAudits(Array.isArray(v) ? v.slice(0, 10) : [])).catch(() => {})
  }
  const loadCron = () => api<CronJob[]>('/crontab').then(v => { setCrons(Array.isArray(v) ? v : []); setError('') }).catch(e => setError(e.message))

  useEffect(() => {
    loadTasks(); loadCron()
    const i = setInterval(loadTasks, 3000)
    return () => clearInterval(i)
  }, [])
  useEffect(() => { setTaskDetail(current => current ? (tasks.find(task => task.id === current.id) || current) : null) }, [tasks])

  async function addCron() {
    try {
      await post('/actions', {kind: 'crontab.add', resource: 'root', options: {schedule: cronSchedule, command}})
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
                {tasks.map(x => <div key={x.id}><span className={`task-dot ${x.status}`} /><div><strong>{taskTitle(x, lang)}</strong><small>{new Date(x.created_at).toLocaleString()} / {taskStatusLabel(x.status, t)}</small><button className="task-detail-link" onClick={() => setTaskDetail(x)}>{t('viewDetails')}</button></div></div>)}
                {!tasks.length && <div className="empty">{t('noData')}</div>}
              </div>
            </div>
            <div className="panel"><PanelTitle title={t('auditTitle')} />
              <div className="timeline">
                {audits.map(x => <div key={x.id}><span className="task-dot succeeded" /><div><strong>{taskTitle({kind: x.action, summary: x.action}, lang)}</strong><small>{new Date(x.created_at).toLocaleString()} · {x.actor} · {x.remote_ip}</small>{auditDetail(x, lang) && <p>{auditDetail(x, lang)}</p>}</div></div>)}
                {!audits.length && <div className="empty">{t('noData')}</div>}
              </div>
            </div>
          </section>
        )}

        {tab === 'cron' && (
          <div className="stack">
            <div className="panel alert-form">
              <PanelTitle title={t('addCron')} />
              <div className="cron-mode"><button className={cronMode === 'simple' ? 'active' : ''} onClick={() => setCronMode('simple')}>{t('simpleMode')}</button><button className={cronMode === 'advanced' ? 'active' : ''} onClick={() => setCronMode('advanced')}>{t('advancedMode')}</button></div>
              {cronMode === 'simple' ? <>
                <div className="cron-simple">
                  <select value={cronUnit} onChange={e => setCronUnit(e.target.value as 'minutes' | 'hours' | 'daily')}><option value="minutes">{t('minutesUnit')}</option><option value="hours">{t('hoursUnit')}</option><option value="daily">{t('dailyAt')}</option></select>
                  {cronUnit === 'daily' ? <input type="time" value={cronTime} onChange={e => setCronTime(e.target.value)} /> : <input type="number" min="1" max={cronUnit === 'minutes' ? 59 : 23} value={cronEvery} onChange={e => setCronEvery(Number(e.target.value))} />}
                </div>
                <small className="cron-note">{t('cronNoSeconds')}</small>
              </> : <><p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('cronHint')}</p><label>{t('cronSchedule')}<input value={schedule} onChange={e => setSchedule(e.target.value)} placeholder="0 3 * * *" /></label></>}
              <div className="cron-preview">{t('cronPreview')}: <code>{cronSchedule}</code></div>
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
        {taskDetail && <TaskDetailModal task={taskDetail} onClose={() => setTaskDetail(null)} />}
      </div>
    </>
  )
}

function TaskDetailModal({task, onClose}: {task: Task; onClose: () => void}) {
  const {t, lang} = useI18n()
  return <div className="modal-back"><div className="modal task-detail-modal">
    <button className="close" onClick={onClose}>×</button>
    <h2>{taskTitle(task, lang)}</h2>
    <div className="deploy-status"><span className={`task-dot ${task.status}`} /><strong>{taskStatusLabel(task.status, t)}</strong><small>{task.id}</small></div>
    <dl className="task-meta"><div><dt>{t('createdAt')}</dt><dd>{new Date(task.created_at).toLocaleString()}</dd></div><div><dt>{t('updatedAt')}</dt><dd>{new Date(task.updated_at).toLocaleString()}</dd></div></dl>
    <h3>{t('taskLog')}</h3><pre className="deploy-log">{task.log || t('noLog')}</pre>
    <div className="card-actions"><button className="primary" onClick={onClose}>{t('close')}</button></div>
  </div></div>
}

/* —— Alerts —— */
function Alerts() {
  const {t} = useI18n()
  const [rules, setRules] = useState<AlertRule[]>([])
  const [name, setName] = useState(t('highCPU')), [metric, setMetric] = useState('cpu')
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
            <label>{t('metric')}<select value={metric} onChange={e => setMetric(e.target.value)}><option value="cpu">CPU %</option><option value="memory">{t('memory')} %</option><option value="disk">{t('disk')} %</option><option value="load">{t('load')}</option></select></label>
            <label>{t('threshold')}<input type="number" value={threshold} onChange={e => setThreshold(+e.target.value)} /></label>
            <label>{t('durationSec')}<input type="number" value={duration} onChange={e => setDuration(+e.target.value)} /></label>
            <button className="primary" onClick={save}>{t('save')}</button>
          </div>
          <div className="panel alert-form">
            <PanelTitle title={t('notify')} />
            <label>{t('webhookURL')}<input value={webhook} onChange={e => setWebhook(e.target.value)} placeholder="https://example.com/hook" /></label>
            <button className="primary" onClick={saveNotify}>{t('save')}</button>
            {message && <div className="success">{message}</div>}
          </div>
        </section>
        <div className="panel table-panel alert-table">
          <table>
            <thead><tr><th>{t('ruleName')}</th><th>{t('metric')}</th><th>{t('threshold')}</th><th>{t('durationSec')}</th><th /></tr></thead>
            <tbody>
              {rules.map(r => <tr key={r.id}><td>{r.name}</td><td>{metricLabel(r.metric, t)}</td><td>{t(r.operator === 'lt' ? 'lessThan' : 'greaterThan')} {r.threshold}</td><td>{r.duration_seconds} {t('seconds')}</td><td className="actions"><button className="danger" title={t('remove')} onClick={() => remove(r)}><Trash2 /></button></td></tr>)}
            </tbody>
          </table>
          {!rules.length && <div className="empty">{t('noData')}</div>}
        </div>
      </div>
    </>
  )
}

/* —— Settings —— */
function Settings({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const tabFromPath = () => location.pathname === '/settings/security' ? 'security' : 'general'
  const [tab, setTab] = useState<'general' | 'security'>(me.must_set_entry ? 'security' : tabFromPath())
  useEffect(() => {
    if (me.must_set_entry) history.replaceState({}, '', '/settings/security')
    else if (location.pathname === '/settings') history.replaceState({}, '', '/settings/general')
    const pop = () => setTab(tabFromPath())
    addEventListener('popstate', pop)
    return () => removeEventListener('popstate', pop)
  }, [me.must_set_entry])
  function selectTab(next: 'general' | 'security') {
    history.pushState({}, '', `/settings/${next}`)
    setTab(next)
  }
  return (
    <>
      <PageHead title={t('settings')} />
      <div className="page-body">
        <div className="tabs">
          <button className={tab === 'general' ? 'active' : ''} onClick={() => selectTab('general')}>{t('settingsTabGeneral')}</button>
          <button className={tab === 'security' ? 'active' : ''} onClick={() => selectTab('security')}>{t('settingsTabSecurity')}</button>
        </div>
        <div className="settings-stack" hidden={tab !== 'general'}><LocalIPBlock /><UpdateBlock /></div>
        <div className="settings-stack" hidden={tab !== 'security'}><PanelDomainWizard /><EntrySecurityBlock me={me} setMe={setMe} /><SecurityBlock me={me} setMe={setMe} /></div>
      </div>
    </>
  )
}

function LocalIPBlock() {
  const {t} = useI18n()
  const initial = cached<{local_ip: string; detected_ip: string}>('settingsLocalIP', {local_ip: '', detected_ip: ''})
  const [ip, setIP] = useState(initial.local_ip), [detected, setDetected] = useState(initial.detected_ip)
  const [message, setMessage] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  useEffect(() => { api<{local_ip: string; detected_ip: string}>('/settings/local-ip').then(v => { cache('settingsLocalIP', v); setIP(v.local_ip); setDetected(v.detected_ip) }).catch(e => setError(e.message)) }, [])
  async function save() {
    const warnings = [t('ipWarning1'), t('ipWarning2'), t('ipWarning3')]
    for (const warning of warnings) if (!window.confirm(warning)) return
    setBusy(true); setError(''); setMessage('')
    try { const v = await post<{local_ip: string; detected_ip: string}>('/settings/local-ip', {local_ip: ip}); cache('settingsLocalIP', v); setIP(v.local_ip); setMessage(t('ipSaved')) }
    catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  return <div className="panel settings-card">
    <div className="settings-card-head"><span className="security-icon"><Network /></span><div><h2>{t('localIPTitle')}</h2><p>{t('localIPHint')}</p></div></div>
    <label>{t('localIPAddress')}<input value={ip} onChange={e => setIP(e.target.value)} placeholder={detected || '192.168.1.10'} /><small>{t('detectedIP')}: {detected || '—'}</small></label>
    <div className="card-actions"><button className="primary" disabled={busy || !ip.trim()} onClick={save}>{busy ? '…' : t('save')}</button></div>
    {message && <div className="success">{message}</div>}{error && <div className="error">{error}</div>}
  </div>
}

function EntrySecurityBlock({me, setMe}: {me: Me; setMe: (m: Me) => void}) {
  const {t} = useI18n()
  const saved = cached<{path: string; decoy: '404' | 'dino'}>('settingsEntry', {path: me.entry_path || '', decoy: me.decoy_mode === 'dino' ? 'dino' : '404'})
  const [path, setPath] = useState(saved.path)
  const [decoy, setDecoy] = useState<'404' | 'dino'>(saved.decoy)
  const [message, setMessage] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const pathError = entryPathHint(path, t)
  useEffect(() => cache('settingsEntry', {path, decoy}), [path, decoy])
  async function save() {
    if (pathError) { setError(pathError); return }
    setBusy(true); setError(''); setMessage('')
    try {
      const v = await post<{entry_path: string; decoy_mode: string; entry_url: string}>('/settings/entry', {path, decoy_mode: decoy})
      setMe({...me, must_set_entry: false, entry_path: v.entry_path, decoy_mode: v.decoy_mode, entry_url: v.entry_url})
      setPath(v.entry_path)
      setMessage(t('entrySaved'))
    } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  return (
    <div className="panel settings-card">
      <div className="settings-card-head">
        <span className="security-icon"><LockKeyhole /></span>
        <div>
          <h2 style={{margin: '0 0 4px', fontSize: 16}}>{t('entryTitle')}</h2>
          <p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('entryDesc')}</p>
        </div>
      </div>
      <label>{t('entryPath')}
        <input value={path} onChange={e => { setPath(e.target.value); setError('') }} placeholder="s8k2m9xq" maxLength={64} />
        <small className={pathError ? 'field-error' : ''} style={{color: pathError ? 'var(--danger)' : 'var(--muted)'}}>{pathError || t('entryPathHint')}</small>
      </label>
      <label style={{display: 'block', marginTop: 12}}>{t('decoyMode')}
        <select value={decoy} onChange={e => setDecoy(e.target.value as '404' | 'dino')}>
          <option value="404">{t('decoy404')}</option>
          <option value="dino">{t('decoyDino')}</option>
        </select>
      </label>
      {!pathError && path && <div className="entry-preview" style={{marginTop: 12}}>{t('entryPreview')}: <code>{location.origin}/{path.replace(/^\/+|\/+$/g, '')}</code></div>}
      <div className="card-actions" style={{marginTop: 14}}>
        <button className="btn" type="button" onClick={() => { setPath(Math.random().toString(36).slice(2, 12)); setError('') }}>{t('randomEntry')}</button>
        <button className="primary" disabled={busy || !!pathError} onClick={save}>{busy ? '…' : t('saveEntry')}</button>
      </div>
      {message && <div className="success" style={{marginTop: 12}}>{message}</div>}
      {error && <div className="error banner" style={{marginTop: 12}}>{error}</div>}
    </div>
  )
}

function PanelDomainWizard() {
  const {t} = useI18n()
  const saved = cached('settingsPanelDomain', {step: 0, domain: '', tool: 'certbot', email: ''})
  const [step, setStep] = useState(saved.step)
  const [domain, setDomain] = useState(saved.domain), [tool, setTool] = useState(saved.tool), [email, setEmail] = useState(saved.email)
  const [message, setMessage] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  useEffect(() => cache('settingsPanelDomain', {step, domain, tool, email}), [step, domain, tool, email])

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
    <div className="panel domain-card settings-card">
      <div className="settings-card-head">
        <span className="security-icon"><Globe2 /></span>
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
    <div className="security-settings">
      <div className="security-head">
        <span className="security-icon"><ShieldCheck /></span>
        <div><h2>{t('totpTitle')}</h2><p>{me.totp_enabled ? t('totpOn') : t('totpOff')}</p></div>
        <span className={`security-status ${me.totp_enabled ? 'enabled' : ''}`}>{me.totp_enabled ? <CheckCircle2 /> : <LockKeyhole />}{me.totp_enabled ? t('enabled') : t('notEnabled')}</span>
      </div>
      {!me.totp_enabled && !secret && <div className="security-action"><div><strong>{t('authenticatorTitle')}</strong><p>{t('authenticatorHint')}</p></div><button className="primary" onClick={setup}>{t('setupTOTP')}</button></div>}
      {secret && (
        <div className="totp-setup">
          <div><strong>{t('totpSecret')}</strong><code>{secret.secret}</code><p>{t('totpSecretHint')}</p></div>
          <div className="totp-verify"><input inputMode="numeric" placeholder="000000" value={code} onChange={e => setCode(e.target.value.replace(/\D/g, ''))} maxLength={6} /><button className="primary" disabled={code.length !== 6} onClick={enable}>{t('totpVerify')}</button></div>
        </div>
      )}
      {message && <div className="success">{message}</div>}
      {error && <div className="error">{error}</div>}
    </div>
  )
}

function UpdateBlock() {
  const {t} = useI18n()
  const saved = cached<SystemInfo | null>('settingsUpdateInfo', null)
  const [info, setInfo] = useState<SystemInfo | null>(saved)
  const [channel, setChannel] = useState<'stable' | 'prerelease'>(() => cached('settingsUpdateChannel', saved?.channel === 'prerelease' ? 'prerelease' : 'stable'))
  const [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const [updating, setUpdating] = useState(false)

  const load = () => api<SystemInfo>('/system').then(v => {
    cache('settingsUpdateInfo', v)
    setInfo(v)
    setChannel(v.channel === 'prerelease' ? 'prerelease' : 'stable')
  }).catch(e => setError(e.message))

  useEffect(() => { void load() }, [])

  const remote = channel === 'prerelease' ? (info?.latest_prerelease || '') : (info?.latest_stable || '')
  const needsUpdate = !!(remote && info?.version && remote !== info.version)

  async function doUpdate() {
    if (!needsUpdate) return
    setBusy(true); setError('')
    setUpdating(true)
    try {
      await post('/actions', {kind: 'panel.self_update', resource: channel, options: {channel}})
    } catch {
      // Server may die mid-update during restart; progress modal continues on timers.
    } finally { setBusy(false) }
  }

  return (
    <div className="panel settings-card">
      <div className="settings-card-head">
        <span className="security-icon"><Download /></span>
        <div><h2 style={{margin: '0 0 4px', fontSize: 16}}>{t('updateTitle')}</h2><p style={{margin: 0, color: 'var(--muted)', fontSize: 13}}>{t('updateHint')}</p></div>
      </div>
      {info && (
        <div style={{display: 'grid', gap: 10, marginBottom: 16, fontSize: 13}}>
          <div><strong>{t('currentVersion')}:</strong> {info.version}</div>
          <div><strong>{t(channel === 'prerelease' ? 'latestPre' : 'latestStable')}:</strong> {remote || '-'}</div>
          <div><strong>{t('channel')}:</strong> {t(channel === 'prerelease' ? 'channelPre' : 'channelStable')}</div>
          <div>
            <strong>{t('updateCheck')}:</strong>{' '}
            {remote ? (needsUpdate ? <span className="days-warn">{t('updateAvailable')} ({remote})</span> : <span className="days-ok">{t('alreadyLatest')}</span>) : t('updateUnknown')}
          </div>
        </div>
      )}
      <label className="update-channel">{t('channel')}
        <select value={channel} onChange={e => { const next = e.target.value as 'stable' | 'prerelease'; setChannel(next); cache('settingsUpdateChannel', next) }}>
          <option value="stable">{t('channelStable')}</option>
          <option value="prerelease">{t('channelPre')}</option>
        </select>
        {channel === 'prerelease' && <small>{t('prereleaseWarning')}</small>}
      </label>
      <div className="card-actions" style={{marginTop: 14}}>
        <button className="btn" onClick={load}><RefreshCw size={14} />{t('checkUpdate')}</button>
        <button className="primary" disabled={busy || !needsUpdate} onClick={doUpdate}>{busy ? '…' : t('doUpdate')}</button>
      </div>
      {!needsUpdate && remote && <div className="success" style={{marginTop: 12}}>{t('alreadyLatest')}</div>}
      {error && <div className="error banner" style={{marginTop: 12}}>{error}</div>}
      {updating && <UpdateProgressModal channel={channel} target={remote} onDone={() => { setUpdating(false); void post('/auth/logout', {}).finally(() => { location.href = '/' }) }} />}
    </div>
  )
}

function UpdateProgressModal({channel, target, onDone}: {channel: string; target: string; onDone: () => void}) {
  const {t} = useI18n()
  // 0 download, 1 install, 2 restart, 3 login
  const [step, setStep] = useState(0)
  useEffect(() => {
    const timers: number[] = []
    timers.push(window.setTimeout(() => setStep(1), 2500))
    timers.push(window.setTimeout(() => setStep(2), 12000))
    // After restart window, probe until panel answers or give up to login step.
    let tries = 0
    const probe = window.setInterval(async () => {
      tries++
      if (tries < 4) return // wait ~restart window first (~16s)
      try {
        await fetch('/api/v1/me', {credentials: 'same-origin'})
        // any response (even 401) means web is back
        setStep(3)
        window.clearInterval(probe)
        window.setTimeout(onDone, 2500)
      } catch {
        if (tries > 40) {
          setStep(3)
          window.clearInterval(probe)
          window.setTimeout(onDone, 2500)
        }
      }
    }, 2000)
    timers.push(window.setTimeout(() => setStep(2), 14000))
    return () => { timers.forEach(clearTimeout); window.clearInterval(probe) }
  }, [onDone])

  const steps = [
    t('updateStepDownload'),
    t('updateStepInstall'),
    t('updateStepRestart'),
    t('updateStepLogin'),
  ]
  return (
    <div className="modal-back update-progress-back">
      <div className="modal update-progress-modal">
        <h2>{t('updatingTitle')}</h2>
        <p className="form-hint">{t(channel === 'prerelease' ? 'channelPre' : 'channelStable')}{target ? ` · ${target}` : ''}</p>
        <ol className="update-steps">
          {steps.map((label, i) => (
            <li key={label} className={i < step ? 'done' : i === step ? 'active' : ''}>
              <span className="step-num">{i < step ? '✓' : i + 1}</span>
              <span>{label}</span>
              {i === step && i < 3 && <em className="step-spin" />}
            </li>
          ))}
        </ol>
        <p className="form-hint" style={{marginBottom: 0}}>{t('updateDoNotClose')}</p>
      </div>
    </div>
  )
}

function ConfirmModal({title, message, confirmLabel, danger, onCancel, onConfirm}: {
  title: string
  message: string
  confirmLabel: string
  danger?: boolean
  onCancel: () => void
  onConfirm: () => void
}) {
  const {t} = useI18n()
  return (
    <div className="modal-back">
      <div className="modal confirm-modal">
        <h2>{title}</h2>
        <p className="confirm-msg">{message}</p>
        <div className="card-actions">
          <button type="button" className="btn" onClick={onCancel}>{t('cancel')}</button>
          <button type="button" className="primary" style={danger ? {background: 'var(--danger)', borderColor: 'var(--danger)'} : undefined} onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
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

/** Localize app/service status codes for the UI. */
function statusLabel(status: string, t: (k: any) => string) {
  switch (status) {
    case 'active':
    case 'running':
      return t('statusRunning')
    case 'inactive':
    case 'stopped':
      return t('statusStopped')
    case 'available':
    case 'installed':
      return t('statusInstalled')
    case 'not-installed':
    case 'missing':
      return t('statusNotInstalled')
    default:
      return status || '-'
  }
}

function statusOk(status: string) {
  return status === 'active' || status === 'running' || status === 'available' || status === 'installed'
}

function taskStatusLabel(status: string, t: (k: any) => string) {
  switch (status) {
    case 'queued': return t('statusQueued')
    case 'running': return t('statusRunningTask')
    case 'succeeded': return t('statusSucceeded')
    case 'failed': return t('statusFailed')
    case 'rolled_back': return t('statusRolledBack')
    default: return status
  }
}

/** Prefer backend Chinese summary; fall back to localizing kind + resource. */
function taskTitle(task: {kind: string; summary: string; resource?: string}, lang: string) {
  // New backend summaries are already Chinese human text (contain · or CJK).
  if (lang === 'zh' && task.summary && (/[·\u4e00-\u9fff]/.test(task.summary))) return task.summary
  const labels: Record<string, {zh: string; en: string}> = {
    'panel.self_update': {zh: '面板更新', en: 'Panel update'},
    'panel.bind_domain': {zh: '绑定面板域名', en: 'Bind panel domain'},
    'panel.unbind_domain': {zh: '恢复面板 IP 访问', en: 'Restore IP access'},
    'web.site.create': {zh: '创建网站', en: 'Create site'},
    'web.site.configure': {zh: '保存网站设置', en: 'Save site settings'},
    'web.site.rewrite': {zh: '设置伪静态', en: 'Set rewrite rules'},
    'web.site.delete': {zh: '删除网站', en: 'Delete site'},
    'web.apply': {zh: '应用网站配置', en: 'Apply web config'},
    'cert.issue': {zh: '申请证书', en: 'Issue certificate'},
    'cert.renew': {zh: '续期证书', en: 'Renew certificate'},
    'cert.delete': {zh: '删除证书', en: 'Delete certificate'},
    'package.install': {zh: '安装软件', en: 'Install software'},
    'package.update': {zh: '更新软件', en: 'Update software'},
    'files.write': {zh: '保存文件', en: 'Save file'},
    'files.mkdir': {zh: '新建目录', en: 'Create folder'},
    'files.delete': {zh: '删除文件', en: 'Delete file'},
    'files.rename': {zh: '重命名', en: 'Rename'},
    'crontab.add': {zh: '添加计划任务', en: 'Add cron job'},
    'crontab.remove': {zh: '删除计划任务', en: 'Remove cron job'},
    'docker.deploy': {zh: '部署 Docker', en: 'Deploy Docker'},
    'docker.container.start': {zh: '启动容器', en: 'Start container'},
    'docker.container.stop': {zh: '停止容器', en: 'Stop container'},
    'docker.container.restart': {zh: '重启容器', en: 'Restart container'},
    'docker.container.delete': {zh: '删除容器', en: 'Delete container'},
    'service.start': {zh: '启动服务', en: 'Start service'},
    'service.stop': {zh: '停止服务', en: 'Stop service'},
    'service.restart': {zh: '重启服务', en: 'Restart service'},
    'notification.configure': {zh: '配置通知', en: 'Configure notifications'},
    'auth.login_failed': {zh: '登录失败', en: 'Sign-in failed'},
    'auth.login': {zh: '管理员登录', en: 'Administrator signed in'},
    'auth.logout': {zh: '管理员退出', en: 'Administrator signed out'},
    'account.change': {zh: '修改管理员账号', en: 'Administrator account changed'},
    'account.totp_enable': {zh: '开启双因素认证', en: 'Two-factor authentication enabled'},
    'account.totp_disable': {zh: '关闭双因素认证', en: 'Two-factor authentication disabled'},
    'alert.save': {zh: '保存告警规则', en: 'Alert rule saved'},
    'alert.delete': {zh: '删除告警规则', en: 'Alert rule deleted'},
    'task.create': {zh: '创建面板任务', en: 'Panel task created'},
    'settings.entry': {zh: '修改安全入口', en: 'Secure entry changed'},
    'settings.local_ip': {zh: '修改本机 IP', en: 'Local IP changed'},
  }
  const L = labels[task.kind]
  const title = L ? (lang === 'zh' ? L.zh : L.en) : task.kind
  const rest = lang === 'en' && /[\u4e00-\u9fff]/.test(task.summary)
    ? (task.summary.includes('·') ? task.summary.split('·').slice(1).join('·').trim() : '')
    : (task.summary || '').replace(task.kind, '').trim()
  return rest ? `${title} ${rest}` : title
}

function auditDetail(event: Audit, lang: string) {
  if (!event.detail) return event.resource && event.resource !== 'session' ? event.resource : ''
  const details: Record<string, {zh: string; en: string}> = {
    'invalid credentials': {zh: '账号、密码或验证码不正确', en: 'Invalid credentials'},
    'login succeeded': {zh: '登录成功', en: 'Sign-in succeeded'},
    'credentials changed': {zh: '管理员凭据已修改', en: 'Administrator credentials changed'},
  }
  return details[event.detail]?.[lang === 'zh' ? 'zh' : 'en'] || event.detail
}

function metricLabel(metric: string, t: (k: any) => string) {
  if (metric === 'memory') return t('memory')
  if (metric === 'disk') return t('disk')
  if (metric === 'load') return t('load')
  return metric.toUpperCase()
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
