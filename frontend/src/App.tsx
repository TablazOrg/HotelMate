import { type FormEvent, useEffect, useMemo, useState } from 'react'
import {
  ApiError, api, clearSession, type GuestLoginResponse, type Hotel, loadSession, saveSession,
  type SessionRecord, type Staff, type StaffLoginResponse, type StaffRole, type Stay,
} from './api'
import { GuestCheckInPanel } from './GuestCheckInPanel'
import { GuestExperience } from './GuestExperience'
import { ContentAdminPanel, PreArrivalOrders, VisitorExperience } from './HotelContentPanels'
import { KnowledgeAdminPanel, StaffConversationInbox } from './ConversationPanels'
import { OperationsPanel } from './OperationsPanel'
import { AuditPanel, ReportingPanel } from './ReportingPanel'
import { AdminRequestOverview, ServiceCatalogPanel, StaffRequestQueue } from './ServiceOperationsPanel'
import { formatDate, handoffTheme, toFaDigits } from './handoff'

type AuthenticatedState = { session: SessionRecord; staff?: Staff; hotel?: Hotel; stay?: Stay }
type AdminTab = 'dashboard' | 'reception' | 'catalog' | 'content' | 'conversations' | 'reports' | 'security' | 'branding' | 'staff' | 'knowledge'

const roleLabels: Record<StaffRole, string> = {
  primary_admin: 'مدیر اصلی', secondary_admin: 'مدیر دوم', operations_manager: 'مدیر عملیات',
  reception: 'پذیرش', housekeeping: 'خانه‌داری', food_beverage: 'غذا و نوشیدنی',
}

function messageFrom(error: unknown) {
  if (error instanceof ApiError) return error.message
  return 'ارتباط با سرور انجام نشد. دوباره تلاش کنید.'
}

function App() {
  const [authState, setAuthState] = useState<AuthenticatedState | null>(null)
  const [booting, setBooting] = useState(true)

  useEffect(() => {
    const session = loadSession()
    if (!session) { setBooting(false); return }
    const path = session.actorType === 'staff' ? '/api/v1/staff/me' : '/api/v1/guest/me'
    api<{ staff?: Staff; hotel?: Hotel; stay?: Stay }>(path, {}, session.token)
      .then((payload) => setAuthState({ session, ...payload }))
      .catch(() => clearSession())
      .finally(() => setBooting(false))
  }, [])

  function authenticate(response: StaffLoginResponse | GuestLoginResponse) {
    const session: SessionRecord = { actorType: response.actorType, token: response.token, expiresAt: response.expiresAt }
    saveSession(session)
    if (response.actorType === 'staff') {
      window.history.replaceState({}, '', '/staff')
      setAuthState({ session, staff: response.staff, hotel: response.hotel })
    } else {
      window.history.replaceState({}, '', response.stay.status === 'pre_arrival' ? '/pre-arrival' : '/')
      setAuthState({ session, stay: response.stay, hotel: response.stay.hotel })
    }
  }

  function logout() {
    clearSession(); setAuthState(null); window.history.replaceState({}, '', '/')
  }

  if (booting) return <div className="loading-screen"><span className="spinner" />در حال بازیابی نشست…</div>
  if (!authState) return window.location.pathname.startsWith('/visitor') ? <VisitorExperience /> : <LoginScreen onAuthenticated={authenticate} />
  if (authState.session.actorType === 'guest' && authState.stay) {
    return authState.stay.status === 'pre_arrival'
      ? <PreArrivalExperience stay={authState.stay} token={authState.session.token} onLogout={logout} />
      : <GuestExperience stay={authState.stay} token={authState.session.token} onLogout={logout} />
  }
  if (authState.staff && authState.hotel) return <StaffDashboard session={authState.session} initialStaff={authState.staff} initialHotel={authState.hotel} onLogout={logout} />
  return <LoginScreen onAuthenticated={authenticate} />
}

