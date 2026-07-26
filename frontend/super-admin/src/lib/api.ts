// Typed API client for the super-admin console — injects the SUPER_ADMIN bearer
// token automatically.

import { getSession } from '../auth'

const BASE = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = getSession()?.token ?? ''
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw Object.assign(
      new Error((err as { message?: string }).message ?? `HTTP ${res.status}`),
      { status: res.status, code: (err as { error?: string }).error },
    )
  }
  // Some endpoints return empty bodies.
  const text = await res.text()
  return (text ? JSON.parse(text) : {}) as T
}

export const api = {
  get:    <T>(path: string)                 => request<T>('GET',    path),
  post:   <T>(path: string, body?: unknown) => request<T>('POST',   path, body),
  patch:  <T>(path: string, body?: unknown) => request<T>('PATCH',  path, body),
  delete: <T>(path: string)                 => request<T>('DELETE', path),
}

export interface Tenant {
  tenant_id: string
  name: string
  domain: string
  institution_id: string
  attendance_threshold: number
  is_active: boolean
  created_at: string
  active_academic_year: string
  active_semester: number
  logo_url: string
  brand_color: string
  sidebar_color: string
  background_color: string
  footer_color: string
  text_color_light?: string
  text_color_dark?: string
  motto: string
  slogan: string
  address: string
}

export interface TenantUser {
  user_id: string
  email: string
  role: string
  full_name: string
  is_active: boolean
  last_login_at: string | null
  created_at: string
}

export interface Branding {
  tenant_id: string
  name: string
  logo_url: string
  motto: string
  slogan: string
  brand_color: string
  sidebar_color: string
  background_color: string
  footer_color: string
  text_color_light?: string
  text_color_dark?: string
  address: string
}
