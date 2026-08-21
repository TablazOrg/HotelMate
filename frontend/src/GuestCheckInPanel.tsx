import { type FormEvent, useEffect, useState } from 'react'
import { api, type OnlineCheckIn } from './api'
import { formatDate, toFaDigits } from './handoff'

const statusLabels: Record<OnlineCheckIn['status'], string> = {
  submitted: 'در انتظار بررسی پذیرش', approved: 'مدرک تأیید شده', rejected: 'نیازمند ارسال دوباره',
}

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'ارسال مدرک انجام نشد.'
}

export function GuestCheckInPanel({ token }: { token: string }) {
  const [checkIn, setCheckIn] = useState<OnlineCheckIn | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    api<{ onlineCheckIn: OnlineCheckIn | null }>('/api/v1/guest/online-check-in', {}, token)
      .then((response) => setCheckIn(response.onlineCheckIn))
      .catch((requestError) => setError(messageFrom(requestError)))
      .finally(() => setLoading(false))
  }, [token])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!file) return
    setSubmitting(true); setError(''); setNotice('')
    try {
      const body = new FormData(); body.append('document', file)
      const response = await api<{ onlineCheckIn: OnlineCheckIn }>('/api/v1/guest/online-check-in', { method: 'POST', body }, token)
      setCheckIn(response.onlineCheckIn); setFile(null); setNotice('مدرک به‌صورت امن برای پذیرش ارسال شد.')
      const input = document.getElementById('identity-document') as HTMLInputElement | null
      if (input) input.value = ''
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setSubmitting(false)
    }
  }

  const documentDone = checkIn?.status === 'approved'
  const documentWaiting = checkIn?.status === 'submitted'

  return <section className="checkin-card-handoff">
    <header><span><i className="ri-passport-line" /></span><div><h2>چک‌این آنلاین</h2><p>بدون معطلی در پذیرش — کلید اتاق آماده باشد</p></div>{checkIn && <b className={`status-pill ${checkIn.status}`}>{statusLabels[checkIn.status]}</b>}</header>
    <div className="checkin-timeline">
      <TimelineStep state="done" icon="ri-user-line" title="اطلاعات مسافران" description="اطلاعات رزرو با موفقیت دریافت شد" />
      <TimelineStep state={documentDone ? 'done' : 'current'} icon={documentWaiting ? 'ri-time-line' : 'ri-id-card-line'} title="بارگذاری مدرک هویتی" description={documentWaiting ? 'مدرک برای بررسی پذیرش ارسال شده است' : 'کارت ملی یا پاسپورت — امن و رمزنگاری‌شده'} />
      <TimelineStep state={documentDone ? 'current' : 'upcoming'} icon="ri-time-line" title="اعلام ساعت ورود" description="در مرحله بعد توسعه فعال می‌شود" last />
    </div>

    <form onSubmit={submit} className="handoff-upload-form">
      {checkIn && <div className="secure-document"><i className="ri-shield-check-line" /><div><strong>{checkIn.documentName}</strong><small>{toFaDigits(Math.max(1, Math.round(checkIn.documentSize / 1024)))} کیلوبایت · ارسال {formatDate(checkIn.submittedAt)}</small></div></div>}
      {checkIn?.status === 'rejected' && checkIn.reviewNote && <div className="inline-alert error"><i className="ri-error-warning-line" />دلیل رد: {checkIn.reviewNote}</div>}
      {error && <div className="inline-alert error" role="alert">{error}</div>}
      {notice && <div className="inline-alert success"><i className="ri-checkbox-circle-line" />{notice}</div>}
      {!documentDone && <label className="file-picker" htmlFor="identity-document"><i className="ri-upload-cloud-2-line" /><span>{file ? file.name : checkIn ? 'انتخاب فایل جایگزین' : 'انتخاب مدرک هویتی'}</span><small>PDF، JPG یا PNG تا سقف ۵ مگابایت</small><input id="identity-document" type="file" accept="application/pdf,image/jpeg,image/png" onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></label>}
      <button className={documentDone ? 'success-button' : 'primary-button'} disabled={loading || submitting || documentDone || !file}>
        {submitting ? 'در حال ارسال امن…' : documentDone ? 'مدرک هویتی تأیید شد ✓' : documentWaiting && !file ? 'در انتظار بررسی پذیرش' : checkIn ? 'ارسال مدرک جایگزین' : 'ادامه: بارگذاری مدرک هویتی'}
      </button>
      <p className="privacy-line"><i className="ri-lock-line" /> فایل فقط برای پرسنل مجاز قابل مشاهده است و طبق سیاست نگهداری حذف می‌شود.</p>
    </form>
  </section>
}

function TimelineStep({ state, icon, title, description, last = false }: { state: 'done' | 'current' | 'upcoming'; icon: string; title: string; description: string; last?: boolean }) {
  return <div className={`timeline-step ${state}`}><div className="timeline-marker"><span>{state === 'done' ? <i className="ri-check-line" /> : <i className={icon} />}</span>{!last && <b />}</div><div><strong>{title}</strong><p>{description}</p></div></div>
}