function LoginScreen({ onAuthenticated }: { onAuthenticated: (response: StaffLoginResponse | GuestLoginResponse) => void }) {
  const path = window.location.pathname
  const mode: 'guest' | 'staff' = path.startsWith('/staff') ? 'staff' : 'guest'
  const guestAccess: 'reservation' | 'room' = path.startsWith('/pre-arrival') ? 'reservation' : 'room'
  const [hotelSlug, setHotelSlug] = useState('')
  const [roomNumber, setRoomNumber] = useState('')
  const [confirmationCode, setConfirmationCode] = useState('')
  const [identityNumber, setIdentityNumber] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [hotel, setHotel] = useState<Hotel | null>(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [apiOnline, setApiOnline] = useState<boolean | null>(null)

  useEffect(() => { api('/healthz').then(() => setApiOnline(true)).catch(() => setApiOnline(false)) }, [])
  useEffect(() => {
    const slug = hotelSlug.trim().toLowerCase()
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) { setHotel(null); return }
    const timeout = window.setTimeout(() => api<{ hotel: Hotel }>(`/api/v1/public/hotels/${encodeURIComponent(slug)}`).then((payload) => setHotel(payload.hotel)).catch(() => setHotel(null)), 350)
    return () => window.clearTimeout(timeout)
  }, [hotelSlug])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setSubmitting(true)
    try {
      if (mode === 'staff') {
        onAuthenticated(await api<StaffLoginResponse>('/api/v1/auth/staff/login', { method: 'POST', body: JSON.stringify({ hotelSlug, email, password }) }))
      } else {
        const endpoint = guestAccess === 'reservation' ? '/api/v1/auth/guest/reservation' : '/api/v1/auth/guest/login'
        const credentials = guestAccess === 'reservation' ? { hotelSlug, confirmationCode, identityNumber } : { hotelSlug, roomNumber, identityNumber }
        onAuthenticated(await api<GuestLoginResponse>(endpoint, { method: 'POST', body: JSON.stringify(credentials) }))
      }
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setSubmitting(false) }
  }

  const title = mode === 'staff' ? 'ورود پرسنل هتل' : guestAccess === 'reservation' ? 'ورود پیش از رسیدن' : 'خوش آمدید'
  const helper = mode === 'staff' ? 'برای مشاهده صف عملیات وارد حساب سازمانی شوید.' : guestAccess === 'reservation' ? 'با اطلاعات رزرو، چک‌این آنلاین را تکمیل کنید.' : 'برای ورود به اقامت، اطلاعات اتاق را وارد کنید.'

  return <main className="auth-shell" style={handoffTheme(hotel?.primaryColor)}>
    <section className="auth-mobile">
      <div className="auth-brand">{hotel?.logoUrl ? <img src={hotel.logoUrl} alt="" /> : <span>H</span>}<strong>{hotel?.name || 'HotelMate'}</strong></div>
      <div className="auth-intro"><span><i className={mode === 'staff' ? 'ri-shield-user-line' : guestAccess === 'reservation' ? 'ri-calendar-check-line' : 'ri-key-2-line'} /></span><h1>{title}</h1><p>{helper}</p></div>
      <form onSubmit={submit} className="auth-form">
        <label>شناسه هتل<input value={hotelSlug} onChange={(event) => setHotelSlug(event.target.value)} placeholder="example-hotel" autoComplete="organization" dir="ltr" required /></label>
        {mode === 'staff' ? <>
          <label>ایمیل سازمانی<input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" dir="ltr" required /></label>
          <label>رمز عبور<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" dir="ltr" required /></label>
        </> : <>
          {guestAccess === 'reservation' ? <label>کد تأیید رزرو<input value={confirmationCode} onChange={(event) => setConfirmationCode(event.target.value.toUpperCase())} dir="ltr" required /></label> : <label>شماره اتاق<input value={roomNumber} onChange={(event) => setRoomNumber(event.target.value)} inputMode="numeric" required /></label>}
          <label>کد ملی یا شماره پاسپورت<input value={identityNumber} onChange={(event) => setIdentityNumber(event.target.value)} dir="ltr" required /></label>
        </>}
        {error && <div className="inline-alert error" role="alert"><i className="ri-error-warning-line" />{error}</div>}
        <button className="primary-button auth-submit" disabled={submitting}>{submitting ? 'در حال ورود امن…' : mode === 'staff' ? 'ورود به پنل عملیات' : guestAccess === 'reservation' ? 'مشاهده رزرو' : 'ورود به اقامت'}</button>
      </form>
      {mode === 'guest' && <p className="otp-note">ورود با پیامک یک‌بارمصرف به‌زودی فعال می‌شود</p>}
      <nav className="auth-links">
        {mode === 'staff' ? <a href="/">ورود مهمان <i className="ri-arrow-left-line" /></a> : <><a href={guestAccess === 'room' ? '/pre-arrival' : '/'}>{guestAccess === 'room' ? 'ورود پیش از رسیدن' : 'ورود مهمان مقیم'} <i className="ri-arrow-left-line" /></a><a href="/staff">ورود پرسنل <i className="ri-shield-user-line" /></a></>}
      </nav>
      {mode === 'guest' && <a className="visitor-link" href={`/visitor${hotelSlug ? `?hotel=${encodeURIComponent(hotelSlug.trim().toLowerCase())}` : ''}`}>مشاهده امکانات هتل بدون ورود <i className="ri-arrow-left-line" /></a>}
      <div className={`api-status ${apiOnline === false ? 'offline' : ''}`}><i />{apiOnline === null ? 'در حال بررسی سرویس' : apiOnline ? 'ارتباط امن برقرار است' : 'سرویس در دسترس نیست'}</div>
    </section>
  </main>
}

