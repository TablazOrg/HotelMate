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

export type RoomStatus = 'available' | 'occupied' | 'cleaning' | 'out_of_service'

export interface Room {
  id: string
  number: string
  floor: number
  type: string
  status: RoomStatus
}

export interface Guest {
  id: string
  firstName: string
  lastName: string
}

export type ReservationStatus = 'pending' | 'confirmed' | 'cancelled' | 'completed'

export interface StaySummary {
  id: string
  status: Stay['status']
  room: Room
  checkInAt: string | null
  checkOutAt: string | null
}

export interface Reservation {
  id: string
  status: ReservationStatus
  confirmationCode: string
  arrivalDate: string
  departureDate: string
  confirmedAt: string | null
  guest: Guest
  room: Room | null
  stay?: StaySummary
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
  guest: Guest
  room: Room
  hotel: Hotel
  checkInAt: string | null
  checkOutAt: string | null
  reservation?: Reservation
}

export type OnlineCheckInStatus = 'submitted' | 'approved' | 'rejected'

export interface OnlineCheckIn {
  id: string
  status: OnlineCheckInStatus
  documentName: string
  documentMediaType: string
  documentSize: number
  submittedAt: string
  reviewedAt: string | null
  reviewedById: string | null
  reviewNote: string
  retentionUntil: string
  documentAvailable: boolean
  stay?: Stay
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
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
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

export async function apiBlob(path: string, token: string): Promise<{ blob: Blob; filename: string }> {
  const response = await fetch(`${apiBaseUrl}${path}`, { headers: { Authorization: `Bearer ${token}` } })
  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    throw new ApiError(response.status, payload?.error?.code ?? 'request_failed', payload?.error?.message ?? 'دریافت فایل انجام نشد')
  }
  const disposition = response.headers.get('Content-Disposition') ?? ''
  const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? 'identity-document'
  return { blob: await response.blob(), filename }
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
