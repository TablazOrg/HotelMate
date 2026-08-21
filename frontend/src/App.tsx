import { type FormEvent, useEffect, useMemo, useState } from 'react'
import {
  ApiError,
  api,
  clearSession,
  type GuestLoginResponse,
  type Hotel,
  loadSession,
  saveSession,
  type SessionRecord,
  type Staff,
  type StaffLoginResponse,
  type StaffRole,
  type Stay,
} from './api'

type AuthenticatedState = {
  session: SessionRecord
  staff?: Staff
  hotel?: Hotel
  stay?: Stay
}

const roleLabels: Record<StaffRole, string> = {
  primary_admin: 'مدیر اصلی',
  secondary_admin: 'مدیر دوم',
  operations_manager: 'مدیر عملیات',
  reception: 'پذیرش',
  housekeeping: 'خانه‌داری',
  food_beverage: 'غذا و نوشیدنی',
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
    if (!session) {
      setBooting(false)
      return
    }
    const path = session.actorType === 'staff' ? '/api/v1/staff/me' : '/api/v1/guest/me'
    api<{ staff?: Staff; hotel?: Hotel; stay?: Stay }>(path, {}, session.token)
      .then((payload) => setAuthState({ session, ...payload }))
      .catch(() => clearSession())
      .finally(() => setBooting(false))
  }, [])

  function authenticate(response: StaffLoginResponse | GuestLoginResponse) {
    const session: SessionRecord = {
      actorType: response.actorType,
      token: response.token,
      expiresAt: response.expiresAt,
    }
    saveSession(session)
    if (response.actorType === 'staff') {
      setAuthState({ session, staff: response.staff, hotel: response.hotel })
    } else {
      setAuthState({ session, stay: response.stay, hotel: response.stay.hotel })
    }
  }

  function logout() {
    clearSession()
    setAuthState(null)
  }

  if (booting) {
    return <div className="loading-screen"><span className="spinner" />در حال بازیابی نشست…</div>
  }

  if (!authState) return <LoginScreen onAuthenticated={authenticate} />

  if (authState.session.actorType === 'guest' && authState.stay) {
    return <GuestDashboard stay={authState.stay} onLogout={logout} />
  }

  if (authState.staff && authState.hotel) {
    return (
      <StaffDashboard
        session={authState.session}
        initialStaff={authState.staff}
        initialHotel={authState.hotel}
        onLogout={logout}
      />
    )
  }

  return <LoginScreen onAuthenticated={authenticate} />
}

function LoginScreen({ onAuthenticated }: { onAuthenticated: (response: StaffLoginResponse | GuestLoginResponse) => void }) {
  const [mode, setMode] = useState<'guest' | 'staff'>('guest')
  const [hotelSlug, setHotelSlug] = useState('')
  const [roomNumber, setRoomNumber] = useState('')
  const [identityNumber, setIdentityNumber] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [hotel, setHotel] = useState<Hotel | null>(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [apiOnline, setApiOnline] = useState<boolean | null>(null)

  useEffect(() => {
    api<{ status: string }>('/healthz')
      .then(() => setApiOnline(true))
      .catch(() => setApiOnline(false))
  }, [])

  useEffect(() => {
    const slug = hotelSlug.trim().toLowerCase()
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
      setHotel(null)
      return
    }
    const timeout = window.setTimeout(() => {
      api<{ hotel: Hotel }>(`/api/v1/public/hotels/${encodeURIComponent(slug)}`)
        .then((payload) => setHotel(payload.hotel))
        .catch(() => setHotel(null))
    }, 350)
    return () => window.clearTimeout(timeout)
  }, [hotelSlug])

  const primaryColor = hotel?.primaryColor || '#0f766e'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      if (mode === 'staff') {
        const response = await api<StaffLoginResponse>('/api/v1/auth/staff/login', {
          method: 'POST',
          body: JSON.stringify({ hotelSlug, email, password }),
        })
        onAuthenticated(response)
      } else {
        const response = await api<GuestLoginResponse>('/api/v1/auth/guest/login', {
          method: 'POST',
          body: JSON.stringify({ hotelSlug, roomNumber, identityNumber }),
        })
        onAuthenticated(response)
      }
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="auth-page" style={{ '--brand': primaryColor } as React.CSSProperties}>
      <section className="auth-aside">
        <div className="brand-lockup">
          {hotel?.logoUrl ? <img src={hotel.logoUrl} alt="" /> : <span>HM</span>}
          <div><strong>{hotel?.name || 'HotelMate'}</strong><small>همراه هوشمند اقامت شما</small></div>
        </div>
        <div className="auth-message">
          <p className="eyebrow">تجربه‌ای آرام‌تر، پاسخ‌گویی سریع‌تر</p>
          <h1>هر آنچه در هتل نیاز دارید، همین‌جاست.</h1>
          <p>درخواست خدمات، پیگیری لحظه‌ای و ارتباط با تیم هتل بدون تماس تلفنی.</p>
        </div>
        <div className={`api-indicator ${apiOnline === false ? 'offline' : ''}`}>
          <span />{apiOnline === null ? 'در حال بررسی سرویس' : apiOnline ? 'سرویس آنلاین است' : 'سرویس در دسترس نیست'}
        </div>
      </section>

      <section className="auth-panel">
        <div className="auth-card">
          <div className="auth-tabs" role="tablist" aria-label="نوع ورود">
            <button type="button" className={mode === 'guest' ? 'active' : ''} onClick={() => { setMode('guest'); setError('') }}>ورود مهمان</button>
            <button type="button" className={mode === 'staff' ? 'active' : ''} onClick={() => { setMode('staff'); setError('') }}>ورود پرسنل</button>
          </div>
          <div className="form-heading">
            <p>{mode === 'guest' ? 'خوش آمدید' : 'پنل عملیات'}</p>
            <h2>{mode === 'guest' ? 'ورود به اقامت' : 'ورود پرسنل هتل'}</h2>
          </div>
          <form onSubmit={submit}>
            <label>شناسه هتل<input value={hotelSlug} onChange={(e) => setHotelSlug(e.target.value)} placeholder="example-hotel" autoComplete="organization" required /></label>
            {mode === 'guest' ? (
              <>
                <label>شماره اتاق<input value={roomNumber} onChange={(e) => setRoomNumber(e.target.value)} inputMode="numeric" autoComplete="off" required /></label>
                <label>کد ملی یا شماره پاسپورت<input value={identityNumber} onChange={(e) => setIdentityNumber(e.target.value)} autoComplete="off" required /></label>
              </>
            ) : (
              <>
                <label>ایمیل سازمانی<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="username" required /></label>
                <label>رمز عبور<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required /></label>
              </>
            )}
            {error && <div className="form-error" role="alert">{error}</div>}
            <button className="primary-button" disabled={submitting}>{submitting ? 'در حال ورود…' : 'ورود امن'}</button>
          </form>
          <p className="privacy-note">اطلاعات هویتی شما ذخیره یا در گزارش‌ها نمایش داده نمی‌شود.</p>
        </div>
      </section>
    </main>
  )
}