function PreArrivalExperience({ stay, token, onLogout }: { stay: Stay; token: string; onLogout: () => void }) {
  const arrival = stay.reservation?.arrivalDate
  const departure = stay.reservation?.departureDate
  const days = arrival ? Math.max(0, Math.ceil((new Date(arrival).getTime() - Date.now()) / 86400000)) : 0
  return <div className="prearrival-app" style={handoffTheme(stay.hotel.primaryColor)}><div className="mobile-surface prearrival-surface">
    <header className="prearrival-top"><div className="auth-brand small"><span>H</span><strong>{stay.hotel.name}</strong></div><button type="button" onClick={onLogout}><i className="ri-logout-box-r-line" /></button></header>
    <main className="prearrival-content view-enter">
      <span className="confirmed-pill"><i className="ri-checkbox-circle-fill" /> رزرو تأیید شد</span>
      <h1>{days ? `تا اقامت شما ${toFaDigits(days)} روز مانده` : 'برای اقامت شما آماده‌ایم'}</h1>
      <p className="reservation-line">{stay.hotel.name} · {stay.room.type || 'اتاق رزرو شده'} · {formatDate(arrival)} تا {formatDate(departure)}</p>
      <GuestCheckInPanel token={token} />
      <PreArrivalOrders token={token} />
    </main>
  </div></div>
}

