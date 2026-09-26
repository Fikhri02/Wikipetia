import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { TopicPanel } from './TopicPanel'

type SuccessCb = { onSuccess?: () => void; onError?: () => void }
const createCalls: Array<{ title: string; cbs?: SuccessCb }> = []
const createState = {
  mutate: (title: string, cbs?: SuccessCb) => createCalls.push({ title, cbs }),
  isPending: false,
  error: null as unknown,
}

vi.mock('../../api/hooks', () => ({
  useCreateTopic: () => createState,
  useArchiveTopic: () => ({ mutate: vi.fn(), isPending: false, error: null }),
}))

beforeEach(() => {
  createCalls.length = 0
  createState.error = null
})

describe('TopicPanel', () => {
  it('clears the field on submit, so a late response cannot wipe newer typing', () => {
    // Clearing in onSuccess loses input: the response for topic N lands while
    // the user is already typing topic N+1, wipes it, and the next submit sends
    // an empty title — which the server rejects with 422. Adding four topics
    // quickly reproduced this.
    renderWithProviders(<TopicPanel courseId="c1" topics={[]} />)
    const input = screen.getByLabelText('New topic')

    fireEvent.change(input, { target: { value: 'Atoms' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(createCalls[0].title).toBe('Atoms')
    // Cleared immediately, not on response.
    expect(input).toHaveValue('')

    // The user types the next topic before the first response arrives.
    fireEvent.change(input, { target: { value: 'Bonding' } })
    act(() => createCalls[0].cbs?.onSuccess?.())

    // The late success must NOT have wiped it.
    expect(input).toHaveValue('Bonding')

    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(createCalls[1].title).toBe('Bonding')
  })

  it('restores the text when the create fails, so the work is not lost', () => {
    renderWithProviders(<TopicPanel courseId="c1" topics={[]} />)
    const input = screen.getByLabelText('New topic')

    fireEvent.change(input, { target: { value: 'Moles' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(input).toHaveValue('')

    act(() => createCalls[0].cbs?.onError?.())
    expect(input).toHaveValue('Moles')
  })

  it('does not submit an empty title at all', () => {
    renderWithProviders(<TopicPanel courseId="c1" topics={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(createCalls).toHaveLength(0)
  })
})
