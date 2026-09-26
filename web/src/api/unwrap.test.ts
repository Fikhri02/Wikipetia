import { describe, it, expect } from 'vitest'
import { expectNoContent, unwrap } from './hooks'
import type { ApiError } from './errors'

function res(init: { data?: unknown; error?: unknown; status: number }) {
  return {
    data: init.data,
    error: init.error,
    response: new Response(null, { status: init.status }),
  } as never
}

describe('unwrap', () => {
  it('returns the body on success', () => {
    expect(unwrap(res({ data: { courses: [] }, status: 200 }))).toEqual({ courses: [] })
  })

  it('throws the narrowed problem+json error', () => {
    // The app throws plain ApiError objects, not Error instances, so assert on
    // the shape rather than on a message.
    let thrown: unknown
    try {
      unwrap(res({ error: { title: 'Not Found', status: 404 }, status: 404 }))
    } catch (e) {
      thrown = e
    }
    expect((thrown as ApiError).title).toBe('Not Found')
    expect((thrown as ApiError).status).toBe(404)
  })

  it('throws when the response failed but carried no parseable error body', () => {
    // A dead API behind the dev proxy answers 500 with a plain-text body, so
    // openapi-fetch leaves `error` undefined. Returning undefined here would
    // make a down server look like an empty library — the user would be told
    // they have no courses, which is a lie.
    expect(() => unwrap(res({ status: 500 }))).toThrowError()
  })

  it('throws when a 200 carries no body at all', () => {
    expect(() => unwrap(res({ status: 200 }))).toThrowError()
  })
})

describe('expectNoContent', () => {
  it('accepts a real 204', () => {
    expect(() => expectNoContent(res({ status: 204 }))).not.toThrow()
  })

  it('throws on a failed response with an empty body', () => {
    // openapi-fetch leaves `error` undefined for ANY response with no body,
    // non-ok included. Without a response.ok check, a 502 from a gateway is
    // indistinguishable from a successful 204 — so an archive that failed
    // reported success, and a move that failed never rolled back.
    let thrown: unknown
    try {
      expectNoContent(res({ status: 502 }))
    } catch (e) {
      thrown = e
    }
    expect(thrown).toBeDefined()
    expect((thrown as ApiError).title).toBe('Network error')
  })

  it('still surfaces a problem+json error body', () => {
    let thrown: unknown
    try {
      expectNoContent(res({ error: { title: 'Conflict', status: 409 }, status: 409 }))
    } catch (e) {
      thrown = e
    }
    expect((thrown as ApiError).title).toBe('Conflict')
  })
})
