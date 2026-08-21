const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '')

export type ActorType = 'staff' | 'guest'

export interface Hotel {
  id: string
  name: string
  slug: string
  logoUrl: string
  primaryColor: string
  timezone: string
}

export interface Staff {
  id: string
  firstName: string
  lastName: string
  email: string
  role: StaffRole
  isActive: boolean
  lastLoginAt: string | null
}

export type StaffRole =
  | 'primary_admin'
  | 'secondary_admin'
  | 'operations_manager'
  | 'reception'
  | 'housekeeping'
  | 'food_beverage'

export interface Stay {
  id: string
  status: 'active' | 'pre_arrival' | 'checked_out'
  guest: { id: string; firstName: string; lastName: string }
  room: { id: string; number: string; floor: number; type: string }
  hotel: Hotel
  checkInAt: string | null
  checkOutAt: string | null
}

export interface SessionRecord {
  actorType: ActorType
  token: string
  expiresAt: string
}

export interface StaffLoginResponse extends SessionRecord {
  actorType: 'staff'
  staff: Staff
  hotel: Hotel
}

export interface GuestLoginResponse extends SessionRecord {
  actorType: 'guest'
  stay: Stay
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export async function api<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const response = await fetch(`${apiBaseUrl}${path}`, { ...init, headers })
  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    const code = payload?.error?.code ?? 'request_failed'
    const message = payload?.error?.message ?? 'ارتباط با سرور انجام نشد'
    throw new ApiError(response.status, code, message)
  }
  return payload as T
}

const sessionKey = 'hotelmate.session'

export function loadSession(): SessionRecord | null {
  try {
    const raw = sessionStorage.getItem(sessionKey)
    if (!raw) return null
    const session = JSON.parse(raw) as SessionRecord
    if (!session.token || !session.actorType || new Date(session.expiresAt).getTime() <= Date.now()) {
      sessionStorage.removeItem(sessionKey)
      return null
    }
    return session
  } catch {
    sessionStorage.removeItem(sessionKey)
    return null
  }
}

export function saveSession(session: SessionRecord) {
  sessionStorage.setItem(sessionKey, JSON.stringify(session))
}

export function clearSession() {
  sessionStorage.removeItem(sessionKey)
}
