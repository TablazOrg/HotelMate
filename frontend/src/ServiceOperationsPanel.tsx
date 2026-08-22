import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  api,
  subscribeRealtime,
  type Hotel,
  type OperationalReport,
  type Service,
  type ServiceCategory,
  type ServiceRequest,
  type ServiceRequestStatus,
  type Staff,
} from './api'
import { formatPrice, formatTime, handoffTheme, mergeRequest, requestStatusMeta, serviceIcon, toFaDigits } from './handoff'

const statusTabs: { value: Exclude<ServiceRequestStatus, 'cancelled'>; label: string }[] = [
  { value: 'new', label: 'جدید' },
  { value: 'in_progress', label: 'در حال انجام' },
  { value: 'completed', label: 'تکمیل‌شده' },
]

const roleLabels: Record<Staff['role'], string> = {
  primary_admin: 'ادمین اصلی', secondary_admin: 'ادمین دوم', operations_manager: 'مدیر عملیات',
  reception: 'پذیرش', housekeeping: 'خانه‌داری', food_beverage: 'غذا و نوشیدنی',
}

const categoryLabels: Record<ServiceCategory, string> = {
  housekeeping: 'خانه‌داری', food_beverage: 'غذا و نوشیدنی', transport: 'حمل‌ونقل', wellness: 'سلامت', other: 'سایر',
}

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'عملیات انجام نشد.'
}

