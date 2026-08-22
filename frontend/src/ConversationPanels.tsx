import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, subscribeRealtime, type ChatMessage, type Conversation, type KnowledgeItem } from './api'
import { formatTime, toFaDigits } from './handoff'

const messageFrom = (error: unknown) => error instanceof Error ? error.message : 'ارتباط با سرور انجام نشد.'

function mergeConversation(items: Conversation[], incoming: Conversation) {
  return [incoming, ...items.filter((item) => item.id !== incoming.id)].sort((a, b) => new Date(b.lastMessageAt).getTime() - new Date(a.lastMessageAt).getTime())
}

export function GuestChat({ token }: { token: string }) {
  const [conversation, setConversation] = useState<Conversation | null>(null)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const endRef = useRef<HTMLDivElement | null>(null)

  const markRead = useCallback(() => api<{ conversation: Conversation }>('/api/v1/guest/conversation/read', { method: 'POST' }, token).catch(() => null), [token])
  useEffect(() => {
    api<{ conversation: Conversation }>('/api/v1/guest/conversation', {}, token)
      .then((response) => setConversation(response.conversation)).catch((requestError) => setError(messageFrom(requestError)))
    return subscribeRealtime(token, (event) => {
      if (!event.payload.conversation) return
      setConversation(event.payload.conversation)
      void markRead()
    })
  }, [markRead, token])
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }) }, [conversation?.messages.length, sending])

  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const body = draft.trim()
    if (!body || sending) return
    setDraft(''); setSending(true); setError('')
    try {
      const response = await api<{ conversation: Conversation }>('/api/v1/guest/conversation/messages', { method: 'POST', body: JSON.stringify({ body }) }, token)
      setConversation(response.conversation)
    } catch (requestError) { setDraft(body); setError(messageFrom(requestError)) }
    finally { setSending(false) }
  }

  const handedOff = conversation?.status === 'handed_off'
  const closed = conversation?.status === 'closed'
  const staffName = conversation?.assignedTo?.firstName ?? 'پذیرش'
  return <div className="view-enter screen-view chat-screen">
    <header className={`chat-head ${handedOff ? 'staff-mode' : ''}`}><span><i className={handedOff ? 'ri-user-3-line' : 'ri-sparkling-2-line'} /></span><div><h1>{handedOff ? 'پذیرش هتل' : 'دستیار هوشمند'}</h1><p>{handedOff ? `${staffName} · آنلاین` : 'پاسخ فوری · ۲۴ ساعته'}</p></div></header>
    {error && <div className="inline-alert error">{error}</div>}
    <div className="chat-thread" aria-live="polite">
      {conversation?.messages.map((message) => <GuestBubble message={message} key={message.id} />)}
      {handedOff && <div className="handoff-pill"><i className="ri-user-shared-line" /> گفتگو به پذیرش منتقل شد — {staffName} از پذیرش پاسخ می‌دهد</div>}
      {sending && <div className="chat-bubble ai typing"><i /><i /><i /><span>در حال نوشتن…</span></div>}
      {!conversation && !error && <div className="center-loading"><span className="spinner" /> در حال آماده‌سازی گفتگو…</div>}
      <div ref={endRef} />
    </div>
    <form className="chat-composer" onSubmit={send}><input value={draft} onChange={(event) => setDraft(event.target.value)} maxLength={1000} disabled={closed} placeholder={closed ? 'این گفتگو بسته شده است' : 'مثلاً: استخر تا چه ساعتی باز است؟'} /><button type="submit" disabled={!draft.trim() || sending || closed} aria-label="ارسال"><i className="ri-send-plane-2-line" /></button></form>
  </div>
}

function GuestBubble({ message }: { message: ChatMessage }) {
  if (message.role === 'system') return <div className="chat-system-message">{message.body}</div>
  return <div className={`chat-message ${message.role}`}><div className={`chat-bubble ${message.role}`}>{message.body}{message.redacted && <small className="redacted-note">اطلاعات شخصی محافظت شد</small>}</div><small>{message.senderName} · {formatTime(message.createdAt)}</small></div>
}

type InboxFilter = 'all' | 'handed_off' | 'ai' | 'closed'

