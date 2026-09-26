import { describe, it, expect } from 'vitest'
import { apiError, fieldError } from './errors'

describe('apiError', () => {
  it('narrows a problem+json body into title, detail and field errors', () => {
    const err = apiError({
      title: 'Unprocessable Entity',
      status: 422,
      detail: 'validation failed',
      errors: [
        { location: 'body.title', message: 'must not be empty' },
        { location: 'body.completion.threshold', message: 'must be between 0 and 100' },
      ],
    })

    expect(err.status).toBe(422)
    expect(err.detail).toBe('validation failed')
    expect(err.fields['title']).toBe('must not be empty')
    expect(err.fields['completion.threshold']).toBe('must be between 0 and 100')
  })

  it('keeps path errors distinguishable from body errors', () => {
    const err = apiError({
      title: 'Unprocessable Entity',
      status: 422,
      errors: [{ location: 'path.id', message: 'invalid UUID length: 10' }],
    })
    expect(err.fields['path.id']).toBe('invalid UUID length: 10')
  })

  it('survives an error with no errors array', () => {
    const err = apiError({ title: 'Not Found', status: 404, detail: 'course abc' })
    expect(err.title).toBe('Not Found')
    expect(err.detail).toBe('course abc')
    expect(err.fields).toEqual({})
  })

  it('survives something that is not a problem+json body at all', () => {
    const err = apiError(new TypeError('Failed to fetch'))
    expect(err.title).toBe('Network error')
    expect(err.detail).toContain('Failed to fetch')
  })

  it('survives undefined', () => {
    expect(apiError(undefined).title).toBe('Something went wrong')
  })
})

describe('fieldError', () => {
  it('reads one field', () => {
    const err = apiError({
      status: 422,
      errors: [{ location: 'body.title', message: 'must not be empty' }],
    })
    expect(fieldError(err, 'title')).toBe('must not be empty')
    expect(fieldError(err, 'description')).toBeUndefined()
  })
})
