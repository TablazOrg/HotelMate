import { type FormEvent, useCallback, useEffect, useState } from 'react'
import {
  api,
  apiBlob,
  type OnlineCheckIn,
  type Reservation,
  type Room,
  type RoomStatus,
} from './api'

type Tab = 'reservations' | 'rooms' | 'checkins'

const roomStatusLabels: Record<RoomStatus, string> = {
  available: 'آماده',
  occupied: 'اشغال',
  cleaning: 'نیازمند نظافت',
  out_of_service: 'خارج از سرویس',
}

const reservationStatusLabels: Record<Reservation['status'], string> = {
  pending: 'در انتظار تأیید',
  confirmed: 'تأیید شده',
  cancelled: 'لغو شده',
  completed: 'پایان یافته',
}

const checkInStatusLabels: Record<OnlineCheckIn['status'], string> = {
  submitted: 'در انتظار بررسی',
  approved: 'تأیید شده',
  rejected: 'رد شده',
}

const formatDate = (value: string) => new Intl.DateTimeFormat('fa-IR', { dateStyle: 'medium' }).format(new Date(value))
const messageFrom = (error: unknown) => error instanceof Error ? error.message : 'عملیات انجام نشد.'

export function OperationsPanel({ token }: { token: string }) {
  const [tab, setTab] = useState<Tab>('reservations')
  const [rooms, setRooms] = useState<Room[]>([])
  const [reservations, setReservations] = useState<Reservation[]>([])
  const [checkIns, setCheckIns] = useState<OnlineCheckIn[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    setError('')
    try {
      const [roomResponse, reservationResponse, checkInResponse] = await Promise.all([
        api<{ rooms: Room[] }>('/api/v1/staff/rooms', {}, token),
        api<{ reservations: Reservation[] }>('/api/v1/staff/reservations', {}, token),
        api<{ onlineCheckIns: OnlineCheckIn[] }>('/api/v1/staff/online-check-ins', {}, token),
      ])
      setRooms(roomResponse.rooms)
      setReservations(reservationResponse.reservations)
      setCheckIns(checkInResponse.onlineCheckIns)
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => { void refresh() }, [refresh])

  return (
    <section className="section-block operations-panel">
      <div className="section-heading operations-heading">
        <div><p>میز پذیرش</p><h2>چرخه رزرو و اقامت</h2></div>
        <button type="button" className="small-button" onClick={() => void refresh()} disabled={loading}>به‌روزرسانی</button>
      </div>
      <div className="operations-tabs" role="tablist" aria-label="عملیات پذیرش">
        <button type="button" className={tab === 'reservations' ? 'active' : ''} onClick={() => setTab('reservations')}>رزروها <span>{reservations.length}</span></button>
        <button type="button" className={tab === 'rooms' ? 'active' : ''} onClick={() => setTab('rooms')}>اتاق‌ها <span>{rooms.length}</span></button>
        <button type="button" className={tab === 'checkins' ? 'active' : ''} onClick={() => setTab('checkins')}>چک‌این آنلاین <span>{checkIns.filter((item) => item.status === 'submitted').length}</span></button>
      </div>
      {error && <div className="form-error" role="alert">{error}</div>}
      {loading ? <p className="muted">در حال دریافت اطلاعات پذیرش…</p> : (
        <>
          {tab === 'rooms' && <RoomsTab token={token} rooms={rooms} onChanged={refresh} />}
          {tab === 'reservations' && <ReservationsTab token={token} rooms={rooms} reservations={reservations} onChanged={refresh} />}
          {tab === 'checkins' && <CheckInsTab token={token} checkIns={checkIns} onChanged={refresh} />}
        </>
      )}
    </section>
  )
}

function RoomsTab({ token, rooms, onChanged }: { token: string; rooms: Room[]; onChanged: () => Promise<void> }) {
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ number: '', floor: '1', type: '' })
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  async function createRoom(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy('create'); setError('')
    try {
      await api('/api/v1/staff/rooms', { method: 'POST', body: JSON.stringify({ ...form, floor: Number(form.floor) }) }, token)
      setForm({ number: '', floor: '1', type: '' }); setShowForm(false); await onChanged()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function setStatus(room: Room, status: RoomStatus) {
    setBusy(room.id); setError('')
    try {
      await api(`/api/v1/staff/rooms/${room.id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }, token)
      await onChanged()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  return <div className="operations-content">
    <div className="subsection-bar"><div><strong>وضعیت اتاق‌ها</strong><small>وضعیت «اشغال» فقط با چک‌این تغییر می‌کند.</small></div><button type="button" className="small-button" onClick={() => setShowForm(!showForm)}>{showForm ? 'بستن' : 'اتاق جدید'}</button></div>
    {showForm && <form className="inline-form" onSubmit={createRoom}>
      <label>شماره اتاق<input value={form.number} onChange={(event) => setForm({ ...form, number: event.target.value })} required /></label>
      <label>طبقه<input type="number" min="-5" max="200" value={form.floor} onChange={(event) => setForm({ ...form, floor: event.target.value })} required /></label>
      <label>نوع اتاق<input value={form.type} onChange={(event) => setForm({ ...form, type: event.target.value })} placeholder="دو تخته" /></label>
      <button className="primary-button" disabled={busy === 'create'}>{busy === 'create' ? 'در حال ایجاد…' : 'ایجاد اتاق'}</button>
    </form>}
    {error && <div className="form-error">{error}</div>}
    <div className="room-grid">
      {rooms.map((room) => <article className="room-card" key={room.id}>
        <div><small>اتاق</small><strong>{room.number}</strong><span>طبقه {room.floor} · {room.type || 'استاندارد'}</span></div>
        <label className={`status-select ${room.status}`}>وضعیت
          <select value={room.status} disabled={room.status === 'occupied' || busy === room.id} onChange={(event) => void setStatus(room, event.target.value as RoomStatus)}>
            <option value="available">{roomStatusLabels.available}</option>
            {room.status === 'occupied' && <option value="occupied">{roomStatusLabels.occupied}</option>}
            <option value="cleaning">{roomStatusLabels.cleaning}</option>
            <option value="out_of_service">{roomStatusLabels.out_of_service}</option>
          </select>
        </label>
      </article>)}
      {!rooms.length && <div className="empty-state">هنوز اتاقی ثبت نشده است.</div>}
    </div>
  </div>
}

function ReservationsTab({ token, rooms, reservations, onChanged }: { token: string; rooms: Room[]; reservations: Reservation[]; onChanged: () => Promise<void> }) {
  const [showForm, setShowForm] = useState(false)
  const today = new Date().toISOString().slice(0, 10)
  const [form, setForm] = useState({ firstName: '', lastName: '', identityType: 'national_id', identityNumber: '', phone: '', roomId: '', arrivalDate: today, departureDate: '' })
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  async function createReservation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy('create'); setError('')
    try {
      await api('/api/v1/staff/reservations', { method: 'POST', body: JSON.stringify({
        guest: { firstName: form.firstName, lastName: form.lastName, identityType: form.identityType, identityNumber: form.identityNumber, phone: form.phone },
        roomId: form.roomId, arrivalDate: form.arrivalDate, departureDate: form.departureDate,
      }) }, token)
      setForm({ firstName: '', lastName: '', identityType: 'national_id', identityNumber: '', phone: '', roomId: '', arrivalDate: today, departureDate: '' })
      setShowForm(false); await onChanged()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function transition(reservation: Reservation, action: 'confirm' | 'check-in' | 'check-out') {
    const stay = reservation.stay
    const path = action === 'confirm'
      ? `/api/v1/staff/reservations/${reservation.id}/confirm`
      : `/api/v1/staff/stays/${stay?.id}/${action}`
    setBusy(reservation.id); setError('')
    try {
      await api(path, { method: 'POST', body: action === 'check-in' ? JSON.stringify({ roomId: reservation.room?.id }) : undefined }, token)
      await onChanged()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  return <div className="operations-content">
    <div className="subsection-bar"><div><strong>رزروهای هتل</strong><small>تأیید رزرو، اقامت پیش از ورود را به‌صورت خودکار ایجاد می‌کند.</small></div><button type="button" className="small-button" onClick={() => setShowForm(!showForm)}>{showForm ? 'بستن' : 'رزرو جدید'}</button></div>
    {showForm && <form className="reservation-form" onSubmit={createReservation}>
      <div className="form-row"><label>نام مهمان<input value={form.firstName} onChange={(event) => setForm({ ...form, firstName: event.target.value })} required /></label><label>نام خانوادگی<input value={form.lastName} onChange={(event) => setForm({ ...form, lastName: event.target.value })} required /></label></div>
      <div className="form-row"><label>نوع مدرک<select value={form.identityType} onChange={(event) => setForm({ ...form, identityType: event.target.value })}><option value="national_id">کد ملی</option><option value="passport">پاسپورت</option></select></label><label>شماره مدرک<input value={form.identityNumber} onChange={(event) => setForm({ ...form, identityNumber: event.target.value })} required /></label></div>
      <div className="form-row"><label>شماره تماس<input value={form.phone} onChange={(event) => setForm({ ...form, phone: event.target.value })} /></label><label>اتاق<select value={form.roomId} onChange={(event) => setForm({ ...form, roomId: event.target.value })} required><option value="">انتخاب کنید</option>{rooms.filter((room) => room.status !== 'out_of_service').map((room) => <option key={room.id} value={room.id}>اتاق {room.number} · {roomStatusLabels[room.status]}</option>)}</select></label></div>
      <div className="form-row"><label>تاریخ ورود<input type="date" min={today} value={form.arrivalDate} onChange={(event) => setForm({ ...form, arrivalDate: event.target.value })} required /></label><label>تاریخ خروج<input type="date" min={form.arrivalDate || today} value={form.departureDate} onChange={(event) => setForm({ ...form, departureDate: event.target.value })} required /></label></div>
      <button className="primary-button" disabled={busy === 'create'}>{busy === 'create' ? 'در حال ثبت…' : 'ثبت رزرو'}</button>
    </form>}
    {error && <div className="form-error">{error}</div>}
    <div className="reservation-list">
      {reservations.map((reservation) => {
        const stay = reservation.stay
        const action = reservation.status === 'pending' ? 'confirm' : stay?.status === 'pre_arrival' ? 'check-in' : stay?.status === 'active' ? 'check-out' : null
        const actionLabel = action === 'confirm' ? 'تأیید رزرو' : action === 'check-in' ? 'ثبت ورود' : action === 'check-out' ? 'ثبت خروج' : ''
        return <article className="reservation-card" key={reservation.id}>
          <div className="reservation-main"><span className="avatar">{reservation.guest.firstName.slice(0, 1)}{reservation.guest.lastName.slice(0, 1)}</span><div><strong>{reservation.guest.firstName} {reservation.guest.lastName}</strong><small>{formatDate(reservation.arrivalDate)} تا {formatDate(reservation.departureDate)}</small></div></div>
          <div className="reservation-meta"><span className={`status-pill ${reservation.status}`}>{reservationStatusLabels[reservation.status]}</span><strong>اتاق {reservation.room?.number ?? '—'}</strong><code>{reservation.confirmationCode}</code></div>
          <div className="reservation-action">{action ? <button type="button" className="small-button solid" onClick={() => void transition(reservation, action)} disabled={busy === reservation.id}>{busy === reservation.id ? 'در حال انجام…' : actionLabel}</button> : <span className="muted">عملیات تکمیل شده</span>}</div>
        </article>
      })}
      {!reservations.length && <div className="empty-state">هنوز رزروی ثبت نشده است.</div>}
    </div>
  </div>
}

function CheckInsTab({ token, checkIns, onChanged }: { token: string; checkIns: OnlineCheckIn[]; onChanged: () => Promise<void> }) {
  const [notes, setNotes] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  async function review(item: OnlineCheckIn, status: 'approved' | 'rejected') {
    const note = notes[item.id]?.trim() ?? ''
    if (status === 'rejected' && !note) { setError('برای رد مدرک، دلیل را وارد کنید.'); return }
    setBusy(item.id); setError('')
    try {
      await api(`/api/v1/staff/online-check-ins/${item.id}/review`, { method: 'POST', body: JSON.stringify({ status, note }) }, token)
      await onChanged()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function download(item: OnlineCheckIn) {
    setBusy(`download-${item.id}`); setError('')
    try {
      const { blob, filename } = await apiBlob(`/api/v1/staff/online-check-ins/${item.id}/document`, token)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a'); link.href = url; link.download = filename; link.click()
      URL.revokeObjectURL(url)
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  return <div className="operations-content">
    <div className="subsection-bar"><div><strong>مدارک چک‌این آنلاین</strong><small>فایل‌ها فقط از این نشست احراز هویت‌شده دریافت می‌شوند.</small></div></div>
    {error && <div className="form-error">{error}</div>}
    <div className="checkin-list">
      {checkIns.map((item) => <article className="checkin-card" key={item.id}>
        <div className="checkin-person"><span className="avatar">{item.stay?.guest.firstName.slice(0, 1)}{item.stay?.guest.lastName.slice(0, 1)}</span><div><strong>{item.stay?.guest.firstName} {item.stay?.guest.lastName}</strong><small>اتاق {item.stay?.room.number} · ارسال {formatDate(item.submittedAt)}</small></div></div>
        <div className="checkin-document"><span className={`status-pill ${item.status}`}>{checkInStatusLabels[item.status]}</span><button type="button" className="link-button" disabled={!item.documentAvailable || busy === `download-${item.id}`} onClick={() => void download(item)}>{busy === `download-${item.id}` ? 'دریافت…' : item.documentName}</button></div>
        {item.status === 'submitted' ? <div className="review-controls"><input aria-label="یادداشت بررسی" placeholder="یادداشت (برای رد الزامی)" value={notes[item.id] ?? ''} onChange={(event) => setNotes({ ...notes, [item.id]: event.target.value })} /><div><button type="button" className="small-button approve" disabled={busy === item.id} onClick={() => void review(item, 'approved')}>تأیید</button><button type="button" className="small-button reject" disabled={busy === item.id} onClick={() => void review(item, 'rejected')}>رد</button></div></div> : item.reviewNote && <p className="review-note">یادداشت بررسی: {item.reviewNote}</p>}
      </article>)}
      {!checkIns.length && <div className="empty-state">هنوز مدرکی برای بررسی ارسال نشده است.</div>}
    </div>
  </div>
}
