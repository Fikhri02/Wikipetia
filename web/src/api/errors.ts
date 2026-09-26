import type { components } from './types'

type ErrorModel = components['schemas']['ErrorModel']

export type ApiError = {
  title: string
  detail: string
  status?: number
  /** Field name (with the `body.` prefix stripped) → message. */
  fields: Record<string, string>
}

function isErrorModel(value: unknown): value is ErrorModel {
  return typeof value === 'object' && value !== null && ('title' in value || 'status' in value)
}

/**
 * Narrows whatever openapi-fetch hands back into something a component can
 * render. The API always answers errors with problem+json, but a dropped
 * connection produces a TypeError instead, and that must not crash the UI.
 */
export function apiError(error: unknown): ApiError {
  if (isErrorModel(error)) {
    const fields: Record<string, string> = {}
    for (const detail of error.errors ?? []) {
      if (!detail?.location || !detail.message) continue
      // huma reports `body.title`; forms know the field as `title`.
      const key = detail.location.startsWith('body.')
        ? detail.location.slice('body.'.length)
        : detail.location
      fields[key] = detail.message
    }
    return {
      title: error.title ?? 'Request failed',
      detail: error.detail ?? '',
      status: error.status,
      fields,
    }
  }

  if (error instanceof Error) {
    return { title: 'Network error', detail: error.message, fields: {} }
  }

  return { title: 'Something went wrong', detail: '', fields: {} }
}

export function fieldError(err: ApiError, field: string): string | undefined {
  return err.fields[field]
}