export function StaffConversationInbox({ token }: { token: string }) {
  const [items, setItems] = useState<Conversation[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [filter, setFilter] = useState<InboxFilter>('all')
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const selected = items.find((item) => item.id === selectedID) ?? null

  const load = useCallback(async () => {
    try {
      const response = await api<{ conversations: Conversation[] }>('/api/v1/staff/conversations', {}, token)
      setItems(response.conversations)
      setSelectedID((current) => current || response.conversations[0]?.id || '')
    } catch (requestError) { setError(messageFrom(requestError)) }
  }, [token])
  useEffect(() => { void load(); return subscribeRealtime(token, (event) => { if (event.payload.conversation) setItems((current) => mergeConversation(current, event.payload.conversation!)) }, setConnected) }, [load, token])

  async function select(item: Conversation) {
    setSelectedID(item.id)
    try {
      const response = await api<{ conversation: Conversation }>(`/api/v1/staff/conversations/${item.id}/read`, { method: 'POST' }, token)
      setItems((current) => mergeConversation(current, response.conversation))
    } catch { /* list data remains usable */ }
  }
  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected || !draft.trim()) return
    setBusy(true); setError('')
    try {
      const response = await api<{ conversation: Conversation }>(`/api/v1/staff/conversations/${selected.id}/messages`, { method: 'POST', body: JSON.stringify({ body: draft.trim() }) }, token)
      setItems((current) => mergeConversation(current, response.conversation)); setDraft('')
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy(false) }
  }
  async function closeConversation() {
    if (!selected) return
    setBusy(true)
    try {
      const response = await api<{ conversation: Conversation }>(`/api/v1/staff/conversations/${selected.id}/close`, { method: 'POST' }, token)
      setItems((current) => mergeConversation(current, response.conversation))
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy(false) }
  }

  const visible = items.filter((item) => filter === 'all' || item.status === filter)
  const handedOffCount = items.filter((item) => item.status === 'handed_off').length
  return <div className="conversation-admin view-enter"><div className="admin-page-heading"><div><p>پاسخ‌گویی زنده و انتقال از دستیار</p><h1>گفتگوهای پذیرش</h1></div><span className={`connection-pill ${connected ? 'connected' : ''}`}><i />{connected ? 'متصل' : 'در حال اتصال'}</span></div>
    <div className="conversation-filters">{([['all', 'همه'], ['handed_off', `نیازمند پذیرش · ${toFaDigits(handedOffCount)}`], ['ai', 'دستیار'], ['closed', 'بسته']] as const).map(([value, label]) => <button type="button" className={filter === value ? 'active' : ''} key={value} onClick={() => setFilter(value)}>{label}</button>)}</div>
    {error && <div className="inline-alert error">{error}</div>}
    <div className="conversation-workspace"><aside className="conversation-list">{visible.map((item) => { const last = item.messages[item.messages.length - 1]; return <button type="button" className={selectedID === item.id ? 'active' : ''} key={item.id} onClick={() => void select(item)}><span className="conversation-avatar">{item.guest.firstName.slice(0, 1)}{item.guest.lastName.slice(0, 1)}</span><div><strong>{item.guest.firstName} {item.guest.lastName}</strong><small>اتاق {toFaDigits(item.stay?.room.number ?? '—')} · {last?.body ?? 'بدون پیام'}</small></div><time>{formatTime(item.lastMessageAt)}</time>{item.staffUnreadCount > 0 && <b>{toFaDigits(item.staffUnreadCount)}</b>}</button>})}{!visible.length && <div className="guest-empty"><i className="ri-chat-off-line" /><p>گفتگویی در این وضعیت نیست</p></div>}</aside>
      <section className="staff-chat-pane">{selected ? <><header><div><strong>{selected.guest.firstName} {selected.guest.lastName}</strong><small>اتاق {toFaDigits(selected.stay?.room.number ?? '—')}</small></div><span className={`status-pill ${selected.status === 'handed_off' ? 'warning' : selected.status === 'closed' ? '' : 'success'}`}>{selected.status === 'handed_off' ? 'منتقل‌شده به پذیرش' : selected.status === 'closed' ? 'بسته' : 'پاسخ‌گویی دستیار'}</span>{selected.status !== 'closed' && <button type="button" className="small-button" disabled={busy} onClick={() => void closeConversation()}>بستن گفتگو</button>}</header><div className="staff-chat-thread">{selected.messages.map((message) => <GuestBubble message={message} key={message.id} />)}</div><form className="staff-chat-composer" onSubmit={send}><input value={draft} onChange={(event) => setDraft(event.target.value)} maxLength={1000} disabled={selected.status !== 'handed_off'} placeholder={selected.status === 'handed_off' ? 'پاسخ پذیرش را بنویسید…' : 'فقط گفتگوهای منتقل‌شده قابل پاسخ هستند'} /><button disabled={busy || selected.status !== 'handed_off' || !draft.trim()}><i className="ri-send-plane-2-line" /> ارسال پاسخ</button></form></> : <div className="guest-empty"><i className="ri-chat-3-line" /><p>یک گفتگو را انتخاب کنید</p></div>}</section>
    </div>
  </div>
}

