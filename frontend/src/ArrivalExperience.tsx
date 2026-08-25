import { type FormEvent, type PointerEvent as ReactPointerEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  api, apiBlob, type ArrivalAnalytics, type ArrivalJourney,
  type ArrivalSettings, type GuestLoginResponse, type Reservation,
} from './api'
import { formatDate, toFaDigits } from './handoff'

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'ارتباط با سرور انجام نشد.'
}

const statusLabels: Record<ArrivalJourney['status'], string> = {
  draft: 'در حال تکمیل', submitted: 'در انتظار بررسی', needs_changes: 'نیازمند اصلاح', approved: 'تأیید شده',
  arrival_pending: 'در انتظار ورود', room_ready: 'اتاق آماده است', checked_in: 'ورود ثبت شد',
  expired: 'منقضی شده', cancelled: 'لغو شده',
}

const methodLabels: Record<string, string> = {
  car: 'خودروی شخصی', taxi: 'تاکسی', flight: 'هواپیما', train: 'قطار', bus: 'اتوبوس', walk: 'پیاده', other: 'سایر',
}

export function InvitationExchange({ onAuthenticated }: { onAuthenticated: (response: GuestLoginResponse) => void }) {
  const fragment = useMemo(() => new URLSearchParams(window.location.hash.replace(/^#/, '')), [])
  const [invitationToken, setInvitationToken] = useState(fragment.get('invite') ?? '')
  const [recoveryCode, setRecoveryCode] = useState(fragment.get('recovery') ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const attempted = useRef(false)

  const exchange = useCallback(async (payload: { invitationToken?: string; recoveryCode?: string }) => {
    setBusy(true); setError('')
    try {
      const response = await api<GuestLoginResponse>('/api/v1/check-in/exchange', { method: 'POST', body: JSON.stringify(payload) })
      onAuthenticated(response)
    } catch (requestError) {
      setError(messageFrom(requestError))
    } finally { setBusy(false) }
  }, [onAuthenticated])

  useEffect(() => {
    window.history.replaceState({}, '', '/check-in')
    if (attempted.current) return
    attempted.current = true
    if (invitationToken) void exchange({ invitationToken })
    else if (recoveryCode) void exchange({ recoveryCode })
  }, [exchange, invitationToken, recoveryCode])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    void exchange(invitationToken.trim() ? { invitationToken: invitationToken.trim() } : { recoveryCode: recoveryCode.trim().toUpperCase() })
  }

  return <main className="arrival-exchange-shell">
    <section className="arrival-exchange-card" aria-labelledby="exchange-title">
      <a className="arrival-wordmark" href="/"><img src="/hotelmate-logo.svg" alt="" /><strong>HotelMate</strong></a>
      <span className="arrival-lock"><i className="ri-shield-keyhole-line" /></span>
      <h1 id="exchange-title">ورود امن به چک‌این آنلاین</h1>
      <p>لینک دعوت پس از باز شدن از نوار آدرس پاک می‌شود. اگر لینک در دسترس نیست، کد بازیابی روی دعوت یا QR را وارد کنید.</p>
      {busy && <div className="arrival-loading" role="status"><span className="spinner" />در حال بررسی دعوت امن…</div>}
      {!busy && <form onSubmit={submit} className="arrival-exchange-form">
        <label>کد دعوت امن<textarea dir="ltr" rows={3} value={invitationToken} onChange={(event) => { setInvitationToken(event.target.value); if (event.target.value) setRecoveryCode('') }} placeholder="eyJ…" /></label>
        <span>یا</span>
        <label>کد بازیابی<input dir="ltr" autoComplete="one-time-code" value={recoveryCode} onChange={(event) => { setRecoveryCode(event.target.value.toUpperCase()); if (event.target.value) setInvitationToken('') }} placeholder="ABCD1234…" /></label>
        {error && <div className="inline-alert error" role="alert"><i className="ri-error-warning-line" />{error}</div>}
        <button className="primary-button" disabled={!invitationToken.trim() && !recoveryCode.trim()}>ادامه امن</button>
      </form>}
      <small><i className="ri-lock-line" /> اطلاعات هویتی در لینک یا کد بازیابی قرار نمی‌گیرد.</small>
    </section>
  </main>
}

type DetailsForm = {
  contactPhone: string; contactEmail: string; nationality: string; arrivalEta: string; arrivalMethod: string
  transportDetails: string; accessibilityNeeds: string; specialRequests: string
}

export function ArrivalWizard({ token }: { token: string }) {
  const [arrival, setArrival] = useState<ArrivalJourney | null>(null)
  const [settings, setSettings] = useState<ArrivalSettings | null>(null)
  const [step, setStep] = useState(1)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [form, setForm] = useState<DetailsForm>({ contactPhone: '', contactEmail: '', nationality: '', arrivalEta: '', arrivalMethod: '', transportDetails: '', accessibilityNeeds: '', specialRequests: '' })
  const [companions, setCompanions] = useState<{ firstName: string; lastName: string; relationship: string; nationality: string; dateOfBirth: string; documentRequired: boolean }[]>([])
  const hydrated = useRef(false)
  const autosaveTimer = useRef(0)
  const lastSavedDetails = useRef('')
  const validationRef = useRef<HTMLDivElement>(null)

  const hydrate = useCallback((journey: ArrivalJourney, nextSettings?: ArrivalSettings) => {
    setArrival(journey); if (nextSettings) setSettings(nextSettings)
    setStep(Math.min(3, Math.max(1, journey.currentStep || 1)))
    setForm({
      contactPhone: journey.contactPhone ?? '', contactEmail: journey.contactEmail ?? '', nationality: journey.nationality ?? '',
      arrivalEta: journey.arrivalEta ? new Date(journey.arrivalEta).toISOString().slice(0, 16) : '', arrivalMethod: journey.arrivalMethod ?? '',
      transportDetails: journey.transportDetails ?? '', accessibilityNeeds: journey.accessibilityNeeds ?? '', specialRequests: journey.specialRequests ?? '',
    })
    setCompanions(journey.companions.map((item) => ({ firstName: item.firstName, lastName: item.lastName, relationship: item.relationship, nationality: item.nationality, dateOfBirth: item.dateOfBirth?.slice(0, 10) ?? '', documentRequired: item.documentRequired })))
    lastSavedDetails.current = JSON.stringify({
      contactPhone: journey.contactPhone ?? '', contactEmail: journey.contactEmail ?? '', nationality: journey.nationality ?? '',
      arrivalEta: journey.arrivalEta ? new Date(journey.arrivalEta).toISOString().slice(0, 16) : '', arrivalMethod: journey.arrivalMethod ?? '',
      transportDetails: journey.transportDetails ?? '', accessibilityNeeds: journey.accessibilityNeeds ?? '', specialRequests: journey.specialRequests ?? '',
    })
    hydrated.current = true
  }, [])

  const refresh = useCallback(async () => {
    try {
      const response = await api<{ arrival: ArrivalJourney; settings: ArrivalSettings }>('/api/v1/guest/arrival', {}, token)
      hydrate(response.arrival, response.settings)
      void api('/api/v1/guest/arrival/events', { method: 'POST', body: JSON.stringify({ type: 'start', step: response.arrival.currentStep, durationMs: 0 }) }, token).catch(() => undefined)
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setLoading(false) }
  }, [hydrate, token])

  useEffect(() => { void refresh() }, [refresh])

  const saveDetails = useCallback(async (advance: boolean) => {
    if (!arrival || !hydrated.current) return
    setBusy(advance ? 'details' : 'autosave'); if (advance) { setError(''); setNotice('') }
    try {
      const response = await api<{ arrival: ArrivalJourney }>('/api/v1/guest/arrival/details', { method: 'PATCH', body: JSON.stringify({
        ...form, arrivalEta: form.arrivalEta ? new Date(form.arrivalEta).toISOString() : '', answers: {},
      }) }, token)
      lastSavedDetails.current = JSON.stringify(form)
      setArrival(response.arrival)
      if (advance) { setStep(2); setNotice('اطلاعات مرحله اول ذخیره شد.') }
    } catch (requestError) {
      if (advance) { setError(messageFrom(requestError)); window.setTimeout(() => validationRef.current?.focus(), 0) }
      void api('/api/v1/guest/arrival/events', { method: 'POST', body: JSON.stringify({ type: 'error', step: 1, durationMs: 0 }) }, token).catch(() => undefined)
    } finally { setBusy('') }
  }, [arrival, form, token])

  useEffect(() => {
    if (!hydrated.current || !arrival || (arrival.status !== 'draft' && arrival.status !== 'needs_changes')) return
    if (JSON.stringify(form) === lastSavedDetails.current) return
    window.clearTimeout(autosaveTimer.current)
    autosaveTimer.current = window.setTimeout(() => { void saveDetails(false) }, 900)
    return () => window.clearTimeout(autosaveTimer.current)
  }, [form, arrival?.status, saveDetails])

  async function saveCompanions() {
    setBusy('companions'); setError('')
    try {
      const response = await api<{ arrival: ArrivalJourney }>('/api/v1/guest/arrival/companions', { method: 'PUT', body: JSON.stringify({ companions }) }, token)
      setArrival(response.arrival); setNotice('اطلاعات همراهان ذخیره شد.')
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function cancel() {
    if (!window.confirm('ادامه چک‌این آنلاین لغو شود؟ پذیرش همچنان می‌تواند ورود حضوری شما را انجام دهد.')) return
    setBusy('cancel')
    try { const response = await api<{ arrival: ArrivalJourney }>('/api/v1/guest/arrival/cancel', { method: 'POST' }, token); setArrival(response.arrival) }
    catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  if (loading) return <section className="arrival-wizard-card"><div className="arrival-loading"><span className="spinner" />در حال بازیابی پیشرفت چک‌این…</div></section>
  if (!arrival || !settings) return <section className="arrival-wizard-card"><div className="inline-alert error" role="alert">{error || 'سفر چک‌این پیدا نشد.'}</div></section>

  const editable = arrival.status === 'draft' || arrival.status === 'needs_changes'
  if (!editable) return <ArrivalStatusCard arrival={arrival} onRefresh={refresh} />

  return <section className="arrival-wizard-card" aria-labelledby="arrival-title">
    <header className="arrival-wizard-header">
      <div><span className="arrival-kicker">چک‌این آنلاین ۲.۰</span><h2 id="arrival-title">آمادگی ورود شما</h2><p>اطلاعات روی سرور هتل ذخیره می‌شود؛ هر زمان می‌توانید ادامه دهید.</p></div>
      <strong>{toFaDigits(arrival.completenessScore)}٪</strong>
    </header>
    {arrival.status === 'needs_changes' && <div className="arrival-rework" role="alert"><i className="ri-edit-2-line" /><div><strong>پذیرش یک اصلاح درخواست کرده است</strong><p>{arrival.needsChangesReason}</p></div></div>}
    <ol className="arrival-progress" aria-label="پیشرفت سه مرحله‌ای">
      {(settings.steps?.length ? settings.steps : [{ id: 'details', title: 'اطلاعات', order: 1 }, { id: 'documents', title: 'مدارک', order: 2 }, { id: 'review', title: 'بازبینی', order: 3 }]).map((item, index) => <li key={item.id} className={step === index + 1 ? 'current' : step > index + 1 ? 'done' : ''} aria-current={step === index + 1 ? 'step' : undefined}><span>{step > index + 1 ? <i className="ri-check-line" /> : toFaDigits(index + 1)}</span><small>{item.title}</small></li>)}
    </ol>
    {error && <div className="arrival-validation" ref={validationRef} tabIndex={-1} role="alert"><i className="ri-error-warning-line" /><span>{error}</span></div>}
    {notice && <div className="inline-alert success" role="status"><i className="ri-checkbox-circle-line" />{notice}</div>}

    {step === 1 && <form className="arrival-step-form" onSubmit={(event) => { event.preventDefault(); void saveDetails(true) }}>
      <fieldset><legend>راه ارتباط و زمان رسیدن</legend>
        <div className="form-row"><label>شماره تماس<input dir="ltr" autoComplete="tel" value={form.contactPhone} onChange={(event) => setForm({ ...form, contactPhone: event.target.value })} required /></label><label>ایمیل (اختیاری)<input dir="ltr" type="email" autoComplete="email" value={form.contactEmail} onChange={(event) => setForm({ ...form, contactEmail: event.target.value })} /></label></div>
        <div className="form-row"><label>زمان تقریبی ورود<input type="datetime-local" value={form.arrivalEta} onChange={(event) => setForm({ ...form, arrivalEta: event.target.value })} required /></label><label>روش رسیدن<select value={form.arrivalMethod} onChange={(event) => setForm({ ...form, arrivalMethod: event.target.value })} required><option value="">انتخاب کنید</option>{Object.entries(methodLabels).map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select></label></div>
        <label>ملیت (فقط در صورت نیاز مقرراتی)<input value={form.nationality} onChange={(event) => setForm({ ...form, nationality: event.target.value })} /></label>
        <label>جزئیات پرواز، قطار یا ترانسفر (اختیاری)<input value={form.transportDetails} onChange={(event) => setForm({ ...form, transportDetails: event.target.value })} /></label>
        <label>نیازهای دسترس‌پذیری (اختیاری)<textarea rows={2} value={form.accessibilityNeeds} onChange={(event) => setForm({ ...form, accessibilityNeeds: event.target.value })} /></label>
        <label>درخواست ویژه (اختیاری)<textarea rows={2} value={form.specialRequests} onChange={(event) => setForm({ ...form, specialRequests: event.target.value })} /></label>
        <small className="autosave-state"><i className={busy === 'autosave' ? 'ri-loader-4-line spin' : 'ri-cloud-line'} />{busy === 'autosave' ? 'در حال ذخیره خودکار…' : 'ذخیره خودکار روی سرور فعال است'}</small>
      </fieldset>
      <CompanionEditor companions={companions} setCompanions={setCompanions} onSave={saveCompanions} busy={busy === 'companions'} />
      <div className="arrival-actions"><button className="primary-button" disabled={busy === 'details'}>{busy === 'details' ? 'در حال ذخیره…' : 'ادامه: مدارک و ثبت دیجیتال'}</button><button type="button" className="link-button danger" onClick={() => void cancel()} disabled={busy === 'cancel'}>لغو چک‌این آنلاین</button></div>
    </form>}
    {step === 2 && <ArrivalEvidenceStep token={token} arrival={arrival} settings={settings} onArrival={setArrival} onBack={() => setStep(1)} onNext={() => setStep(3)} setError={setError} setNotice={setNotice} />}
    {step === 3 && <ArrivalReviewStep token={token} arrival={arrival} settings={settings} onArrival={setArrival} onBack={() => setStep(2)} setError={setError} />}
  </section>
}

function CompanionEditor({ companions, setCompanions, onSave, busy }: {
  companions: { firstName: string; lastName: string; relationship: string; nationality: string; dateOfBirth: string; documentRequired: boolean }[]
  setCompanions: (items: typeof companions) => void; onSave: () => void; busy: boolean
}) {
  return <fieldset className="companion-editor"><legend>همراهان <small>(اختیاری)</small></legend>
    {companions.map((item, index) => <div className="companion-row" key={index}>
      <div className="form-row"><label>نام<input value={item.firstName} onChange={(event) => setCompanions(companions.map((value, itemIndex) => itemIndex === index ? { ...value, firstName: event.target.value } : value))} required /></label><label>نام خانوادگی<input value={item.lastName} onChange={(event) => setCompanions(companions.map((value, itemIndex) => itemIndex === index ? { ...value, lastName: event.target.value } : value))} required /></label></div>
      <div className="form-row"><label>نسبت<input value={item.relationship} onChange={(event) => setCompanions(companions.map((value, itemIndex) => itemIndex === index ? { ...value, relationship: event.target.value } : value))} /></label><label>تاریخ تولد<input type="date" value={item.dateOfBirth} onChange={(event) => setCompanions(companions.map((value, itemIndex) => itemIndex === index ? { ...value, dateOfBirth: event.target.value } : value))} /></label></div>
      <label className="arrival-checkbox"><input type="checkbox" checked={item.documentRequired} onChange={(event) => setCompanions(companions.map((value, itemIndex) => itemIndex === index ? { ...value, documentRequired: event.target.checked } : value))} />مدرک جداگانه این همراه طبق مقررات هتل لازم است</label>
      <button type="button" className="link-button danger" onClick={() => setCompanions(companions.filter((_, itemIndex) => itemIndex !== index))}>حذف همراه</button>
    </div>)}
    <div className="companion-buttons"><button type="button" className="small-button" onClick={() => setCompanions([...companions, { firstName: '', lastName: '', relationship: '', nationality: '', dateOfBirth: '', documentRequired: true }])} disabled={companions.length >= 8}><i className="ri-user-add-line" /> افزودن همراه</button>{companions.length > 0 && <button type="button" className="small-button solid" onClick={onSave} disabled={busy}>{busy ? 'در حال ذخیره…' : 'ذخیره همراهان'}</button>}</div>
  </fieldset>
}

function ArrivalEvidenceStep({ token, arrival, settings, onArrival, onBack, onNext, setError, setNotice }: {
  token: string; arrival: ArrivalJourney; settings: ArrivalSettings; onArrival: (arrival: ArrivalJourney) => void
  onBack: () => void; onNext: () => void; setError: (error: string) => void; setNotice: (notice: string) => void
}) {
  const [file, setFile] = useState<File | null>(null)
  const [target, setTarget] = useState('guest')
  const [evidenceType, setEvidenceType] = useState('identity')
  const [side, setSide] = useState('single')
  const [busy, setBusy] = useState('')
  const [signature, setSignature] = useState<Blob | null>(null)
  const [signerName, setSignerName] = useState(arrival.signerName || `${arrival.stay.guest.firstName} ${arrival.stay.guest.lastName}`)
  const [consent, setConsent] = useState(false)

  async function refresh() {
    const response = await api<{ arrival: ArrivalJourney; settings: ArrivalSettings }>('/api/v1/guest/arrival', {}, token)
    onArrival(response.arrival)
  }

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!file) return
    setBusy('document'); setError(''); setNotice('')
    try {
      const body = new FormData(); body.append('document', file); body.append('evidenceType', evidenceType); body.append('side', side); if (target !== 'guest') body.append('companionId', target)
      await api('/api/v1/guest/arrival/documents', { method: 'POST', body }, token); await refresh(); setFile(null); setNotice('مدرک به‌صورت امن ذخیره شد.')
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function remove(id: string) {
    setBusy(id); setError('')
    try { await api(`/api/v1/guest/arrival/documents/${id}`, { method: 'DELETE' }, token); await refresh() }
    catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function saveSignature() {
    if (!signature || !consent) { setError('امضا و تأیید صریح مقررات الزامی است.'); return }
    setBusy('signature'); setError('')
    try {
      const body = new FormData(); body.append('signature', signature, 'registration-signature.png'); body.append('signerName', signerName); body.append('consent', 'true'); body.append('termsVersion', settings.termsVersion); body.append('locale', settings.termsLocale)
      const response = await api<{ arrival: ArrivalJourney }>('/api/v1/guest/arrival/signature', { method: 'POST', body }, token); onArrival(response.arrival); setNotice('رضایت و امضای نسخه مقررات ذخیره شد.')
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  const requiredTargets = ['guest', ...arrival.companions.filter((item) => item.documentRequired).map((item) => item.id)]
  const documentedTargets = new Set(arrival.documents.map((item) => item.companionId ?? 'guest'))
  const documentsComplete = requiredTargets.every((item) => documentedTargets.has(item))

  return <div className="arrival-step-form arrival-evidence-step">
    <div className="evidence-purpose"><i className="ri-shield-check-line" /><p><strong>هدف دریافت مدرک</strong>{settings.documentPurpose}</p></div>
    <form onSubmit={upload} className="arrival-upload-box">
      <div className="form-row"><label>مدرک متعلق به<select value={target} onChange={(event) => setTarget(event.target.value)}><option value="guest">مهمان اصلی</option>{arrival.companions.map((item) => <option value={item.id} key={item.id}>{item.firstName} {item.lastName}</option>)}</select></label><label>نوع مدرک<select value={evidenceType} onChange={(event) => setEvidenceType(event.target.value)}><option value="identity">مدرک هویتی</option><option value="passport">پاسپورت</option><option value="visa">ویزا</option><option value="other">سایر</option></select></label></div>
      <label>سمت مدرک<select value={side} onChange={(event) => setSide(event.target.value)}><option value="single">تک‌صفحه / کامل</option><option value="front">رو</option><option value="back">پشت</option></select></label>
      <label className="file-picker"><i className="ri-camera-line" /><span>{file?.name || 'تصویر یا PDF را انتخاب کنید'}</span><small>JPG، PNG یا PDF تا ۵ مگابایت؛ فایل ناامن رد می‌شود.</small><input type="file" accept="image/jpeg,image/png,application/pdf" capture="environment" onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></label>
      <button className="small-button solid" disabled={!file || busy === 'document'}>{busy === 'document' ? 'در حال بررسی و ذخیره…' : 'افزودن مدرک'}</button>
    </form>
    <div className="arrival-document-list">{arrival.documents.map((document) => <article key={document.id}><i className={document.mediaType === 'application/pdf' ? 'ri-file-pdf-2-line' : 'ri-image-line'} /><div><strong>{document.name}</strong><small>{document.companionId ? arrival.companions.find((item) => item.id === document.companionId)?.firstName : 'مهمان اصلی'} · {toFaDigits(Math.max(1, Math.round(document.size / 1024)))} کیلوبایت · بررسی دستی امن</small></div><button type="button" onClick={() => void remove(document.id)} disabled={busy === document.id} aria-label={`حذف ${document.name}`}><i className="ri-delete-bin-line" /></button></article>)}</div>
    {!documentsComplete && <div className="inline-alert warning"><i className="ri-information-line" />برای مهمان اصلی و هر همراه الزامی، دست‌کم یک مدرک ثبت کنید.</div>}
    <fieldset className="signature-fieldset"><legend>رضایت و امضای ثبت اقامت</legend><div className="terms-box" tabIndex={0}><strong>نسخه {settings.termsVersion} · {settings.termsLocale}</strong><p>{settings.termsText}</p></div><label>نام کامل امضاکننده<input value={signerName} onChange={(event) => setSignerName(event.target.value)} required /></label><SignaturePad signerName={signerName} onChange={setSignature} /><label className="arrival-checkbox"><input type="checkbox" checked={consent} onChange={(event) => setConsent(event.target.checked)} />متن نسخه بالا را خوانده‌ام و با ثبت امضا، درستی اطلاعات و رضایت خود را تأیید می‌کنم.</label><button type="button" className="small-button solid" onClick={() => void saveSignature()} disabled={!signature || !signerName.trim() || !consent || busy === 'signature'}>{busy === 'signature' ? 'در حال ثبت امن…' : arrival.signaturePresent ? 'جایگزینی رضایت و امضا' : 'ثبت رضایت و امضا'}</button>{arrival.signaturePresent && <span className="signature-saved"><i className="ri-checkbox-circle-fill" /> امضای نسخه {arrival.termsVersion} ثبت شده است</span>}</fieldset>
    <div className="arrival-actions"><button type="button" className="secondary-button" onClick={onBack}>مرحله قبل</button><button type="button" className="primary-button" onClick={onNext} disabled={!documentsComplete || !arrival.signaturePresent}>ادامه: بازبینی نهایی</button></div>
  </div>
}

function SignaturePad({ signerName, onChange }: { signerName: string; onChange: (blob: Blob | null) => void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  function point(event: ReactPointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current; if (!canvas) return { x: 0, y: 0 }
    const rect = canvas.getBoundingClientRect(); return { x: (event.clientX - rect.left) * (canvas.width / rect.width), y: (event.clientY - rect.top) * (canvas.height / rect.height) }
  }
  function start(event: ReactPointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current; if (!canvas) return; drawing.current = true; canvas.setPointerCapture(event.pointerId)
    const context = canvas.getContext('2d'); const p = point(event); context?.beginPath(); context?.moveTo(p.x, p.y)
  }
  function move(event: ReactPointerEvent<HTMLCanvasElement>) {
    if (!drawing.current) return; const canvas = canvasRef.current; const context = canvas?.getContext('2d'); if (!canvas || !context) return
    const p = point(event); context.strokeStyle = '#17245f'; context.lineWidth = 3; context.lineCap = 'round'; context.lineJoin = 'round'; context.lineTo(p.x, p.y); context.stroke()
  }
  function end() { drawing.current = false; canvasRef.current?.toBlob((blob) => onChange(blob), 'image/png') }
  function clear() { const canvas = canvasRef.current; canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height); onChange(null) }
  function createTypedSignature() {
    const canvas = canvasRef.current; const name = signerName.trim(); if (!canvas || !name) return
    const context = canvas.getContext('2d'); if (!context) return
    context.clearRect(0, 0, canvas.width, canvas.height); context.fillStyle = '#17245f'; context.font = '42px sans-serif'; context.textAlign = 'center'; context.textBaseline = 'middle'; context.direction = 'rtl'; context.fillText(name, canvas.width / 2, canvas.height / 2)
    canvas.toBlob((blob) => onChange(blob), 'image/png')
  }
  return <div className="signature-pad"><div><span id="signature-help">با انگشت یا نشانگر امضا کنید؛ برای کار با صفحه‌کلید از امضای متنی استفاده کنید.</span><button type="button" onClick={clear}>پاک کردن</button></div><canvas ref={canvasRef} width={720} height={220} aria-label="صفحه ترسیم امضا" aria-describedby="signature-help" role="img" onPointerDown={start} onPointerMove={move} onPointerUp={end} onPointerCancel={end} /><button type="button" className="small-button" onClick={createTypedSignature} disabled={!signerName.trim()}><i className="ri-keyboard-line" /> ساخت امضای متنی از نام کامل</button></div>
}

function ArrivalReviewStep({ token, arrival, settings, onArrival, onBack, setError }: { token: string; arrival: ArrivalJourney; settings: ArrivalSettings; onArrival: (arrival: ArrivalJourney) => void; onBack: () => void; setError: (error: string) => void }) {
  const [busy, setBusy] = useState(false)
  async function submit() {
    setBusy(true); setError('')
    try {
      const response = await api<{ arrival: ArrivalJourney }>('/api/v1/guest/arrival/submit', { method: 'POST', headers: { 'Idempotency-Key': crypto.randomUUID() } }, token)
      onArrival(response.arrival)
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy(false) }
  }
  return <div className="arrival-step-form arrival-review-step">
    <div className="review-summary"><article><i className="ri-user-line" /><div><strong>اطلاعات ورود</strong><p>{arrival.contactPhone} · {arrival.arrivalEta ? formatDate(arrival.arrivalEta) : '—'} · {methodLabels[arrival.arrivalMethod]}</p></div><button type="button" onClick={onBack}>ویرایش</button></article><article><i className="ri-group-line" /><div><strong>مسافران</strong><p>مهمان اصلی + {toFaDigits(arrival.companions.length)} همراه</p></div></article><article><i className="ri-file-shield-2-line" /><div><strong>مدارک و امضا</strong><p>{toFaDigits(arrival.documents.length)} فایل · امضای مقررات نسخه {settings.termsVersion}</p></div><button type="button" onClick={onBack}>ویرایش</button></article>{arrival.specialRequests && <article><i className="ri-chat-quote-line" /><div><strong>درخواست ویژه</strong><p>{arrival.specialRequests}</p></div></article>}</div>
    {settings.paymentStepEnabled && <div className="inline-alert warning"><i className="ri-bank-card-line" />گام پرداخت به‌صورت کنترل‌شده تعریف شده اما تا اتصال ارائه‌دهنده در M11 هیچ وجهی دریافت نمی‌شود.</div>}
    <div className="review-consent-note"><i className="ri-shield-check-line" /><p>با ارسال، اطلاعات برای بررسی پذیرش قفل می‌شود. اگر اصلاح لازم باشد، دلیل دقیق را دریافت می‌کنید و فقط مراحل لازم را دوباره ارسال خواهید کرد.</p></div>
    <div className="arrival-actions"><button type="button" className="secondary-button" onClick={onBack}>مرحله قبل</button><button type="button" className="primary-button" onClick={() => void submit()} disabled={busy}>{busy ? 'در حال ارسال تکرارناپذیر…' : 'ارسال برای بررسی پذیرش'}</button></div>
  </div>
}

function ArrivalStatusCard({ arrival, onRefresh }: { arrival: ArrivalJourney; onRefresh: () => Promise<void> }) {
  const index = arrival.status === 'submitted' ? 0 : arrival.status === 'approved' || arrival.status === 'arrival_pending' ? 1 : arrival.status === 'room_ready' || arrival.status === 'checked_in' ? 2 : 0
  return <section className={`arrival-status-card ${arrival.status}`}><span className="arrival-status-icon"><i className={arrival.status === 'room_ready' ? 'ri-key-2-line' : arrival.status === 'checked_in' ? 'ri-door-open-line' : 'ri-time-line'} /></span><span className={`status-pill ${arrival.status}`}>{statusLabels[arrival.status]}</span><h2>{arrival.status === 'room_ready' ? 'اتاق شما آماده است' : arrival.status === 'checked_in' ? 'ورود شما ثبت شد' : 'اطلاعات برای پذیرش ارسال شد'}</h2><p>{arrival.status === 'submitted' ? 'پذیرش مدارک و فرم ثبت اقامت را بررسی می‌کند. در صورت نیاز به اصلاح، دلیل دقیق همین‌جا نمایش داده می‌شود.' : arrival.status === 'room_ready' ? 'برای دریافت کلید یا ادامه فرایند دسترسی، به میز پذیرش مراجعه کنید.' : 'وضعیت آمادگی ورود شما به‌روز است.'}</p><ol>{['بررسی اطلاعات', 'تأیید آمادگی ورود', 'آماده شدن اتاق'].map((label, itemIndex) => <li className={itemIndex <= index ? 'done' : ''} key={label}><span>{itemIndex < index ? <i className="ri-check-line" /> : toFaDigits(itemIndex + 1)}</span>{label}</li>)}</ol><button type="button" className="small-button" onClick={() => void onRefresh()}><i className="ri-refresh-line" /> به‌روزرسانی وضعیت</button></section>
}

export function ArrivalsWorkspace({ token, canManageSettings }: { token: string; canManageSettings: boolean }) {
  const [arrivals, setArrivals] = useState<ArrivalJourney[]>([])
  const [analytics, setAnalytics] = useState<ArrivalAnalytics | null>(null)
  const [settings, setSettings] = useState<ArrivalSettings | null>(null)
  const [reservations, setReservations] = useState<Reservation[]>([])
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [invitation, setInvitation] = useState<{ link: string; recoveryCode: string; expiresAt: string; qrDataUrl?: string } | null>(null)
  const [notes, setNotes] = useState<Record<string, string>>({})

  const refresh = useCallback(async () => {
    setError('')
    try {
      const [arrivalResponse, analyticsResponse, settingsResponse, reservationResponse] = await Promise.all([
        api<{ arrivals: ArrivalJourney[] }>(`/api/v1/staff/arrivals${status ? `?status=${status}` : ''}`, {}, token),
        api<{ analytics: ArrivalAnalytics }>('/api/v1/staff/arrivals/analytics', {}, token),
        api<{ settings: ArrivalSettings }>('/api/v1/staff/arrival-settings', {}, token),
        api<{ reservations: Reservation[] }>('/api/v1/staff/reservations?status=confirmed', {}, token),
      ])
      setArrivals(arrivalResponse.arrivals); setAnalytics(analyticsResponse.analytics); setSettings(settingsResponse.settings); setReservations(reservationResponse.reservations)
    } catch (requestError) { setError(messageFrom(requestError)) }
  }, [status, token])
  useEffect(() => { void refresh() }, [refresh])

  async function createInvitation(reservation: Reservation) {
    setBusy(`invite-${reservation.id}`); setError('')
    try {
      const response = await api<{ invitation: { link: string; recoveryCode: string; expiresAt: string; qrDataUrl?: string } }>(`/api/v1/staff/reservations/${reservation.id}/check-in-invitations`, { method: 'POST', body: JSON.stringify({ ttlHours: settings?.invitationTtlHours ?? 168 }) }, token)
      setInvitation(response.invitation)
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function action(item: ArrivalJourney, type: 'assign' | 'approve' | 'needs_changes' | 'arrival_pending' | 'room_ready' | 'remind') {
    setBusy(`${type}-${item.id}`); setError('')
    try {
      if (type === 'assign') await api(`/api/v1/staff/arrivals/${item.id}/assign`, { method: 'POST', body: JSON.stringify({}) }, token)
      else if (type === 'approve' || type === 'needs_changes') await api(`/api/v1/staff/arrivals/${item.id}/review`, { method: 'POST', body: JSON.stringify({ decision: type, reason: notes[item.id] ?? '' }) }, token)
      else if (type === 'remind') await api(`/api/v1/staff/arrivals/${item.id}/remind`, { method: 'POST' }, token)
      else await api(`/api/v1/staff/arrivals/${item.id}/status`, { method: 'POST', body: JSON.stringify({ status: type }) }, token)
      await refresh()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  async function download(item: ArrivalJourney, documentId?: string) {
    setBusy(`download-${documentId ?? item.id}`)
    try {
      const result = await apiBlob(documentId ? `/api/v1/staff/arrivals/${item.id}/documents/${documentId}` : `/api/v1/staff/arrivals/${item.id}/signature`, token)
      const url = URL.createObjectURL(result.blob); const link = document.createElement('a'); link.href = url; link.download = result.filename; link.click(); URL.revokeObjectURL(url)
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  return <div className="arrivals-workspace view-enter">
    <div className="admin-page-heading"><div><p>آمادگی ورود و استثناها</p><h1>میز ورودهای امروز</h1></div><button type="button" className="small-button" onClick={() => void refresh()}><i className="ri-refresh-line" /> به‌روزرسانی</button></div>
    {error && <div className="inline-alert error" role="alert">{error}</div>}
    {analytics && <div className="arrival-analytics-grid"><Metric label="دعوت‌ها" value={analytics.invitations} /><Metric label="نرخ تکمیل" value={`${toFaDigits(Math.round(analytics.completionRate * 100))}٪`} /><Metric label="نیاز به اصلاح" value={analytics.needsChanges} /><Metric label="اتاق آماده" value={analytics.roomReady} /><Metric label="خطای فنی" value={analytics.technicalFailures} /></div>}
    {settings && <ArrivalSettingsPanel token={token} settings={settings} canManage={canManageSettings} onSaved={(value) => setSettings(value)} />}
    <section className="arrival-invitation-panel"><div className="subsection-bar"><div><strong>دعوت امن رزروهای تأییدشده</strong><small>لینک امضاشده، منقضی‌شونده و قابل لغو؛ QR از مسیر بازیابی استفاده می‌کند.</small></div></div><div className="arrival-invite-list">{reservations.map((reservation) => <article key={reservation.id}><div><strong>{reservation.guest.firstName} {reservation.guest.lastName}</strong><small>{formatDate(reservation.arrivalDate)} · اتاق {reservation.room?.number}</small></div><button type="button" className="small-button solid" disabled={!settings?.onlineCheckInEnabled || busy === `invite-${reservation.id}`} onClick={() => void createInvitation(reservation)}>{busy === `invite-${reservation.id}` ? 'در حال ساخت…' : 'ساخت دعوت و QR'}</button></article>)}</div>{invitation && <div className="invitation-result" role="dialog" aria-label="دعوت جدید"><button type="button" className="dialog-close" onClick={() => setInvitation(null)} aria-label="بستن"><i className="ri-close-line" /></button><div><strong>دعوت فقط همین بار نمایش داده می‌شود</strong><small>انقضا: {formatDate(invitation.expiresAt)}</small></div>{invitation.qrDataUrl && <img src={invitation.qrDataUrl} alt="QR بازیابی چک‌این" />}<label>لینک امن<input readOnly dir="ltr" value={`${window.location.origin}${invitation.link}`} /></label><button type="button" className="small-button" onClick={() => void navigator.clipboard.writeText(`${window.location.origin}${invitation.link}`)}>کپی لینک</button><label>کد بازیابی<input readOnly dir="ltr" value={invitation.recoveryCode} /></label></div>}</section>
    <section className="arrival-queue"><div className="arrival-queue-toolbar"><div className="operations-tabs"><button className={!status ? 'active' : ''} onClick={() => setStatus('')}>همه</button>{(['submitted', 'needs_changes', 'approved', 'room_ready'] as const).map((value) => <button key={value} className={status === value ? 'active' : ''} onClick={() => setStatus(value)}>{statusLabels[value]}</button>)}</div><span>{toFaDigits(arrivals.length)} پرونده</span></div>
      <div className="arrival-queue-list">{arrivals.map((item) => <article className={`arrival-queue-card ${item.riskState}`} key={item.id}><header><div className="checkin-person"><span className="avatar">{item.stay.guest.firstName.slice(0, 1)}{item.stay.guest.lastName.slice(0, 1)}</span><div><strong>{item.stay.guest.firstName} {item.stay.guest.lastName}</strong><small>اتاق {item.stay.room.number} · ETA {item.arrivalEta ? formatDate(item.arrivalEta) : 'ثبت نشده'}</small></div></div><span className={`status-pill ${item.status}`}>{statusLabels[item.status]}</span></header><div className="readiness-meter"><span><i style={{ width: `${item.completenessScore}%` }} /></span><strong>{toFaDigits(item.completenessScore)}٪ آماده</strong></div><dl><div><dt>مدارک</dt><dd>{toFaDigits(item.documents.length)} فایل</dd></div><div><dt>امضا</dt><dd>{item.signaturePresent ? 'ثبت شده' : 'ناقص'}</dd></div><div><dt>ریسک</dt><dd>{item.riskState === 'verified' ? 'تأییدشده' : item.riskState === 'needs_changes' ? 'اصلاح' : 'بررسی دستی'}</dd></div><div><dt>مالک</dt><dd>{item.reviewOwnerId ? 'تخصیص یافته' : 'بدون مالک'}</dd></div></dl><div className="arrival-evidence-links">{item.documents.map((document) => <button type="button" key={document.id} onClick={() => void download(item, document.id)}><i className="ri-file-shield-2-line" />{document.name}</button>)}{item.signaturePresent && <button type="button" onClick={() => void download(item)}><i className="ri-pen-nib-line" />امضا</button>}</div>{item.needsChangesReason && <p className="review-note">دلیل اصلاح: {item.needsChangesReason}</p>}<div className="arrival-review-actions">{!item.reviewOwnerId && <button type="button" className="small-button" onClick={() => void action(item, 'assign')}>دریافت پرونده</button>}{item.status === 'submitted' && <><input aria-label="دلیل درخواست اصلاح" placeholder="دلیل دقیق اصلاح" value={notes[item.id] ?? ''} onChange={(event) => setNotes({ ...notes, [item.id]: event.target.value })} /><button type="button" className="small-button approve" onClick={() => void action(item, 'approve')}>تأیید</button><button type="button" className="small-button reject" onClick={() => void action(item, 'needs_changes')}>نیاز به اصلاح</button></>}{item.status === 'approved' && <button type="button" className="small-button solid" onClick={() => void action(item, 'arrival_pending')}>در انتظار ورود</button>}{(item.status === 'approved' || item.status === 'arrival_pending') && <button type="button" className="small-button approve" onClick={() => void action(item, 'room_ready')}>اعلام اتاق آماده</button>}{(item.status === 'draft' || item.status === 'needs_changes') && <button type="button" className="small-button" onClick={() => void action(item, 'remind')}>ثبت یادآوری</button>}</div></article>)}{!arrivals.length && <div className="empty-state">ورودی مطابق این فیلتر وجود ندارد.</div>}</div>
    </section>
  </div>
}

function Metric({ label, value }: { label: string; value: number | string }) { return <article><small>{label}</small><strong>{typeof value === 'number' ? toFaDigits(value) : value}</strong></article> }

function ArrivalSettingsPanel({ token, settings, canManage, onSaved }: { token: string; settings: ArrivalSettings; canManage: boolean; onSaved: (settings: ArrivalSettings) => void }) {
  const [form, setForm] = useState(settings)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  useEffect(() => setForm(settings), [settings])
  async function save() {
    setBusy(true); setNotice(''); setError('')
    const payload = {
      onlineCheckInEnabled: form.onlineCheckInEnabled,
      digitalRegistrationEnabled: form.digitalRegistrationEnabled,
      paymentStepEnabled: form.paymentStepEnabled,
      invitationTtlHours: form.invitationTtlHours,
      termsVersion: form.termsVersion,
      termsLocale: form.termsLocale,
      termsText: form.termsText,
      steps: form.steps,
    }
    try { const response = await api<{ settings: ArrivalSettings }>('/api/v1/staff/arrival-settings', { method: 'PATCH', body: JSON.stringify(payload) }, token); onSaved(response.settings); setNotice('کنترل‌های ورود با ثبت ممیزی ذخیره شد.') }
    catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy(false) }
  }
  return <details className="arrival-settings-panel"><summary><span><i className="ri-settings-3-line" /> کنترل‌های انتشار و ثبت دیجیتال</span><b className={settings.onlineCheckInEnabled ? 'enabled' : ''}>{settings.onlineCheckInEnabled ? 'فعال' : 'خاموش'}</b></summary><div><label className="arrival-checkbox"><input type="checkbox" disabled={!canManage} checked={form.onlineCheckInEnabled} onChange={(event) => setForm({ ...form, onlineCheckInEnabled: event.target.checked })} />چک‌این آنلاین</label><label className="arrival-checkbox"><input type="checkbox" disabled={!canManage} checked={form.digitalRegistrationEnabled} onChange={(event) => setForm({ ...form, digitalRegistrationEnabled: event.target.checked })} />ثبت دیجیتال، مدرک و امضا</label><label className="arrival-checkbox"><input type="checkbox" disabled={!canManage} checked={form.paymentStepEnabled} onChange={(event) => setForm({ ...form, paymentStepEnabled: event.target.checked })} />قرارداد گام پرداخت (بدون دریافت وجه تا M11)</label><div className="form-row"><label>اعتبار دعوت (ساعت)<input type="number" min="1" max="720" disabled={!canManage} value={form.invitationTtlHours} onChange={(event) => setForm({ ...form, invitationTtlHours: Number(event.target.value) })} /></label><label>نسخه مقررات<input dir="ltr" disabled={!canManage} value={form.termsVersion} onChange={(event) => setForm({ ...form, termsVersion: event.target.value })} /></label></div><label>متن مقررات<textarea rows={4} disabled={!canManage} value={form.termsText} onChange={(event) => setForm({ ...form, termsText: event.target.value })} /></label>{error && <div className="inline-alert error" role="alert">{error}</div>}{notice && <div className="inline-alert success" role="status">{notice}</div>}{canManage && <button type="button" className="primary-button compact" onClick={() => void save()} disabled={busy}>{busy ? 'در حال ذخیره…' : 'ذخیره کنترل‌ها'}</button>}</div></details>
}
