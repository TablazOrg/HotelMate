import { useEffect, useState } from 'react'

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '')

type ApiState = 'checking' | 'online' | 'offline'

function App() {
  const [apiState, setApiState] = useState<ApiState>('checking')

  useEffect(() => {
    fetch(`${apiBaseUrl}/healthz`)
      .then((response) => {
        if (!response.ok) throw new Error('API unavailable')
        setApiState('online')
      })
      .catch(() => setApiState('offline'))
  }, [])

  return (
    <main className="page-shell">
      <section className="hero-card" aria-labelledby="app-title">
        <div className="brand-mark" aria-hidden="true">HM</div>
        <div>
          <p className="eyebrow">HotelMate · زیرساخت آماده توسعه</p>
          <h1 id="app-title">همراه هوشمند هتل</h1>
          <p className="intro">
            تجربه مهمان و عملیات هتل را در یک فضای یکپارچه مدیریت کنید.
          </p>
        </div>
        <div className={`status-pill ${apiState}`} role="status">
          <span className="status-dot" />
          {apiState === 'checking' && 'در حال بررسی API'}
          {apiState === 'online' && 'API متصل است'}
          {apiState === 'offline' && 'API در دسترس نیست'}
        </div>
      </section>

      <section className="next-steps" aria-labelledby="next-steps-title">
        <h2 id="next-steps-title">ماژول‌های اصلی</h2>
        <div className="module-grid">
          <article><span>01</span><h3>ورود مهمان</h3><p>احراز هویت امن با اتاق و مدرک شناسایی</p></article>
          <article><span>02</span><h3>درخواست خدمات</h3><p>ثبت، اولویت‌بندی و پیگیری لحظه‌ای درخواست‌ها</p></article>
          <article><span>03</span><h3>پنل عملیات</h3><p>نمایش کارهای پذیرش، خانه‌داری و F&amp;B</p></article>
        </div>
      </section>
    </main>
  )
}

export default App
