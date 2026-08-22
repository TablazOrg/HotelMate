import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Facility, type HotelContent, type MenuItem, type Promotion, type Restaurant, type Service, type ServiceRequest } from './api'
import { formatPrice, handoffTheme, serviceIcon, toFaDigits } from './handoff'

const facilityIcons: Record<string, string> = {
  water: 'ri-water-flash-line', wellness: 'ri-leaf-line', parking: 'ri-parking-box-line',
  restaurant: 'ri-restaurant-line', fitness: 'ri-run-line', wifi: 'ri-wifi-line', concierge: 'ri-service-line',
}

const messageFrom = (error: unknown) => error instanceof Error ? error.message : 'عملیات انجام نشد.'

export function VisitorExperience() {
  const initialSlug = new URLSearchParams(window.location.search).get('hotel') ?? ''
  const [slug, setSlug] = useState(initialSlug)
  const [selectedSlug, setSelectedSlug] = useState(initialSlug)
  const [content, setContent] = useState<HotelContent | null>(null)
  const [loading, setLoading] = useState(Boolean(initialSlug))
  const [error, setError] = useState('')

  useEffect(() => {
    if (!selectedSlug) return
    setLoading(true); setError('')
    api<HotelContent>(`/api/v1/public/hotels/${encodeURIComponent(selectedSlug)}/content`)
      .then(setContent).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
  }, [selectedSlug])

  function findHotel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const normalized = slug.trim().toLowerCase(); if (!normalized) return
    window.history.replaceState({}, '', `/visitor?hotel=${encodeURIComponent(normalized)}`); setSelectedSlug(normalized)
  }

  if (!selectedSlug || (!content && !loading)) return <main className="auth-shell"><section className="auth-mobile visitor-lookup"><div className="auth-brand"><span>H</span><strong>HotelMate</strong></div><div className="auth-intro"><span><i className="ri-hotel-line" /></span><h1>مشاهده امکانات هتل</h1><p>شناسه هتل را وارد کنید تا امکانات، پیشنهادها و منوی رستوران را بدون ورود ببینید.</p></div><form className="auth-form" onSubmit={findHotel}><label>شناسه هتل<input dir="ltr" value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="example-hotel" required /></label>{error && <div className="inline-alert error">{error}</div>}<button className="primary-button">مشاهده هتل</button></form><nav className="auth-links"><a href="/">مهمان هتل هستید؟ وارد شوید <i className="ri-arrow-left-line" /></a></nav></section></main>
  if (loading || !content) return <div className="loading-screen"><span className="spinner" />در حال دریافت اطلاعات هتل…</div>

  return <div className="visitor-app" style={handoffTheme(content.hotel.primaryColor)}><main className="mobile-surface visitor-surface view-enter">
    <section className="visitor-hero"><span>حالت معرفی · بدون ورود</span><div><h1>{content.hotel.name}</h1><p><i className="ri-map-pin-line" /> تجربه اقامت و خدمات هتل</p></div></section>
    <section className="visitor-section"><div className="compact-heading"><h2>امکانات هتل</h2><span>{toFaDigits(content.facilities.length)} امکان فعال</span></div><div className="facility-grid">{content.facilities.map((facility) => <article key={facility.id}><i className={facilityIcons[facility.icon] ?? 'ri-service-line'} /><strong>{facility.name}</strong><small>{facility.hours}</small></article>)}</div></section>
    {content.restaurants.map((restaurant) => <section className="visitor-section" key={restaurant.id}><div className="compact-heading"><h2>{restaurant.name}</h2><span>{restaurant.hours}</span></div><div className="public-menu">{restaurant.menuItems.map((item) => <article key={item.id}><div><strong>{item.name}</strong><small>{item.description}</small></div><b>{formatPrice(item.priceCents, item.currency)} تومان</b></article>)}</div></section>)}
    {content.promotions.map((promotion) => <section className="promotion-card" key={promotion.id}><div><span>پیشنهاد ویژه</span><h2>{promotion.title}</h2><p>{promotion.description}</p></div><b>{promotion.badgeText || `٪${toFaDigits(promotion.discountPct)}`}</b></section>)}
    <a className="primary-button visitor-login" href="/">مهمان هتل هستید؟ وارد شوید</a>
  </main></div>
}

