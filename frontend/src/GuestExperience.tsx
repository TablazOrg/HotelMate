import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, subscribeRealtime, type Service, type ServiceRequest, type Stay } from './api'
import { formatDate, formatPrice, formatTime, handoffTheme, mergeRequest, requestStatusMeta, serviceIcon, toFaDigits } from './handoff'

type GuestTab = 'home' | 'services' | 'tracking' | 'chat'
type ServiceFilter = 'all' | 'free' | 'paid'

const tabs: { id: GuestTab; label: string; icon: string }[] = [
  { id: 'home', label: 'خانه', icon: 'ri-home-5-line' },
  { id: 'services', label: 'سرویس‌ها', icon: 'ri-layout-grid-line' },
  { id: 'tracking', label: 'پیگیری', icon: 'ri-list-check-2' },
  { id: 'chat', label: 'گفتگو', icon: 'ri-chat-3-line' },
]

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'ارتباط با سرور انجام نشد.'
}

export function GuestExperience({ stay, token, onLogout }: { stay: Stay; token: string; onLogout: () => void }) {
  const [tab, setTab] = useState<GuestTab>('home')
  const [filter, setFilter] = useState<ServiceFilter>('all')
  const [services, setServices] = useState<Service[]>([])
  const [requests, setRequests] = useState<ServiceRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [connected, setConnected] = useState(false)
  const [toast, setToast] = useState('')
  const toastTimer = useRef(0)

  const load = useCallback(async () => {
    setError('')
    try {
      const [catalog, history] = await Promise.all([
        api<{ services: Service[] }>('/api/v1/guest/services', {}, token),
        api<{ requests: ServiceRequest[] }>('/api/v1/guest/requests', {}, token),
      ])
      setServices(catalog.services)
      setRequests(history.requests)
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => { void load() }, [load])
  useEffect(() => subscribeRealtime(token, (event) => {
    if (event.payload.request) setRequests((items) => mergeRequest(items, event.payload.request!))
  }, setConnected), [token])
  useEffect(() => () => window.clearTimeout(toastTimer.current), [])

  const quickServices = useMemo(() => services.filter((service) => service.isQuickAction && !service.isPaid), [services])
  const paidServices = useMemo(() => services.filter((service) => service.isPaid), [services])
  const filteredServices = useMemo(() => services.filter((service) => (
    filter === 'all' || (filter === 'free' ? !service.isPaid : service.isPaid)
  )), [filter, services])
  const openCount = requests.filter((request) => request.status === 'new' || request.status === 'in_progress').length

  async function createRequest(service: Service) {
    if (service.isPaid) return
    setBusy(service.id)
    setError('')
    try {
      const response = await api<{ request: ServiceRequest }>('/api/v1/guest/requests', {
        method: 'POST', body: JSON.stringify({ serviceId: service.id, quantity: 1, notes: '' }),
      }, token)
      setRequests((items) => mergeRequest(items, response.request))
      setToast(`درخواست «${service.name}» ثبت شد و به تیم هتل رسید`)
      window.clearTimeout(toastTimer.current)
      toastTimer.current = window.setTimeout(() => setToast(''), 3200)
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="guest-app" style={handoffTheme(stay.hotel.primaryColor)}>
      <div className="mobile-surface">
        <main className="guest-content">
          {tab === 'home' && <HomeView stay={stay} quickServices={quickServices} paidServices={paidServices} openCount={openCount} busy={busy} onRequest={createRequest} onTracking={() => setTab('tracking')} />}
          {tab === 'services' && <ServicesView services={filteredServices} filter={filter} setFilter={setFilter} busy={busy} onRequest={createRequest} />}
          {tab === 'tracking' && <TrackingView requests={requests} connected={connected} loading={loading} />}
          {tab === 'chat' && <ChatPlaceholder />}
          {error && <div className="inline-alert error" role="alert"><i className="ri-error-warning-line" />{error}<button type="button" onClick={() => setError('')} aria-label="بستن"><i className="ri-close-line" /></button></div>}
        </main>

        <button type="button" className="guest-logout" onClick={onLogout}><i className="ri-logout-box-r-line" /> خروج امن</button>
        {toast && <div className="request-toast" role="status"><i className="ri-checkbox-circle-fill" /><span>{toast}</span><button type="button" onClick={() => { setTab('tracking'); setToast('') }}>پیگیری</button></div>}
        <nav className="bottom-nav" aria-label="ناوبری مهمان">
          {tabs.map((item) => <button type="button" key={item.id} className={tab === item.id ? 'active' : ''} onClick={() => setTab(item.id)}>
            <span><i className={item.icon} />{item.id === 'tracking' && openCount > 0 && <b>{toFaDigits(openCount)}</b>}</span><small>{item.label}</small>
          </button>)}
        </nav>
      </div>
    </div>
  )
}

function HomeView({ stay, quickServices, paidServices, openCount, busy, onRequest, onTracking }: {
  stay: Stay; quickServices: Service[]; paidServices: Service[]; openCount: number; busy: string
  onRequest: (service: Service) => void; onTracking: () => void
}) {
  const departure = stay.reservation?.departureDate || stay.checkOutAt
  return <div className="view-enter">
    <header className="guest-header">
      <div><p>سلام، خوش آمدید</p><h1>{stay.guest.firstName} {stay.guest.lastName}</h1></div>
      <span className="room-chip"><i className="ri-door-open-line" /> اتاق {toFaDigits(stay.room.number)}</span>
    </header>
    <button type="button" className="stay-banner" onClick={onTracking}>
      <span><i />اقامت شما {departure ? `تا ${formatDate(departure)}` : ''} فعال است</span>
      <strong>{openCount ? `${toFaDigits(openCount)} درخواست فعال` : 'بدون درخواست فعال'} <i className="ri-arrow-left-s-line" /></strong>
    </button>
    <section className="guest-section">
      <div className="compact-heading"><h2>دسترسی سریع</h2><span>با یک لمس ثبت کنید</span></div>
      <div className="quick-grid">
        {quickServices.map((service) => <button type="button" key={service.id} disabled={busy === service.id} onClick={() => onRequest(service)}>
          {busy === service.id ? <span className="tiny-spinner" /> : <i className={serviceIcon(service)} />}<strong>{service.name}</strong>
        </button>)}
        {!quickServices.length && <p className="empty-copy">سرویس فعالی ثبت نشده است.</p>}
      </div>
    </section>
    <section className="guest-section">
      <div className="compact-heading"><h2>سرویس‌های ویژه</h2><span>پرداخت در محل · مرحله بعد</span></div>
      <div className="service-rows">
        {paidServices.length ? paidServices.slice(0, 3).map((service) => <ServiceRow service={service} key={service.id} busy={busy} onRequest={onRequest} />) : <div className="future-card"><i className="ri-gift-line" /><div><strong>سرویس‌های ویژه در راه است</strong><p>سفارش‌های پولی پس از تکمیل Milestone 4 فعال می‌شوند.</p></div></div>}
      </div>
    </section>
  </div>
}

function ServicesView({ services, filter, setFilter, busy, onRequest }: {
  services: Service[]; filter: ServiceFilter; setFilter: (value: ServiceFilter) => void; busy: string; onRequest: (service: Service) => void
}) {
  return <div className="view-enter screen-view">
    <header className="screen-heading"><h1>کاتالوگ سرویس‌ها</h1><p>هر آنچه برای یک اقامت راحت نیاز دارید</p></header>
    <div className="filter-chips" role="tablist" aria-label="فیلتر سرویس‌ها">
      {([['all', 'همه'], ['free', 'رایگان'], ['paid', 'پولی']] as const).map(([value, label]) => <button type="button" key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>{label}</button>)}
    </div>
    <div className="service-rows catalog-list">
      {services.map((service) => <ServiceRow service={service} key={service.id} busy={busy} onRequest={onRequest} />)}
      {!services.length && <EmptyState icon="ri-inbox-line" text="سرویسی در این دسته وجود ندارد" />}
    </div>
  </div>
}

function ServiceRow({ service, busy, onRequest }: { service: Service; busy: string; onRequest: (service: Service) => void }) {
  return <article className="service-row">
    <span className="service-icon"><i className={serviceIcon(service)} /></span>
    <div><strong>{service.name}</strong><small>{service.isPaid ? `${formatPrice(service.priceCents, service.currency)} تومان · پرداخت در محل` : `رایگان · تحویل تا ${toFaDigits(service.estimatedMinutes)} دقیقه`}</small></div>
    <button type="button" disabled={service.isPaid || busy === service.id} onClick={() => onRequest(service)}>{busy === service.id ? 'ثبت…' : service.isPaid ? 'به‌زودی' : 'درخواست'}</button>
  </article>
}

function TrackingView({ requests, connected, loading }: { requests: ServiceRequest[]; connected: boolean; loading: boolean }) {
  return <div className="view-enter screen-view">
    <header className="tracking-heading"><h1>پیگیری درخواست‌ها</h1><span className={connected ? 'connected' : ''}><i />{connected ? 'به‌روزرسانی زنده' : 'در حال اتصال'}</span></header>
    {loading ? <div className="center-loading"><span className="spinner" /> در حال دریافت درخواست‌ها…</div> : <div className="tracking-list">
      {requests.map((request) => <RequestCard request={request} key={request.id} />)}
      {!requests.length && <EmptyState icon="ri-inbox-line" text="هنوز درخواستی ثبت نکرده‌اید" />}
    </div>}
  </div>
}

function RequestCard({ request }: { request: ServiceRequest }) {
  const status = requestStatusMeta[request.status]
  const step = request.status === 'completed' ? 3 : request.status === 'in_progress' ? 2 : request.status === 'cancelled' ? 0 : 1
  return <article className={`request-card ${request.status}`}>
    <div className="request-card-head"><span className="service-icon"><i className={serviceIcon(request.service)} /></span><div><strong>{request.service.name}</strong><small>ثبت‌شده در {formatTime(request.createdAt)}{request.totalPriceCents > 0 ? ` · ${formatPrice(request.totalPriceCents, request.service.currency)} تومان` : ''}</small></div><span className={`status-pill ${status.className}`}>{status.label}</span></div>
    {request.status !== 'cancelled' && <><div className="request-progress">{[1, 2, 3].map((item) => <i key={item} className={item <= step ? 'filled' : ''} />)}</div><div className="progress-labels"><span>جدید</span><span>در حال انجام</span><span>تکمیل شد</span></div></>}
  </article>
}

function ChatPlaceholder() {
  return <div className="view-enter screen-view chat-placeholder">
    <header className="chat-head"><span><i className="ri-sparkling-2-line" /></span><div><h1>دستیار هوشمند</h1><p>به‌زودی در دسترس</p></div></header>
    <div className="chat-future"><i className="ri-chat-smile-3-line" /><h2>گفتگو در Milestone 5 فعال می‌شود</h2><p>پاسخ‌گویی هوشمند و انتقال امن گفتگو به پذیرش پس از تکمیل دانش‌نامه و کنترل‌های حریم خصوصی ارائه خواهد شد.</p></div>
    <div className="chat-composer disabled"><span>مثلاً: استخر تا چه ساعتی باز است؟</span><button type="button" disabled><i className="ri-send-plane-2-line" /></button></div>
  </div>
}

function EmptyState({ icon, text }: { icon: string; text: string }) {
  return <div className="guest-empty"><i className={icon} /><p>{text}</p></div>
}
