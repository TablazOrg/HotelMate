import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type AuditPage, type OperationalReport } from './api'
import { formatPrice, toFaDigits } from './handoff'

function dateInput(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function initialRange(days: number) {
  const to = new Date()
  const from = new Date()
  from.setDate(from.getDate() - (days - 1))
  return { from: dateInput(from), to: dateInput(to) }
}

function messageFrom(error: unknown) {
  return error instanceof Error ? error.message : 'دریافت گزارش انجام نشد.'
}

function formatPersianDate(value: string) {
  return new Intl.DateTimeFormat('fa-IR', { month: 'short', day: 'numeric' }).format(new Date(`${value}T12:00:00`))
}

export function ReportingPanel({ token }: { token: string }) {
  const [range, setRange] = useState(() => initialRange(7))
  const [report, setReport] = useState<OperationalReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try {
      const query = new URLSearchParams(range)
      const response = await api<{ report: OperationalReport }>(`/api/v1/staff/reports/operations?${query}`, {}, token)
      setReport(response.report)
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setLoading(false) }
  }, [range, token])

  useEffect(() => { void load() }, [load])
  const maxDaily = useMemo(() => Math.max(1, ...(report?.daily.map((item) => Math.max(item.requests, item.completed)) ?? [1])), [report])
  const summary = report?.summary
  const stats = [
    ['درخواست ثبت‌شده', summary?.requestsCreated ?? 0, 'در بازه انتخابی'],
    ['تکمیل‌شده', summary?.completedRequests ?? 0, summary?.averageFulfillmentMinutes ? `میانگین ${toFaDigits(summary.averageFulfillmentMinutes)} دقیقه` : 'بدون زمان تکمیل'],
    ['سفارش پولی', summary?.paidOrders ?? 0, `${formatPrice(summary?.orderedRevenueCents ?? 0)} تومان سفارش`],
    ['درآمد تحقق‌یافته', `${formatPrice(summary?.recognizedRevenueCents ?? 0)} تومان`, 'درخواست‌های تکمیل‌شده'],
    ['اتاق فعال', `${toFaDigits(summary?.activeRooms ?? 0)} / ${toFaDigits(summary?.totalRooms ?? 0)}`, 'وضعیت جاری هتل'],
    ['تحویل به پذیرش', summary?.handedOffConversations ?? 0, `${toFaDigits(summary?.pendingKnowledge ?? 0)} دانش در انتظار`],
  ] as const

  function chooseDays(days: number) { setRange(initialRange(days)) }

  return <div className="reporting-page view-enter">
    <div className="admin-page-heading"><div><p>شاخص‌های محاسبه‌شده در سرور · {report?.timezone ?? 'منطقه زمانی هتل'}</p><h1>گزارش عملیات و درآمد</h1></div><div className="range-presets"><button type="button" onClick={() => chooseDays(7)}>۷ روز</button><button type="button" onClick={() => chooseDays(30)}>۳۰ روز</button></div></div>
    <div className="report-range"><label>از<input type="date" value={range.from} onChange={(event) => setRange({ ...range, from: event.target.value })} /></label><label>تا<input type="date" value={range.to} onChange={(event) => setRange({ ...range, to: event.target.value })} /></label><button type="button" className="primary-button compact" onClick={() => void load()}>به‌روزرسانی</button></div>
    {error && <div className="inline-alert error">{error}</div>}
    <div className="stats-grid">{stats.map(([label, value, hint]) => <article key={label}><span>{label}</span><strong>{loading ? '—' : toFaDigits(value)}</strong><small>{hint}</small></article>)}</div>
    <div className="report-grid">
      <section className="report-card"><div className="compact-heading"><h2>روند روزانه</h2><span>درخواست / تکمیل</span></div><div className="daily-chart">{report?.daily.map((item) => <div className="daily-column" key={item.date}><div><i className="requests" style={{ height: `${Math.max(5, item.requests / maxDaily * 100)}%` }} /><i className="completed" style={{ height: `${Math.max(5, item.completed / maxDaily * 100)}%` }} /></div><strong>{toFaDigits(item.requests)} / {toFaDigits(item.completed)}</strong><small>{formatPersianDate(item.date)}</small></div>)}{!loading && !report?.daily.length && <div className="empty-state">داده‌ای در این بازه نیست.</div>}</div></section>
      <section className="report-card"><div className="compact-heading"><h2>درآمد بر پایه سرویس</h2><span>مبالغ به تومان</span></div><div className="service-report-table"><div className="table-head"><span>سرویس</span><span>سفارش</span><span>تحقق‌یافته</span></div>{report?.byService.map((item) => <div className="table-row" key={item.serviceId}><span>{item.serviceName}</span><span>{toFaDigits(item.orders)}</span><strong>{formatPrice(item.recognizedRevenueCents)}</strong></div>)}{!loading && !report?.byService.length && <div className="table-empty">سفارش درآمدی ثبت نشده است.</div>}</div></section>
    </div>
  </div>
}

