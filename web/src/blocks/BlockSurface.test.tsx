import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../test-utils'
import { BlockSurface } from './BlockSurface'

const completeMutate = vi.fn()
vi.mock('../api/hooks', () => ({
  useCompleteBlock: () => ({ mutate: completeMutate, isPending: false, error: null }),
  useSubmitQuiz: () => ({ mutate: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() }),
}))

beforeEach(() => vi.clearAllMocks())

function blockOf(overrides: Record<string, unknown>) {
  return {
    id: 'b1',
    title: 'A block',
    type: 'reading',
    content: { body_md: '# Atoms' },
    completion: { kind: 'mark_done' },
    topic_ids: [],
    status: 'not_started',
    attempts: 0,
    ...overrides,
  } as never
}

describe('BlockSurface', () => {
  it('renders the component the registry maps the type to', () => {
    renderWithProviders(<BlockSurface enrolmentId="e1" block={blockOf({})} />)
    expect(screen.getByText('# Atoms')).toBeInTheDocument()
  })

  it('reports an unknown block type instead of rendering blank', () => {
    renderWithProviders(<BlockSurface enrolmentId="e1" block={blockOf({ type: 'hologram' })} />)
    expect(screen.getByText(/hologram/)).toBeInTheDocument()
  })

  it('reports a malformed payload instead of crashing', () => {
    renderWithProviders(
      <BlockSurface enrolmentId="e1" block={blockOf({ content: { wrong: true } })} />,
    )
    expect(screen.getByText(/could not be displayed/i)).toBeInTheDocument()
  })

  it('marks a reading block done', () => {
    renderWithProviders(<BlockSurface enrolmentId="e1" block={blockOf({})} />)
    fireEvent.click(screen.getByRole('button', { name: /mark as done/i }))
    expect(completeMutate).toHaveBeenCalledWith({ blockId: 'b1', marked_done: true })
  })

  it('shows a completed reading block as done rather than offering the button again', () => {
    renderWithProviders(<BlockSurface enrolmentId="e1" block={blockOf({ status: 'completed' })} />)
    expect(screen.queryByRole('button', { name: /mark as done/i })).not.toBeInTheDocument()
    expect(screen.getByText(/completed/i)).toBeInTheDocument()
  })
})
