import type { CSSProperties } from 'react'
import type { Service, ServiceRequest, ServiceRequestStatus } from './api'

const brandStops: Record<string, { dark: string; soft: string }> = {
  '#6c4fe0': { dark: '#4a32b0', soft: 'rgba(108,79,224,.08)' },
  '#17245f': { dark: '#0d1640', soft: 'rgba(23,36,95,.08)' },
  '#575eff': { dark: '#242bcc', soft: 'rgba(87,94,255,.08)' },
  '#13b476': { dark: '#09734a', soft: 'rgba(19,180,118,.09)' },
}

export function handoffTheme(color?: string): CSSProperties {
  const primary = (color || '#6c4fe0').toLowerCase()
  const stop = brandStops[primary]
  return {
    '--brand': primary,
    '--brand-dark': stop?.dark ?? `color-mix(in srgb, ${primary} 72%, #0d121c)`,
    '--brand-soft': stop?.soft ?? `color-mix(in srgb, ${primary} 8%, transparent)`,
  } as CSSProperties
}

export function toFaDigits(value: string | number) {
  return String(value).replace(/[0-9]/g, (digit) => '۰۱۲۳۴۵۶۷۸۹'[Number(digit)])
}

export function formatPrice(value: number, currency = 'IRR') {
  const displayValue = currency === 'IRR' ? Math.round(value / 10) : value
  return toFaDigits(Math.max(0, displayValue).toLocaleString('en-US').replace(/,/g, '٬'))
}

export function formatTime(value: string) {
  return new Intl.DateTimeFormat('fa-IR', { hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

export function formatDate(value?: string | null) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('fa-IR', { month: 'long', day: 'numeric' }).format(new Date(value))
}

const iconNames: Record<string, string> = {
  cleaning: 'ri-brush-line',
  water: 'ri-drop-line',
  coffee: 'ri-cup-line',
  amenities: 'ri-hand-heart-line',
  clock: 'ri-time-line',
  car: 'ri-taxi-line',
  concierge: 'ri-service-line',
  wellness: 'ri-leaf-line',
}

export function serviceIcon(service: Pick<Service, 'icon' | 'category'>) {
  return iconNames[service.icon] ?? ({
    housekeeping: 'ri-brush-line',
    food_beverage: 'ri-restaurant-line',
    transport: 'ri-taxi-line',
    wellness: 'ri-leaf-line',
    other: 'ri-service-line',
  } as const)[service.category]
}

export const requestStatusMeta: Record<ServiceRequestStatus, { label: string; className: string }> = {
  new: { label: 'جدید', className: 'info' },
  in_progress: { label: 'در حال انجام', className: 'warning' },
  completed: { label: 'تکمیل شد', className: 'success' },
  cancelled: { label: 'لغو شد', className: 'error' },
}

export function mergeRequest(items: ServiceRequest[], request: ServiceRequest) {
  const next = items.some((item) => item.id === request.id)
    ? items.map((item) => item.id === request.id ? request : item)
    : [request, ...items]
  return next.sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
}