const actionLabels: Record<string, string> = {
  'auth.staff.login': 'ورود پرسنل', 'auth.guest.login': 'ورود مهمان', 'hotel.onboard': 'راه‌اندازی هتل',
  'service_request.create': 'ثبت درخواست', 'service_request.transition': 'تغییر وضعیت درخواست',
  'conversation.message': 'پیام مهمان', 'conversation.reply': 'پاسخ پذیرش', 'knowledge.review': 'بازبینی دانش',
}

export function AuditPanel({ token }: { token: string }) {
  const [range, setRange] = useState(() => initialRange(7))
  const [outcome, setOutcome] = useState('')
  const [action, setAction] = useState('')
  const [offset, setOffset] = useState(0)
  const [page, setPage] = useState<AuditPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const limit = 25

  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    const query = new URLSearchParams({ ...range, limit: String(limit), offset: String(offset) })
    if (outcome) query.set('outcome', outcome)
    if (action.trim()) query.set('action', action.trim())
    api<{ audit: AuditPage }>(`/api/v1/staff/audit-logs?${query}`, {}, token)
      .then((response) => { if (active) setPage(response.audit) })
      .catch((requestError) => { if (active) setError(messageFrom(requestError)) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [action, offset, outcome, range, token])

  return <div className="audit-page view-enter">
    <div className="admin-page-heading"><div><p>ردیابی تغییرات و رخدادهای حساس</p><h1>امنیت و ممیزی</h1></div><span className="security-count">{toFaDigits(page?.total ?? 0)} رویداد</span></div>
    <div className="audit-filters"><label>از<input type="date" value={range.from} onChange={(event) => { setOffset(0); setRange({ ...range, from: event.target.value }) }} /></label><label>تا<input type="date" value={range.to} onChange={(event) => { setOffset(0); setRange({ ...range, to: event.target.value }) }} /></label><label>نتیجه<select value={outcome} onChange={(event) => { setOffset(0); setOutcome(event.target.value) }}><option value="">همه</option><option value="success">موفق</option><option value="failure">ناموفق</option></select></label><label>کد رویداد<input dir="ltr" value={action} onChange={(event) => { setOffset(0); setAction(event.target.value) }} placeholder="staff.login" /></label></div>
    {error && <div className="inline-alert error">{error}</div>}
    <section className="audit-card"><div className="audit-table"><div className="table-head"><span>رویداد</span><span>عامل</span><span>زمان / نشانی</span><span>شناسه رهگیری</span></div>{page?.items.map((item) => <div className="table-row" key={item.id}><span><b className={`audit-outcome ${item.outcome}`}>{item.outcome === 'success' ? 'موفق' : 'ناموفق'}</b><strong>{actionLabels[item.action] ?? item.action}</strong></span><span>{item.actorType}</span><span>{new Intl.DateTimeFormat('fa-IR', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(item.createdAt))}<small>{item.ipAddress || '—'}</small></span><code title={item.requestId}>{item.requestId || '—'}</code></div>)}{!loading && !page?.items.length && <div className="table-empty">رویدادی با این فیلتر پیدا نشد.</div>}{loading && <div className="center-loading"><span className="spinner" /> در حال دریافت رویدادها…</div>}</div><footer><button type="button" disabled={!offset} onClick={() => setOffset(Math.max(0, offset - limit))}>صفحه قبل</button><span>{toFaDigits(offset + 1)} تا {toFaDigits(Math.min(offset + limit, page?.total ?? 0))}</span><button type="button" disabled={offset + limit >= (page?.total ?? 0)} onClick={() => setOffset(offset + limit)}>صفحه بعد</button></footer></section>
  </div>
}