export function KnowledgeAdminPanel({ token, canReview }: { token: string; canReview: boolean }) {
  const [items, setItems] = useState<KnowledgeItem[]>([])
  const [showForm, setShowForm] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [rejecting, setRejecting] = useState('')
  const [rejectNote, setRejectNote] = useState('')
  const [form, setForm] = useState({ title: '', content: '', source: 'پیشنهاد پذیرش', supersedesId: '' })
  const load = useCallback(() => api<{ knowledge: KnowledgeItem[] }>('/api/v1/staff/knowledge', {}, token).then((response) => setItems(response.knowledge)).catch((requestError) => setError(messageFrom(requestError))), [token])
  useEffect(() => { void load() }, [load])
  const pending = useMemo(() => items.filter((item) => item.status === 'pending_review').length, [items])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy('create'); setError('')
    try {
      await api('/api/v1/staff/knowledge', { method: 'POST', body: JSON.stringify({ ...form, supersedesId: form.supersedesId || null }) }, token)
      setForm({ title: '', content: '', source: 'پیشنهاد پذیرش', supersedesId: '' }); setShowForm(false); await load()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  async function review(item: KnowledgeItem, status: 'approved' | 'rejected', note = '') {
    setBusy(item.id); setError('')
    try {
      await api(`/api/v1/staff/knowledge/${item.id}/review`, { method: 'POST', body: JSON.stringify({ status, note }) }, token)
      setRejecting(''); setRejectNote(''); await load()
    } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  function version(item: KnowledgeItem) {
    setForm({ title: item.title, content: item.content, source: 'بازنگری دانش‌نامه', supersedesId: item.id }); setShowForm(true); window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return <div className="knowledge-page view-enter"><div className="admin-page-heading"><div><p>فقط پاسخ‌های تأییدشده به مهمان نمایش داده می‌شوند</p><h1>دانش‌نامه هوش مصنوعی</h1></div><div className="heading-actions"><span className="pending-pill">{toFaDigits(pending)} در انتظار بررسی</span><button type="button" className="primary-button compact" onClick={() => setShowForm(!showForm)}><i className="ri-add-line" /> پاسخ جدید</button></div></div>
    {showForm && <form className="knowledge-form" onSubmit={submit}><div className="compact-heading"><h2>{form.supersedesId ? 'نسخه جدید پاسخ' : 'پیشنهاد پاسخ جدید'}</h2><button type="button" className="small-button" onClick={() => setShowForm(false)}>بستن</button></div><label>سؤال مهمان<input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} maxLength={240} required /></label><label>پاسخ تأییدپذیر<textarea value={form.content} onChange={(event) => setForm({ ...form, content: event.target.value })} maxLength={2000} required /></label><label>منبع<input value={form.source} onChange={(event) => setForm({ ...form, source: event.target.value })} maxLength={120} /></label><button className="primary-button" disabled={busy === 'create'}>{busy === 'create' ? 'در حال ثبت…' : 'ارسال برای بررسی'}</button></form>}
    {error && <div className="inline-alert error">{error}</div>}
    <div className="knowledge-list">{items.map((item) => <article key={item.id} className={item.status}><header><div><strong>{item.title}</strong><small>نسخه {toFaDigits(item.version)} · {item.source} · {new Date(item.createdAt).toLocaleDateString('fa-IR')}</small></div><KnowledgeStatus item={item} /></header><p>{item.content}</p>{item.reviewNote && <blockquote>{item.reviewNote}</blockquote>}<footer>{item.status === 'pending_review' && canReview ? <>{rejecting === item.id ? <div className="reject-controls"><input value={rejectNote} onChange={(event) => setRejectNote(event.target.value)} placeholder="دلیل رد محتوا" autoFocus /><button type="button" disabled={!rejectNote.trim() || busy === item.id} onClick={() => void review(item, 'rejected', rejectNote.trim())}>تأیید رد</button><button type="button" onClick={() => setRejecting('')}>انصراف</button></div> : <><button type="button" className="approve-button" disabled={busy === item.id} onClick={() => void review(item, 'approved')}><i className="ri-check-line" /> تأیید</button><button type="button" className="reject-button" onClick={() => setRejecting(item.id)}><i className="ri-close-line" /> رد</button></>}</> : item.status === 'approved' ? <button type="button" className="small-button" onClick={() => version(item)}><i className="ri-file-copy-line" /> نسخه جدید</button> : null}</footer></article>)}</div>
  </div>
}

function KnowledgeStatus({ item }: { item: KnowledgeItem }) {
  const meta = item.status === 'pending_review' ? ['در انتظار بررسی', 'warning'] : item.status === 'approved' ? ['تأیید شد', 'success'] : item.status === 'rejected' ? ['رد شد', 'error'] : ['پیش‌نویس', '']
  return <span className={`status-pill ${meta[1]}`}>{meta[0]}</span>
}