function AppHeader({ hotel, title, subtitle, onLogout }: { hotel: Hotel; title: string; subtitle: string; onLogout: () => void }) {
  return (
    <header className="app-header" style={{ '--brand': hotel.primaryColor } as React.CSSProperties}>
      <div className="brand-lockup compact">
        {hotel.logoUrl ? <img src={hotel.logoUrl} alt="" /> : <span>HM</span>}
        <div><strong>{hotel.name}</strong><small>{subtitle}</small></div>
      </div>
      <div className="header-title"><h1>{title}</h1></div>
      <button className="ghost-button" onClick={onLogout}>خروج</button>
    </header>
  )
}

function GuestDashboard({ stay, onLogout }: { stay: Stay; onLogout: () => void }) {
  const services = [
    ['نظافت اتاق', 'درخواست رسیدگی خانه‌داری'], ['آب معدنی', 'تحویل آب به اتاق'],
    ['چای و قهوه', 'سفارش نوشیدنی گرم'], ['لوازم بهداشتی', 'حوله و اقلام مصرفی'],
    ['خروج دیرهنگام', 'بررسی امکان تمدید'], ['ترانسفر', 'هماهنگی رفت‌وآمد'],
  ]
  return (
    <div className="app-shell" style={{ '--brand': stay.hotel.primaryColor } as React.CSSProperties}>
      <AppHeader hotel={stay.hotel} title={`سلام ${stay.guest.firstName}`} subtitle={`اتاق ${stay.room.number}`} onLogout={onLogout} />
      <main className="dashboard">
        <section className="welcome-card">
          <div><p className="eyebrow">اقامت فعال</p><h2>چه کمکی از ما برمی‌آید؟</h2><p>درخواست‌های شما مستقیماً به تیم مربوطه ارسال می‌شود.</p></div>
          <div className="room-badge"><small>شماره اتاق</small><strong>{stay.room.number}</strong></div>
        </section>
        <section className="section-block">
          <div className="section-heading"><div><p>دسترسی سریع</p><h2>خدمات پرکاربرد</h2></div><span className="coming-label">فعال‌سازی در M3</span></div>
          <div className="service-grid">
            {services.map(([name, description], index) => <button disabled key={name}><span>{String(index + 1).padStart(2, '0')}</span><strong>{name}</strong><small>{description}</small></button>)}
          </div>
        </section>
      </main>
    </div>
  )
}