export function StaffRequestQueue({ token, staff, hotel, onLogout }: { token: string; staff: Staff; hotel: Hotel; onLogout: () => void }) {
  const [requests, setRequests] = useState<ServiceRequest[]>([])
  const [status, setStatus] = useState<Exclude<ServiceRequestStatus, 'cancelled'>>('new')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [connected, setConnected] = useState(false)

  const load = useCallback(async () => {
    try {
      const response = await api<{ requests: ServiceRequest[] }>('/api/v1/staff/requests', {}, token)
      setRequests(response.requests)
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

  const visible = requests.filter((request) => request.status === status)

  async function advance(request: ServiceRequest) {
    const nextStatus: ServiceRequestStatus = request.status === 'new' ? 'in_progress' : 'completed'
    setBusy(request.id)
    setError('')
    try {
      const response = await api<{ request: ServiceRequest }>(`/api/v1/staff/requests/${request.id}/transition`, {
        method: 'POST', body: JSON.stringify({ status: nextStatus, note: '' }),
      }, token)
      setRequests((items) => mergeRequest(items, response.request))
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setBusy('')
    }
  }

  return <div className="staff-mobile-app" style={handoffTheme(hotel.primaryColor)}>
    <div className="mobile-surface staff-surface">
      <header className="staff-mobile-header">
        <span className="staff-avatar">{staff.firstName.slice(0, 1)}{staff.lastName.slice(0, 1)}</span>
        <div><h1>{staff.firstName} {staff.lastName}</h1><p>{roleLabels[staff.role]} · {hotel.name}</p></div>
        <span className={`connection-pill ${connected ? 'connected' : ''}`}><i />{connected ? 'متصل' : 'اتصال…'}</span>
      </header>
      <div className="queue-tabs" role="tablist" aria-label="وضعیت درخواست‌ها">
        {statusTabs.map((item) => <button type="button" key={item.value} className={status === item.value ? 'active' : ''} onClick={() => setStatus(item.value)}><strong>{toFaDigits(requests.filter((request) => request.status === item.value).length)}</strong><small>{item.label}</small></button>)}
      </div>
      {error && <div className="inline-alert error"><i className="ri-error-warning-line" />{error}</div>}
      <main className="queue-list">
        {loading ? <div className="center-loading"><span className="spinner" /> در حال دریافت صف…</div> : visible.map((request) => {
          const meta = requestStatusMeta[request.status]
          return <article className="queue-card" key={request.id}>
            <div className="queue-card-head"><span className="service-icon"><i className={serviceIcon(request.service)} /></span><div><strong>{request.service.name}</strong><small>اتاق {toFaDigits(request.stay?.room.number ?? '—')} · {request.stay?.guest.firstName} {request.stay?.guest.lastName} · {formatTime(request.createdAt)}</small></div><span className={`status-pill ${meta.className}`}>{meta.label}</span></div>
            {request.notes && <p className="request-note"><i className="ri-sticky-note-line" /> {request.notes}</p>}
            {request.status !== 'completed' && <button type="button" className={request.status === 'in_progress' ? 'queue-action complete' : 'queue-action'} disabled={busy === request.id} onClick={() => void advance(request)}>{busy === request.id ? 'در حال ثبت…' : request.status === 'new' ? 'شروع رسیدگی' : 'تکمیل درخواست'}</button>}
          </article>
        })}
        {!loading && !visible.length && <div className="guest-empty queue-empty"><i className="ri-checkbox-multiple-line" /><p>موردی در این وضعیت نیست</p></div>}
      </main>
      <button type="button" className="staff-logout" onClick={onLogout}><i className="ri-logout-box-r-line" /> خروج از شیفت</button>
    </div>
  </div>
}

export function AdminRequestOverview({ token }: { token: string }) {
  const [requests, setRequests] = useState<ServiceRequest[]>([])
  const [report, setReport] = useState<OperationalReport | null>(null)
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  const refreshReport = useCallback(async () => {
    const reporting = await api<{ report: OperationalReport }>('/api/v1/staff/reports/operations', {}, token)
    setReport(reporting.report)
  }, [token])

  const load = useCallback(async () => {
    try {
      const [queue] = await Promise.all([
        api<{ requests: ServiceRequest[] }>('/api/v1/staff/requests', {}, token),
        refreshReport(),
      ])
      setRequests(queue.requests)
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setLoading(false)
    }
  }, [refreshReport, token])

  useEffect(() => { void load() }, [load])
  useEffect(() => subscribeRealtime(token, (event) => {
    if (event.payload.request) {
      setRequests((items) => mergeRequest(items, event.payload.request!))
      void refreshReport().catch((requestError) => setError(messageFrom(requestError)))
    }
  }, setConnected), [refreshReport, token])

  async function advance(request: ServiceRequest) {
    const status: ServiceRequestStatus = request.status === 'new' ? 'in_progress' : 'completed'
    setBusy(request.id); setError('')
    try {
      const response = await api<{ request: ServiceRequest }>(`/api/v1/staff/requests/${request.id}/transition`, {
        method: 'POST', body: JSON.stringify({ status, note: '' }),
      }, token)
      setRequests((items) => mergeRequest(items, response.request))
      await refreshReport()
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy('') }
  }

  const summary = report?.summary
  const stats = [
    ['درخواست‌های باز', summary?.openRequests ?? 0, connected ? 'زنده و متصل' : 'در حال اتصال'],
    ['تکمیل‌شده امروز', summary?.completedRequests ?? 0, summary?.averageFulfillmentMinutes ? `میانگین رسیدگی ${toFaDigits(summary.averageFulfillmentMinutes)} دقیقه` : 'هنوز داده‌ای نیست'],
    ['درآمد جانبی امروز', `${formatPrice(summary?.recognizedRevenueCents ?? 0)} تومان`, 'از سفارش‌های پولی'],
    ['اتاق‌های فعال', summary?.activeRooms ?? 0, `از ${toFaDigits(summary?.totalRooms ?? 0)} اتاق`],
  ] as const

  return <div className="admin-overview view-enter">
    <div className="admin-page-heading"><div><p>نمای لحظه‌ای عملیات</p><h1>داشبورد امروز</h1></div><span className={`connection-pill ${connected ? 'connected' : ''}`}><i />{connected ? 'به‌روزرسانی زنده' : 'در حال اتصال'}</span></div>
    {error && <div className="inline-alert error">{error}</div>}
    <div className="stats-grid">{stats.map(([label, value, hint]) => <article key={label}><span>{label}</span><strong>{loading ? '—' : toFaDigits(value)}</strong><small>{hint}</small></article>)}</div>
    <section className="live-table-card"><div className="compact-heading"><h2>درخواست‌های لحظه‌ای</h2><span>آخرین رویدادها</span></div><div className="live-table"><div className="table-head"><span>درخواست</span><span>اتاق / مهمان</span><span>زمان</span><span>وضعیت و اقدام</span></div>{requests.slice(0, 8).map((request) => { const meta = requestStatusMeta[request.status]; return <div className="table-row" key={request.id}><span><i className={serviceIcon(request.service)} />{request.service.name}</span><span>اتاق {toFaDigits(request.stay?.room.number ?? '—')} · {request.stay?.guest.firstName ?? '—'}</span><span>{formatTime(request.createdAt)}</span><span className="table-status-action"><b className={`status-pill ${meta.className}`}>{meta.label}</b>{(request.status === 'new' || request.status === 'in_progress') && <button type="button" disabled={busy === request.id} onClick={() => void advance(request)}>{busy === request.id ? '…' : request.status === 'new' ? 'شروع' : 'تکمیل'}</button>}</span></div>})}{!requests.length && !loading && <div className="table-empty">درخواستی ثبت نشده است.</div>}</div></section>
  </div>
}

type CatalogForm = {
  name: string; code: string; description: string; category: ServiceCategory
  fulfillmentRole: Service['fulfillmentRole']; estimatedMinutes: string; isQuickAction: boolean
  isPaid: boolean; priceToman: string; isPreArrival: boolean; availableFrom: string; availableUntil: string
}

const initialForm: CatalogForm = { name: '', code: '', description: '', category: 'other', fulfillmentRole: 'reception', estimatedMinutes: '20', isQuickAction: false, isPaid: false, priceToman: '', isPreArrival: false, availableFrom: '', availableUntil: '' }

function servicePayload(service: Service) {
  const { id: _id, ...payload } = service
  return payload
}

export function ServiceCatalogPanel({ token, canManage }: { token: string; canManage: boolean }) {
  const [services, setServices] = useState<Service[]>([])
  const [form, setForm] = useState<CatalogForm>(initialForm)
  const [showForm, setShowForm] = useState(false)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try { setServices((await api<{ services: Service[] }>('/api/v1/staff/services', {}, token)).services) }
    catch (requestError) { setError(messageFrom(requestError)) }
    finally { setLoading(false) }
  }, [token])
  useEffect(() => { void load() }, [load])

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy('create'); setError('')
    try {
      const icon = ({ housekeeping: 'cleaning', food_beverage: 'coffee', transport: 'car', wellness: 'wellness', other: 'concierge' } as const)[form.category]
      const response = await api<{ service: Service }>('/api/v1/staff/services', { method: 'POST', body: JSON.stringify({
        ...form, icon, estimatedMinutes: Number(form.estimatedMinutes), priceCents: form.isPaid ? Number(form.priceToman) * 10 : 0, currency: 'IRR',
        sortOrder: (services.length + 1) * 10, isActive: true,
      }) }, token)
      setServices((items) => [...items, response.service]); setForm(initialForm); setShowForm(false)
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy('') }
  }

  async function toggle(service: Service) {
    setBusy(service.id); setError('')
    try {
      const response = await api<{ service: Service }>(`/api/v1/staff/services/${service.id}`, { method: 'PATCH', body: JSON.stringify({ ...servicePayload(service), isActive: !service.isActive }) }, token)
      setServices((items) => items.map((item) => item.id === response.service.id ? response.service : item))
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy('') }
  }

  const activeCount = useMemo(() => services.filter((service) => service.isActive).length, [services])
  return <div className="catalog-admin view-enter">
    <div className="admin-page-heading"><div><p>{toFaDigits(activeCount)} سرویس فعال</p><h1>کاتالوگ سرویس‌ها</h1></div>{canManage && <button type="button" className="primary-button compact" onClick={() => setShowForm(!showForm)}><i className={showForm ? 'ri-close-line' : 'ri-add-line'} />{showForm ? 'بستن' : 'سرویس جدید'}</button>}</div>
    {showForm && <form className="catalog-form" onSubmit={create}>
      <div className="form-row"><label>نام سرویس<input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required /></label><label>کد انگلیسی<input dir="ltr" pattern="[a-z0-9]+(?:-[a-z0-9]+)*" value={form.code} onChange={(event) => setForm({ ...form, code: event.target.value.toLowerCase() })} placeholder="extra-towel" required /></label></div>
      <label>توضیح<input value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label>
      <div className="form-row"><label>دسته<select value={form.category} onChange={(event) => setForm({ ...form, category: event.target.value as ServiceCategory })}>{Object.entries(categoryLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label>تیم مسئول<select value={form.fulfillmentRole} onChange={(event) => setForm({ ...form, fulfillmentRole: event.target.value as Service['fulfillmentRole'] })}><option value="reception">پذیرش</option><option value="housekeeping">خانه‌داری</option><option value="food_beverage">غذا و نوشیدنی</option></select></label></div>
      <div className="form-row"><label>زمان تقریبی (دقیقه)<input type="number" min="1" max="1440" value={form.estimatedMinutes} onChange={(event) => setForm({ ...form, estimatedMinutes: event.target.value })} required /></label><label>قیمت (تومان)<input type="number" min="0" disabled={!form.isPaid} value={form.priceToman} onChange={(event) => setForm({ ...form, priceToman: event.target.value })} required={form.isPaid} /></label></div>
      <div className="catalog-checks"><label className="check-label"><input type="checkbox" checked={form.isQuickAction} onChange={(event) => setForm({ ...form, isQuickAction: event.target.checked })} /> نمایش در دسترسی سریع</label><label className="check-label"><input type="checkbox" checked={form.isPaid} onChange={(event) => setForm({ ...form, isPaid: event.target.checked, isPreArrival: event.target.checked ? form.isPreArrival : false })} /> سرویس پولی</label><label className="check-label"><input type="checkbox" disabled={!form.isPaid} checked={form.isPreArrival} onChange={(event) => setForm({ ...form, isPreArrival: event.target.checked })} /> قابل سفارش پیش از ورود</label></div>
      <div className="form-row"><label>فعال از ساعت<input type="time" value={form.availableFrom} onChange={(event) => setForm({ ...form, availableFrom: event.target.value })} /></label><label>فعال تا ساعت<input type="time" value={form.availableUntil} onChange={(event) => setForm({ ...form, availableUntil: event.target.value })} /></label></div>
      <p className="form-context">پرداخت آنلاین عمداً غیرفعال است؛ سفارش پولی با پرداخت در محل ثبت می‌شود.</p>
      <button className="primary-button" disabled={busy === 'create'}>{busy === 'create' ? 'در حال ایجاد…' : 'ایجاد سرویس'}</button>
    </form>}
    {error && <div className="inline-alert error">{error}</div>}
    <div className="catalog-admin-list">
      {services.map((service) => <article key={service.id} className={!service.isActive ? 'inactive' : ''}><span className="service-icon"><i className={serviceIcon(service)} /></span><div><strong>{service.name}</strong><small>{categoryLabels[service.category]} · {service.isPaid ? `${toFaDigits(Math.round(service.priceCents / 10).toLocaleString('fa-IR'))} تومان` : `تحویل تا ${toFaDigits(service.estimatedMinutes)} دقیقه`}{service.isQuickAction ? ' · دسترسی سریع' : ''}{service.isPreArrival ? ' · پیش از ورود' : ''}</small></div><span className={`status-pill ${service.isActive ? 'success' : ''}`}>{service.isActive ? 'فعال' : 'غیرفعال'}</span>{canManage && <button type="button" className="small-button" disabled={busy === service.id} onClick={() => void toggle(service)}>{busy === service.id ? '…' : service.isActive ? 'غیرفعال‌سازی' : 'فعال‌سازی'}</button>}</article>)}
      {!services.length && !loading && <div className="empty-state">هنوز سرویسی ثبت نشده است.</div>}
    </div>
  </div>
}
