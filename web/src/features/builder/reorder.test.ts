import { describe, it, expect } from 'vitest'
import { moveWithin } from './reorder'

const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]

describe('moveWithin', () => {
  it('moves an item up and anchors before its new successor', () => {
    const { next, before, after } = moveWithin(items, 'c', -1)
    expect(next.map((i) => i.id)).toEqual(['a', 'c', 'b'])
    expect(before).toBe('b')
    expect(after).toBeUndefined()
  })

  it('moves an item down and anchors after its new predecessor', () => {
    const { next, before, after } = moveWithin(items, 'a', 1)
    expect(next.map((i) => i.id)).toEqual(['b', 'a', 'c'])
    expect(after).toBe('b')
    expect(before).toBeUndefined()
  })

  it('is a no-op at the top', () => {
    const { next, before, after } = moveWithin(items, 'a', -1)
    expect(next.map((i) => i.id)).toEqual(['a', 'b', 'c'])
    expect(before).toBeUndefined()
    expect(after).toBeUndefined()
  })

  it('is a no-op at the bottom', () => {
    const { next } = moveWithin(items, 'c', 1)
    expect(next.map((i) => i.id)).toEqual(['a', 'b', 'c'])
  })

  it('is a no-op for an unknown id', () => {
    const { next } = moveWithin(items, 'zzz', -1)
    expect(next.map((i) => i.id)).toEqual(['a', 'b', 'c'])
  })

  it('never anchors an item against itself', () => {
    for (const id of ['a', 'b', 'c']) {
      for (const dir of [-1, 1] as const) {
        const { before, after } = moveWithin(items, id, dir)
        expect(before).not.toBe(id)
        expect(after).not.toBe(id)
      }
    }
  })
})