function StaffDashboard({ session, initialStaff, initialHotel, onLogout }: { session: SessionRecord; initialStaff: Staff; initialHotel: Hotel; onLogout: () => void }) {
  const [hotel, setHotel] = useState(initialHotel)
  const isAdmin = initialStaff.role === 'primary_admin' || initialStaff.role === 'secondary_admin'
  return (
    <div className="app-shell" style={{ '--brand': hotel.primaryColor } as React.CSSProperties}>
      <AppHeader hotel={hotel} title="پنل عملیات هتل" subtitle={roleLabels[initialStaff.role]} onLogout={onLogout} />
      <main className="dashboard staff-dashboard">
        <section className="welcome-card staff-welcome">
          <div><p className="eyebrow">{roleLabels[initialStaff.role]}</p><h2>{initialStaff.firstName} {initialStaff.lastName}</h2><p>دسترسی‌ها به هتل {hotel.name} محدود شده‌اند.</p></div>
          <span className="security-badge">نشست امن و ثبت‌شده</span>
        </section>
        {isAdmin ? (
          <div className="admin-grid">
            <BrandingPanel hotel={hotel} token={session.token} onUpdated={setHotel} />
            <StaffPanel currentStaff={initialStaff} token={session.token} />
          </div>
        ) : (
          <section className="empty-panel"><h2>صف عملیات</h2><p>صف درخواست‌های مرتبط با نقش شما در Milestone 3 فعال می‌شود.</p></section>
        )}
      </main>
    </div>
  )
}

function BrandingPanel({ hotel, token, onUpdated }: { hotel: Hotel; token: string; onUpdated: (hotel: Hotel) => void }) {
  const [form, setForm] = useState({ name: hotel.name, logoUrl: hotel.logoUrl, primaryColor: hotel.primaryColor, timezone: hotel.timezone })
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setNotice(''); setError('')
    try {
      const response = await api<{ hotel: Hotel }>('/api/v1/staff/hotel', { method: 'PATCH', body: JSON.stringify(form) }, token)
      onUpdated(response.hotel); setNotice('تنظیمات برند ذخیره شد.')
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setSaving(false) }
  }

  return (
    <section className="admin-panel">
      <div className="section-heading"><div><p>تنظیمات</p><h2>هویت بصری هتل</h2></div><span className="color-preview" style={{ background: form.primaryColor }} /></div>
      <form onSubmit={submit} className="compact-form">
        <label>نام هتل<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
        <label>آدرس لوگو<input type="url" value={form.logoUrl} onChange={(e) => setForm({ ...form, logoUrl: e.target.value })} placeholder="https://…" /></label>
        <div className="form-row"><label>رنگ اصلی<input type="color" value={form.primaryColor} onChange={(e) => setForm({ ...form, primaryColor: e.target.value })} /></label><label>منطقه زمانی<input value={form.timezone} onChange={(e) => setForm({ ...form, timezone: e.target.value })} required /></label></div>
        {error && <div className="form-error">{error}</div>}{notice && <div className="form-success">{notice}</div>}
        <button className="primary-button" disabled={saving}>{saving ? 'در حال ذخیره…' : 'ذخیره تنظیمات'}</button>
      </form>
    </section>
  )
}

function StaffPanel({ currentStaff, token }: { currentStaff: Staff; token: string }) {
  const [staff, setStaff] = useState<Staff[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '', password: '', role: 'reception' as StaffRole })

  const roles = useMemo(() => {
    const all = Object.keys(roleLabels) as StaffRole[]
    return currentStaff.role === 'primary_admin' ? all : all.filter((role) => role !== 'primary_admin' && role !== 'secondary_admin')
  }, [currentStaff.role])

  useEffect(() => {
    api<{ staff: Staff[] }>('/api/v1/staff/users', {}, token)
      .then((response) => setStaff(response.staff))
      .catch((requestError) => setError(messageFrom(requestError)))
      .finally(() => setLoading(false))
  }, [token])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('')
    try {
      const response = await api<{ staff: Staff }>('/api/v1/staff/users', { method: 'POST', body: JSON.stringify(form) }, token)
      setStaff((items) => [...items, response.staff]); setShowForm(false)
      setForm({ firstName: '', lastName: '', email: '', password: '', role: 'reception' })
    } catch (requestError) { setError(messageFrom(requestError)) }
  }

  return (
    <section className="admin-panel">
      <div className="section-heading"><div><p>دسترسی‌ها</p><h2>حساب‌های پرسنل</h2></div><button className="small-button" onClick={() => setShowForm(!showForm)}>{showForm ? 'بستن' : 'حساب جدید'}</button></div>
      {showForm && <form onSubmit={submit} className="compact-form inset-form">
        <div className="form-row"><label>نام<input value={form.firstName} onChange={(e) => setForm({ ...form, firstName: e.target.value })} required /></label><label>نام خانوادگی<input value={form.lastName} onChange={(e) => setForm({ ...form, lastName: e.target.value })} required /></label></div>
        <label>ایمیل<input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></label>
        <div className="form-row"><label>رمز عبور<input type="password" minLength={12} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></label><label>نقش<select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value as StaffRole })}>{roles.map((role) => <option value={role} key={role}>{roleLabels[role]}</option>)}</select></label></div>
        <button className="primary-button">ایجاد حساب</button>
      </form>}
      {error && <div className="form-error">{error}</div>}
      {loading ? <p className="muted">در حال دریافت حساب‌ها…</p> : <div className="staff-list">{staff.map((user) => <div key={user.id}><span className="avatar">{user.firstName.slice(0, 1)}{user.lastName.slice(0, 1)}</span><div><strong>{user.firstName} {user.lastName}</strong><small>{user.email} · {roleLabels[user.role]}</small></div><i className={user.isActive ? 'active' : ''} /></div>)}</div>}
    </section>
  )
}

export default App
