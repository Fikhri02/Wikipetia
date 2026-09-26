import createClient from 'openapi-fetch'
import type { paths } from './types'

const DEV_USER_KEY = 'wikipetia.devUser'

/**
 * Slice 1 authenticates with a header the API's dev stub trusts completely.
 * This is a laptop-only arrangement; the spec requires a real provider before
 * slice 3, at which point only this file and the switcher change.
 */
export function devUser(): string {
  try {
    return localStorage.getItem(DEV_USER_KEY) ?? 'dev'
  } catch {
    return 'dev'
  }
}

export function setDevUser(name: string): void {
  try {
    localStorage.setItem(DEV_USER_KEY, name)
  } catch {
    // Private browsing: fall back to the default user for this session.
  }
}

export const api = createClient<paths>({
  // Same-origin through the Vite dev proxy, so there is no CORS to configure.
  baseUrl: '/api',
  headers: { 'content-type': 'application/json' },
})

api.use({
  onRequest({ request }) {
    request.headers.set('X-Dev-User', devUser())
    return request
  },
})

/**
 * Go marshals a nil slice as null, so every array in the generated types is
 * `T[] | null`. Normalising once here keeps `?? []` out of every component.
 */
export function list<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}
