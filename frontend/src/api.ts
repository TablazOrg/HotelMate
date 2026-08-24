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

export type ArrivalStatus =
  | 'draft' | 'submitted' | 'needs_changes' | 'approved' | 'arrival_pending'
  | 'room_ready' | 'checked_in' | 'expired' | 'cancelled'

export interface ArrivalStep {
  id: 'details' | 'documents' | 'review'
  order: number
  required: boolean
  title: string
}

export interface ArrivalSettings {
  onlineCheckInEnabled: boolean
  digitalRegistrationEnabled: boolean
  paymentStepEnabled: boolean
  invitationTtlHours: number
  termsVersion: string
  termsLocale: string
  termsText: string
  steps: ArrivalStep[]
  documentPurpose: string
}

export interface ArrivalCompanion {
  id: string
  firstName: string
  lastName: string
  relationship: string
  nationality: string
  dateOfBirth: string | null
  documentRequired: boolean
}

export interface ArrivalDocument {
  id: string
  companionId: string | null
  evidenceType: 'identity' | 'passport' | 'visa' | 'other'
  side: 'single' | 'front' | 'back'
  name: string
  mediaType: string
  size: number
  verificationState: string
  verificationNote: string
  retentionUntil: string
  createdAt: string
}

export interface ArrivalJourney {
  id: string
  status: ArrivalStatus
  currentStep: number
  completenessScore: number
  riskState: string
  contactPhone: string
  contactEmail: string
  nationality: string
  arrivalEta: string | null
  arrivalMethod: string
  transportDetails: string
  accessibilityNeeds: string
  specialRequests: string
  answers: Record<string, unknown>
  termsVersion: string
  termsLocale: string
  signerName: string
  consentAt: string | null
  signaturePresent: boolean
  submittedAt: string | null
  reviewedAt: string | null
  reviewedById: string | null
  reviewOwnerId: string | null
  needsChangesReason: string
  approvedAt: string | null
  arrivalPendingAt: string | null
  roomReadyAt: string | null
  checkedInAt: string | null
  cancelledAt: string | null
  expiresAt: string
  reservation: Reservation
  stay: Stay
  documents: ArrivalDocument[]
  companions: ArrivalCompanion[]
  paymentStep?: { required: boolean; status: string; capability: string }
}

export interface ArrivalAnalytics {
  from: string
  to: string
  invitations: number
  opened: number
  started: number
  submitted: number
  approved: number
  needsChanges: number
  roomReady: number
  checkedIn: number
  technicalFailures: number
  abandoned: number
  completionRate: number
  documentReworkRate: number
  medianCompletionMillis: number
  medianReviewMillis: number
  stepEvents: Record<string, number>
}

export type ServiceCategory = 'housekeeping' | 'food_beverage' | 'transport' | 'wellness' | 'other'

export interface Service {
  id: string
  code: string
  name: string
  description: string
  category: ServiceCategory
  icon: string
  fulfillmentRole: 'reception' | 'housekeeping' | 'food_beverage'
  estimatedMinutes: number
  priceCents: number
  currency: string
  isPaid: boolean
  isQuickAction: boolean
  isPreArrival: boolean
  availableFrom: string
  availableUntil: string
  sortOrder: number
  isActive: boolean
}

export interface Facility {
  id: string
  name: string
  description: string
  icon: string
  hours: string
  sortOrder: number
  isActive: boolean
}

export interface Promotion {
  id: string
  title: string
  description: string
  discountPct: number
  badgeText: string
  startsAt: string
  endsAt: string
  isActive: boolean
}

export interface MenuItem {
  id: string
  restaurantId: string
  name: string
  description: string
  priceCents: number
  currency: string
  sortOrder: number
  isAvailable: boolean
}

export interface Restaurant {
  id: string
  name: string
  description: string
  hours: string
  sortOrder: number
  isActive: boolean
  menuItems: MenuItem[]
}

export interface HotelContent {
  hotel: Hotel
  facilities: Facility[]
  promotions: Promotion[]
  restaurants: Restaurant[]
}

export type ServiceRequestStatus = 'new' | 'in_progress' | 'completed' | 'cancelled'

export interface ServiceRequestEvent {
  id: string
  eventType: 'created' | 'assigned' | 'priority_changed' | 'status_changed' | 'note_added'
  fromStatus?: ServiceRequestStatus
  toStatus?: ServiceRequestStatus
  actorType: ActorType
  note: string
  createdAt: string
}

