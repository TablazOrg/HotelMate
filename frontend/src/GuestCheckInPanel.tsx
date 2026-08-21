import { type FormEvent, useEffect, useState } from 'react'
import { api, type OnlineCheckIn } from './api'

const statusLabels: Record<OnlineCheckIn['status'], string> = {
  submitted: 'در انتظار بررسی',
  approved: 'تأیید شده',
  rejected: 'نیازمند ارسال دوباره',
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
    setSubmitting(true)
    setError('')
    setNotice('')
    try {
      const body = new FormData()
      body.append('document', file)
      const response = await api<{ onlineCheckIn: OnlineCheckIn }>(
        '/api/v1/guest/online-check-in',
        { method: 'POST', body },
        token,
      )
      setCheckIn(response.onlineCheckIn)
      setFile(null)
      setNotice('مدرک با موفقیت و به‌صورت خصوصی ارسال شد.')
      const input = document.getElementById('identity-document') as HTMLInputElement | null
      if (input) input.value = ''
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="section-block checkin-panel">
      <div className="section-heading">
        <div><p>پیش از ورود</p><h2>چک‌این آنلاین</h2></div>
        {checkIn && <span className={`status-pill ${checkIn.status}`}>{statusLabels[checkIn.status]}</span>}
      </div>
      <div className="checkin-layout">
        <div className="privacy-card">
          <span className="privacy-icon">✓</span>
          <div>
            <strong>مدرک شما عمومی نیست</strong>
            <p>فایل فقط برای پرسنل مجاز هتل قابل مشاهده است و پس از پایان دوره نگهداری حذف می‌شود.</p>
          </div>
        </div>
        <form onSubmit={submit} className="upload-form">
          <label htmlFor="identity-document">تصویر یا فایل مدرک هویتی
            <input
              id="identity-document"
              type="file"
              accept="application/pdf,image/jpeg,image/png"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              required={!checkIn}
            />
          </label>
          <small>PDF، JPG یا PNG تا سقف ۵ مگابایت</small>
          {checkIn && (
            <div className="document-summary">
              <div><strong>{checkIn.documentName}</strong><small>{Math.max(1, Math.round(checkIn.documentSize / 1024))} کیلوبایت</small></div>
              <span>{new Intl.DateTimeFormat('fa-IR', { dateStyle: 'medium' }).format(new Date(checkIn.submittedAt))}</span>
            </div>
          )}
          {checkIn?.status === 'rejected' && checkIn.reviewNote && <div className="form-error">دلیل رد: {checkIn.reviewNote}</div>}
          {error && <div className="form-error" role="alert">{error}</div>}
          {notice && <div className="form-success">{notice}</div>}
          <button className="primary-button" disabled={submitting || loading || !file}>
            {submitting ? 'در حال ارسال…' : checkIn ? 'جایگزینی مدرک' : 'ارسال امن مدرک'}
          </button>
        </form>
      </div>
    </section>
  )
}