function StaffDashboard({ session, initialStaff, initialHotel, onLogout }: { session: SessionRecord; initialStaff: Staff; initialHotel: Hotel; onLogout: () => void }) {
  const [hotel, setHotel] = useState(initialHotel)
  const isAdmin = initialStaff.role === 'primary_admin' || initialStaff.role === 'secondary_admin'
  const canManageCatalog = isAdmin || initialStaff.role === 'operations_manager'
  const canReport = isAdmin || initialStaff.role === 'operations_manager'
  const hasReception = isAdmin || initialStaff.role === 'operations_manager' || initialStaff.role === 'reception'
  const isFulfillment = initialStaff.role === 'housekeeping' || initialStaff.role === 'food_beverage'
  const [tab, setTab] = useState<AdminTab>(canReport ? 'dashboard' : 'reception')
  if (isFulfillment) return <StaffRequestQueue token={session.token} staff={initialStaff} hotel={hotel} onLogout={onLogout} />

  const navigation: { id: AdminTab; icon: string; label: string; visible: boolean }[] = [
    { id: 'dashboard', icon: 'ri-dashboard-line', label: 'داشبورد', visible: canReport },
    { id: 'reception', icon: 'ri-hotel-line', label: 'رزرو و پذیرش', visible: hasReception },
    { id: 'catalog', icon: 'ri-layout-grid-line', label: 'کاتالوگ سرویس‌ها', visible: true },
    { id: 'content', icon: 'ri-store-2-line', label: 'امکانات و منو', visible: canManageCatalog },
    { id: 'conversations', icon: 'ri-chat-3-line', label: 'گفتگوهای پذیرش', visible: hasReception },
    { id: 'knowledge', icon: 'ri-sparkling-2-line', label: 'دانش‌نامه AI', visible: true },
    { id: 'reports', icon: 'ri-line-chart-line', label: 'گزارش‌ها', visible: canReport },
    { id: 'security', icon: 'ri-shield-check-line', label: 'امنیت و ممیزی', visible: isAdmin },
    { id: 'branding', icon: 'ri-palette-line', label: 'برندینگ', visible: isAdmin },
    { id: 'staff', icon: 'ri-team-line', label: 'حساب‌های پرسنل', visible: isAdmin },
  ]
  return <div className="admin-app" style={handoffTheme(hotel.primaryColor)}><div className="admin-shell">
    <aside className="admin-sidebar"><div className="admin-logo"><span>H</span><div><strong>{hotel.name}</strong><small>پنل مدیریت</small></div></div><p className="nav-eyebrow">فضای کاری</p><nav>{navigation.filter((item) => item.visible).map((item) => <button type="button" key={item.id} className={tab === item.id ? 'active' : ''} onClick={() => setTab(item.id)}><i className={item.icon} />{item.label}</button>)}</nav><div className="admin-identity"><span>{initialStaff.firstName.slice(0, 1)}{initialStaff.lastName.slice(0, 1)}</span><div><strong>{initialStaff.firstName} {initialStaff.lastName}</strong><small>{roleLabels[initialStaff.role]}</small></div><button type="button" onClick={onLogout} title="خروج"><i className="ri-logout-box-r-line" /></button></div></aside>
    <main className="admin-content">
      {tab === 'dashboard' && <AdminRequestOverview token={session.token} />}
      {tab === 'reception' && <OperationsPanel token={session.token} />}
      {tab === 'catalog' && <ServiceCatalogPanel token={session.token} canManage={canManageCatalog} />}
      {tab === 'content' && <ContentAdminPanel token={session.token} />}
      {tab === 'conversations' && <StaffConversationInbox token={session.token} />}
      {tab === 'branding' && <BrandingPanel hotel={hotel} token={session.token} onUpdated={setHotel} />}
      {tab === 'staff' && <StaffPanel currentStaff={initialStaff} token={session.token} />}
      {tab === 'knowledge' && <KnowledgeAdminPanel token={session.token} canReview={isAdmin} />}
      {tab === 'reports' && <ReportingPanel token={session.token} />}
      {tab === 'security' && <AuditPanel token={session.token} />}
    </main>
  </div></div>
}