export function PreArrivalOrders({ token }: { token: string }) {
  const [services, setServices] = useState<Service[]>([])
  const [busy, setBusy] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api<{ services: Service[] }>('/api/v1/guest/services', {}, token)
      .then((response) => setServices(response.services.filter((service) => service.isPaid && service.isPreArrival)))
      .catch((requestError) => setError(messageFrom(requestError)))
  }, [token])

  async function order(service: Service) {
    setBusy(service.id); setError(''); setNotice('')
    try {
      await api<{ request: ServiceRequest }>('/api/v1/guest/requests', { method: 'POST', body: JSON.stringify({ serviceId: service.id, quantity: 1, notes: '' }) }, token)
      setNotice(`سفارش «${service.name}» ثبت شد و هنگام ورود آماده خواهد بود.`)
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy('') }
  }

  return <section className="prearrival-orders"><div className="compact-heading"><h2>قبل از رسیدن سفارش دهید</h2><span>آماده در لحظه ورود</span></div>{error && <div className="inline-alert error">{error}</div>}{notice && <div className="inline-alert success">{notice}</div>}<div className="service-rows">{services.map((service) => <article className="service-row" key={service.id}><span className="service-icon"><i className={serviceIcon(service)} /></span><div><strong>{service.name}</strong><small>{formatPrice(service.priceCents, service.currency)} تومان · پرداخت در محل</small></div><button type="button" disabled={busy === service.id} onClick={() => void order(service)}>{busy === service.id ? 'ثبت…' : 'سفارش'}</button></article>)}{!services.length && !error && <div className="empty-state">سفارش پیش از ورود فعالی وجود ندارد.</div>}</div></section>
}

type ContentTab = 'facilities' | 'promotions' | 'restaurants'