export interface ServiceRequest {
  id: string
  status: ServiceRequestStatus
  priority: number
  quantity: number
  notes: string
  totalPriceCents: number
  service: Service
  assignedTo: Staff | null
  startedAt: string | null
  completedAt: string | null
  cancelledAt: string | null
  createdAt: string
  updatedAt: string
  events: ServiceRequestEvent[]
  stay?: { id: string; guest: Guest; room: Room }
}

export type ConversationStatus = 'ai' | 'handed_off' | 'closed'

export interface ChatMessage {
  id: string
  role: 'guest' | 'ai' | 'staff' | 'system'
  body: string
  senderName: string
  confidence?: number
  redacted: boolean
  createdAt: string
}

export interface Conversation {
  id: string
  status: ConversationStatus
  guest: Guest
  stay?: { id: string; room: Room }
  assignedTo: Staff | null
  guestUnreadCount: number
  staffUnreadCount: number
  lastMessageAt: string
  messages: ChatMessage[]
}

export type KnowledgeStatus = 'draft' | 'pending_review' | 'approved' | 'rejected'

export interface KnowledgeItem {
  id: string
  title: string
  content: string
  source: string
  status: KnowledgeStatus
  version: number
  supersedesId: string | null
  submittedById: string | null
  reviewedById: string | null
  reviewedAt: string | null
  reviewNote: string
  createdAt: string
  updatedAt: string
}

export interface ReportSummary {
  requestsCreated: number
  openRequests: number
  completedRequests: number
  averageFulfillmentMinutes: number
  paidOrders: number
  orderedRevenueCents: number
  recognizedRevenueCents: number
  activeRooms: number
  totalRooms: number
  handedOffConversations: number
  pendingKnowledge: number
  failedSecurityEvents: number
}

export interface OperationalReport {
  from: string
  to: string
  timezone: string
  currency: string
  summary: ReportSummary
  daily: { date: string; requests: number; completed: number; revenueCents: number }[]
  byService: {
    serviceId: string
    serviceName: string
    orders: number
    quantity: number
    orderedRevenueCents: number
    recognizedRevenueCents: number
  }[]
}

export interface AuditLog {
  id: string
  createdAt: string
  requestId: string
  actorId?: string
  actorType: string
  action: string
  outcome: 'success' | 'failure'
  ipAddress: string
  metadata: Record<string, unknown>
}

export interface AuditPage {
  items: AuditLog[]
  total: number
  limit: number
  offset: number
}

export interface RealtimeEvent {
  type: 'connected' | 'request.created' | 'request.updated' | 'message.created' | 'conversation.handoff' | 'conversation.updated'
  payload: { actorType?: ActorType; request?: ServiceRequest; conversation?: Conversation }
  emittedAt: string
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
  requestId: string

  constructor(status: number, code: string, message: string, requestId = '') {
    super(requestId ? `${message} · شناسه رهگیری ${requestId}` : message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
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
    throw new ApiError(response.status, code, message, response.headers.get('X-Request-ID') ?? '')
  }
  return payload as T
}

export async function apiBlob(path: string, token: string): Promise<{ blob: Blob; filename: string }> {
  const response = await fetch(`${apiBaseUrl}${path}`, { headers: { Authorization: `Bearer ${token}` } })
  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    throw new ApiError(response.status, payload?.error?.code ?? 'request_failed', payload?.error?.message ?? 'دریافت فایل انجام نشد', response.headers.get('X-Request-ID') ?? '')
  }
  const disposition = response.headers.get('Content-Disposition') ?? ''
  const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? 'identity-document'
  return { blob: await response.blob(), filename }
}

export function subscribeRealtime(
  token: string,
  onEvent: (event: RealtimeEvent) => void,
  onConnectionChange?: (connected: boolean) => void,
) {
  let closed = false
  let socket: WebSocket | null = null
  let reconnectTimer = 0
  let attempts = 0

  const connect = () => {
    if (closed) return
    const endpoint = new URL(`${apiBaseUrl}/api/v1/events`, window.location.origin)
    endpoint.protocol = endpoint.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(endpoint, ['hotelmate.events', token])
    socket.onopen = () => { attempts = 0; onConnectionChange?.(true) }
    socket.onmessage = (message) => {
      try { onEvent(JSON.parse(message.data) as RealtimeEvent) } catch { /* ignore invalid server frames */ }
    }
    socket.onerror = () => socket?.close()
    socket.onclose = () => {
      onConnectionChange?.(false)
      if (!closed) {
        const delay = Math.min(1000 * 2 ** attempts, 15_000)
        attempts += 1
        reconnectTimer = window.setTimeout(connect, delay)
      }
    }
  }

  connect()
  return () => {
    closed = true
    window.clearTimeout(reconnectTimer)
    socket?.close(1000, 'component unmounted')
  }
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