function BrandingPanel({ hotel, token, onUpdated }: { hotel: Hotel; token: string; onUpdated: (hotel: Hotel) => void }) {
  const [form, setForm] = useState({ name: hotel.name, logoUrl: hotel.logoUrl, primaryColor: hotel.primaryColor, timezone: hotel.timezone })
  const [notice, setNotice] = useState(''); const [error, setError] = useState(''); const [saving, setSaving] = useState(false)
  const palette = [['#f53d46', 'قرمز تریپ'], ['#17245f', 'سرمه‌ای'], ['#575eff', 'نیلی'], ['#13b476', 'سبز']] as const
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setNotice(''); setError('')
    try { const response = await api<{ hotel: Hotel }>('/api/v1/staff/hotel', { method: 'PATCH', body: JSON.stringify(form) }, token); onUpdated(response.hotel); setNotice('رنگ و نام هتل ذخیره شد.') }
    catch (requestError) { setError(messageFrom(requestError)) } finally { setSaving(false) }
  }
  return <div className="branding-page view-enter"><div className="admin-page-heading"><div><p>تغییرات در همه پنل‌ها اعمال می‌شود</p><h1>برندینگ هتل</h1></div></div><form onSubmit={submit} className="branding-card"><label>نام هتل<input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required /></label><div><label>رنگ اصلی</label><div className="brand-swatches">{palette.map(([color, label]) => <button type="button" key={color} className={form.primaryColor === color ? 'active' : ''} onClick={() => setForm({ ...form, primaryColor: color })}><i style={{ background: color }} /><span>{label}</span></button>)}</div></div><label>آدرس لوگو<input type="url" value={form.logoUrl} onChange={(event) => setForm({ ...form, logoUrl: event.target.value })} placeholder="https://…" /></label><label>منطقه زمانی<input value={form.timezone} onChange={(event) => setForm({ ...form, timezone: event.target.value })} required /></label>{error && <div className="inline-alert error">{error}</div>}{notice && <div className="inline-alert success">{notice}</div>}<div className="brand-preview" style={handoffTheme(form.primaryColor)}><span>پیش‌نمایش دکمه مهمان</span><button type="button">ثبت درخواست</button></div><button className="primary-button" disabled={saving}>{saving ? 'در حال ذخیره…' : 'ذخیره برندینگ'}</button></form></div>
}

function StaffPanel({ currentStaff, token }: { currentStaff: Staff; token: string }) {
  const [staff, setStaff] = useState<Staff[]>([]); const [loading, setLoading] = useState(true); const [showForm, setShowForm] = useState(false); const [error, setError] = useState('')
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '', password: '', role: 'reception' as StaffRole })
  const roles = useMemo(() => { const all = Object.keys(roleLabels) as StaffRole[]; return currentStaff.role === 'primary_admin' ? all : all.filter((role) => role !== 'primary_admin' && role !== 'secondary_admin') }, [currentStaff.role])
  useEffect(() => { api<{ staff: Staff[] }>('/api/v1/staff/users', {}, token).then((response) => setStaff(response.staff)).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false)) }, [token])
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setError(''); try { const response = await api<{ staff: Staff }>('/api/v1/staff/users', { method: 'POST', body: JSON.stringify(form) }, token); setStaff((items) => [...items, response.staff]); setShowForm(false); setForm({ firstName: '', lastName: '', email: '', password: '', role: 'reception' }) } catch (requestError) { setError(messageFrom(requestError)) } }
  return <div className="staff-admin-page view-enter"><div className="admin-page-heading"><div><p>{toFaDigits(staff.length)} حساب سازمانی</p><h1>حساب‌های پرسنل</h1></div><button type="button" className="primary-button compact" onClick={() => setShowForm(!showForm)}>{showForm ? 'بستن' : 'حساب جدید'}</button></div>{showForm && <form onSubmit={submit} className="catalog-form"><div className="form-row"><label>نام<input value={form.firstName} onChange={(event) => setForm({ ...form, firstName: event.target.value })} required /></label><label>نام خانوادگی<input value={form.lastName} onChange={(event) => setForm({ ...form, lastName: event.target.value })} required /></label></div><label>ایمیل<input type="email" dir="ltr" value={form.email} onChange={(event) => setForm({ ...form, email: event.target.value })} required /></label><div className="form-row"><label>رمز عبور<input type="password" minLength={12} value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} required /></label><label>نقش<select value={form.role} onChange={(event) => setForm({ ...form, role: event.target.value as StaffRole })}>{roles.map((role) => <option value={role} key={role}>{roleLabels[role]}</option>)}</select></label></div><button className="primary-button">ایجاد حساب</button></form>}{error && <div className="inline-alert error">{error}</div>}{loading ? <div className="center-loading"><span className="spinner" /> در حال دریافت…</div> : <div className="staff-admin-list">{staff.map((user) => <article key={user.id}><span>{user.firstName.slice(0, 1)}{user.lastName.slice(0, 1)}</span><div><strong>{user.firstName} {user.lastName}</strong><small>{user.email} · {roleLabels[user.role]}</small></div><i className={user.isActive ? 'active' : ''} /></article>)}</div>}</div>
}

export default App