export function ContentAdminPanel({ token }: { token: string }) {
  const [content, setContent] = useState<HotelContent | null>(null)
  const [tab, setTab] = useState<ContentTab>('facilities')
  const [showForm, setShowForm] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [facility, setFacility] = useState({ name: '', description: '', icon: 'concierge', hours: '', sortOrder: '10' })
  const [promotion, setPromotion] = useState({ title: '', description: '', discountPct: '0', badgeText: '', startsAt: new Date().toISOString().slice(0, 10), endsAt: new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10) })
  const [restaurant, setRestaurant] = useState({ name: '', description: '', hours: '', sortOrder: '10' })
  const [menuRestaurant, setMenuRestaurant] = useState('')
  const [menu, setMenu] = useState({ name: '', description: '', priceToman: '', sortOrder: '10' })

  const load = useCallback(async () => {
    try { setContent(await api<HotelContent>('/api/v1/staff/content', {}, token)) }
    catch (requestError) { setError(messageFrom(requestError)) }
  }, [token])
  useEffect(() => { void load() }, [load])

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy('create'); setError('')
    try {
      if (tab === 'facilities') await api('/api/v1/staff/facilities', { method: 'POST', body: JSON.stringify({ ...facility, sortOrder: Number(facility.sortOrder), isActive: true }) }, token)
      if (tab === 'promotions') await api('/api/v1/staff/promotions', { method: 'POST', body: JSON.stringify({ ...promotion, discountPct: Number(promotion.discountPct), startsAt: new Date(`${promotion.startsAt}T00:00:00`).toISOString(), endsAt: new Date(`${promotion.endsAt}T23:59:59`).toISOString(), isActive: true }) }, token)
      if (tab === 'restaurants') await api('/api/v1/staff/restaurants', { method: 'POST', body: JSON.stringify({ ...restaurant, sortOrder: Number(restaurant.sortOrder), isActive: true }) }, token)
      setShowForm(false); await load()
    } catch (requestError) { setError(messageFrom(requestError)) }
    finally { setBusy('') }
  }

  async function toggleFacility(item: Facility) {
    setBusy(item.id); try { await api(`/api/v1/staff/facilities/${item.id}`, { method: 'PATCH', body: JSON.stringify({ name: item.name, description: item.description, icon: item.icon, hours: item.hours, sortOrder: item.sortOrder, isActive: !item.isActive }) }, token); await load() } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  async function togglePromotion(item: Promotion) {
    setBusy(item.id); try { await api(`/api/v1/staff/promotions/${item.id}`, { method: 'PATCH', body: JSON.stringify({ title: item.title, description: item.description, discountPct: item.discountPct, badgeText: item.badgeText, startsAt: item.startsAt, endsAt: item.endsAt, isActive: !item.isActive }) }, token); await load() } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  async function toggleRestaurant(item: Restaurant) {
    setBusy(item.id); try { await api(`/api/v1/staff/restaurants/${item.id}`, { method: 'PATCH', body: JSON.stringify({ name: item.name, description: item.description, hours: item.hours, sortOrder: item.sortOrder, isActive: !item.isActive }) }, token); await load() } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  async function addMenuItem(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!menuRestaurant) return; setBusy('menu'); setError('')
    try { await api(`/api/v1/staff/restaurants/${menuRestaurant}/menu-items`, { method: 'POST', body: JSON.stringify({ name: menu.name, description: menu.description, priceCents: Number(menu.priceToman) * 10, currency: 'IRR', sortOrder: Number(menu.sortOrder), isAvailable: true }) }, token); setMenu({ name: '', description: '', priceToman: '', sortOrder: '10' }); await load() } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }
  async function toggleMenu(item: MenuItem) {
    setBusy(item.id); try { await api(`/api/v1/staff/menu-items/${item.id}`, { method: 'PATCH', body: JSON.stringify({ name: item.name, description: item.description, priceCents: item.priceCents, currency: item.currency, sortOrder: item.sortOrder, isAvailable: !item.isAvailable }) }, token); await load() } catch (requestError) { setError(messageFrom(requestError)) } finally { setBusy('') }
  }

  const counts = useMemo(() => ({ facilities: content?.facilities.length ?? 0, promotions: content?.promotions.length ?? 0, restaurants: content?.restaurants.length ?? 0 }), [content])
  return <div className="content-admin view-enter"><div className="admin-page-heading"><div><p>محتوای عمومی و فروش جانبی</p><h1>امکانات و منوی هتل</h1></div><button type="button" className="primary-button compact" onClick={() => setShowForm(!showForm)}><i className={showForm ? 'ri-close-line' : 'ri-add-line'} />{showForm ? 'بستن' : 'مورد جدید'}</button></div><div className="content-tabs">{([['facilities', 'امکانات'], ['promotions', 'پیشنهادها'], ['restaurants', 'رستوران و منو']] as const).map(([value, label]) => <button type="button" className={tab === value ? 'active' : ''} key={value} onClick={() => { setTab(value); setShowForm(false) }}>{label}<span>{toFaDigits(counts[value])}</span></button>)}</div>
    {showForm && <form className="catalog-form" onSubmit={create}>{tab === 'facilities' && <><div className="form-row"><label>نام<input value={facility.name} onChange={(event) => setFacility({ ...facility, name: event.target.value })} required /></label><label>آیکون<select value={facility.icon} onChange={(event) => setFacility({ ...facility, icon: event.target.value })}><option value="concierge">خدمات</option><option value="water">استخر</option><option value="wellness">اسپا</option><option value="parking">پارکینگ</option><option value="restaurant">رستوران</option><option value="fitness">ورزش</option><option value="wifi">وای‌فای</option></select></label></div><label>توضیح<input value={facility.description} onChange={(event) => setFacility({ ...facility, description: event.target.value })} /></label><div className="form-row"><label>ساعات فعالیت<input value={facility.hours} onChange={(event) => setFacility({ ...facility, hours: event.target.value })} /></label><label>ترتیب<input type="number" value={facility.sortOrder} onChange={(event) => setFacility({ ...facility, sortOrder: event.target.value })} /></label></div></>}{tab === 'promotions' && <><label>عنوان<input value={promotion.title} onChange={(event) => setPromotion({ ...promotion, title: event.target.value })} required /></label><label>توضیح<input value={promotion.description} onChange={(event) => setPromotion({ ...promotion, description: event.target.value })} /></label><div className="form-row"><label>درصد تخفیف<input type="number" min="0" max="100" value={promotion.discountPct} onChange={(event) => setPromotion({ ...promotion, discountPct: event.target.value })} /></label><label>متن نشان<input value={promotion.badgeText} onChange={(event) => setPromotion({ ...promotion, badgeText: event.target.value })} placeholder="٪۱۵" /></label></div><div className="form-row"><label>شروع<input type="date" value={promotion.startsAt} onChange={(event) => setPromotion({ ...promotion, startsAt: event.target.value })} required /></label><label>پایان<input type="date" value={promotion.endsAt} onChange={(event) => setPromotion({ ...promotion, endsAt: event.target.value })} required /></label></div></>}{tab === 'restaurants' && <><label>نام رستوران<input value={restaurant.name} onChange={(event) => setRestaurant({ ...restaurant, name: event.target.value })} required /></label><label>توضیح<input value={restaurant.description} onChange={(event) => setRestaurant({ ...restaurant, description: event.target.value })} /></label><div className="form-row"><label>ساعات سرو<input value={restaurant.hours} onChange={(event) => setRestaurant({ ...restaurant, hours: event.target.value })} /></label><label>ترتیب<input type="number" value={restaurant.sortOrder} onChange={(event) => setRestaurant({ ...restaurant, sortOrder: event.target.value })} /></label></div></>}<button className="primary-button" disabled={busy === 'create'}>{busy === 'create' ? 'در حال ثبت…' : 'ثبت مورد'}</button></form>}
    {error && <div className="inline-alert error">{error}</div>}
    {tab === 'facilities' && <div className="content-card-list">{content?.facilities.map((item) => <article key={item.id} className={!item.isActive ? 'inactive' : ''}><span className="service-icon"><i className={facilityIcons[item.icon] ?? 'ri-service-line'} /></span><div><strong>{item.name}</strong><small>{item.description} · {item.hours || 'بدون ساعت'}</small></div><span className={`status-pill ${item.isActive ? 'success' : ''}`}>{item.isActive ? 'فعال' : 'غیرفعال'}</span><button className="small-button" disabled={busy === item.id} onClick={() => void toggleFacility(item)}>{item.isActive ? 'غیرفعال‌سازی' : 'فعال‌سازی'}</button></article>)}</div>}
    {tab === 'promotions' && <div className="content-card-list">{content?.promotions.map((item) => <article key={item.id} className={!item.isActive ? 'inactive' : ''}><span className="service-icon"><i className="ri-percent-line" /></span><div><strong>{item.title}</strong><small>{item.description} · {toFaDigits(item.discountPct)}٪</small></div><span className={`status-pill ${item.isActive ? 'success' : ''}`}>{item.isActive ? 'فعال' : 'غیرفعال'}</span><button className="small-button" disabled={busy === item.id} onClick={() => void togglePromotion(item)}>{item.isActive ? 'غیرفعال‌سازی' : 'فعال‌سازی'}</button></article>)}</div>}
    {tab === 'restaurants' && <><div className="content-card-list restaurant-admin-list">{content?.restaurants.map((item) => <article key={item.id} className={!item.isActive ? 'inactive' : ''}><span className="service-icon"><i className="ri-restaurant-line" /></span><div><strong>{item.name}</strong><small>{item.hours} · {toFaDigits(item.menuItems.length)} آیتم منو</small></div><span className={`status-pill ${item.isActive ? 'success' : ''}`}>{item.isActive ? 'فعال' : 'غیرفعال'}</span><button className="small-button" onClick={() => { setMenuRestaurant(item.id); setShowForm(false) }}>افزودن غذا</button><button className="small-button" disabled={busy === item.id} onClick={() => void toggleRestaurant(item)}>{item.isActive ? 'غیرفعال' : 'فعال'}</button><div className="menu-admin-items">{item.menuItems.map((menuItem) => <button type="button" key={menuItem.id} className={!menuItem.isAvailable ? 'inactive' : ''} onClick={() => void toggleMenu(menuItem)}><span>{menuItem.name}</span><b>{formatPrice(menuItem.priceCents, menuItem.currency)} تومان</b><small>{menuItem.isAvailable ? 'موجود' : 'ناموجود'}</small></button>)}</div></article>)}</div>{menuRestaurant && <form className="catalog-form menu-item-form" onSubmit={addMenuItem}><div className="compact-heading"><h2>آیتم منوی جدید</h2><button type="button" className="small-button" onClick={() => setMenuRestaurant('')}>بستن</button></div><div className="form-row"><label>نام غذا<input value={menu.name} onChange={(event) => setMenu({ ...menu, name: event.target.value })} required /></label><label>قیمت (تومان)<input type="number" min="0" value={menu.priceToman} onChange={(event) => setMenu({ ...menu, priceToman: event.target.value })} required /></label></div><label>توضیح<input value={menu.description} onChange={(event) => setMenu({ ...menu, description: event.target.value })} /></label><button className="primary-button" disabled={busy === 'menu'}>افزودن به منو</button></form>}</>}
  </div>
}
