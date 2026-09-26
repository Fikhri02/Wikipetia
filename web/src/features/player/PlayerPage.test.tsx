import type { ReactNode } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { PlayerPage } from './PlayerPage'

const usePlayer = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ enrolmentId: 'e1' }),
  Link: ({ children, ...rest }: { children?: ReactNode; [key: string]: unknown }) => (
    <a {...(rest as Record<string, unknown>)}>{children}</a>
  ),
}))

vi.mock('../../api/hooks', () => ({
  usePlayer: () => usePlayer(),
  useCompleteBlock: () => ({ mutate: completeMutate, isPending: false, error: null }),
  useSubmitQuiz: () => ({ mutate: vi.fn(), isPending: false, error: null, data: undefined }),
}))

const completeMutate = vi.fn()

beforeEach(() => vi.clearAllMocks())

function quiz(id: string, title: string) {
  return {
    id,
    title,
    type: 'quiz',
    content: {},
    completion: { kind: 'score_threshold', threshold: 70 },
    topic_ids: [],
    status: 'not_started',
    attempts: 0,
    // A typed question so the draft answer is observable in the DOM.
    questions: [{ id: `${id}-q`, kind: 'typed', prompt: { text: `${title} question` } }],
  }
}

const view = {
  enrolment_id: 'e1',
  course_id: 'c1',
  course_title: 'Chemistry',
  pet_enabled: false,
  mode: 'self_study',
  started_at: '2026-09-01T00:00:00Z',
  blocks_total: 2,
  blocks_completed: 0,
  topics: [],
  modules: [{ id: 'm1', title: 'Module 1', blocks: [quiz('b1', 'Quiz A'), quiz('b2', 'Quiz B')] }],
}

describe('PlayerPage', () => {
  it("does not carry one block's in-progress answers into another block", () => {
    usePlayer.mockReturnValue({ data: view, isLoading: false, error: null })
    renderWithProviders(<PlayerPage />)

    // Answer Quiz A.
    const answerA = screen.getByLabelText('Quiz A question')
    fireEvent.change(answerA, { target: { value: 'my answer to A' } })
    expect(screen.getByLabelText('Quiz A question')).toHaveValue('my answer to A')

    // Switch to Quiz B: it must be a fresh surface, not Quiz A's state reused.
    fireEvent.click(screen.getByRole('button', { name: /Quiz B/ }))
    expect(screen.getByLabelText('Quiz B question')).toHaveValue('')

    // And going back must not have leaked B's emptiness onto A either way —
    // both are simply fresh, which is what a remount guarantees.
    fireEvent.click(screen.getByRole('button', { name: /Quiz A/ }))
    expect(screen.getByLabelText('Quiz A question')).toHaveValue('')
  })
})

describe('PlayerPage selection', () => {
  it('stays on the block the learner is reading when it completes', () => {
    // The fallback "first unfinished block" is an INITIAL choice. Recomputing it
    // on every render means marking a reading done yanks the page to the next
    // block, so the learner never sees the Completed confirmation they just
    // earned — and any in-progress state on screen vanishes.
    const reading = {
      id: 'r1',
      title: 'What is an atom?',
      type: 'reading',
      content: { body_md: '# Atoms' },
      completion: { kind: 'mark_done' },
      topic_ids: [],
      status: 'not_started',
      attempts: 0,
    }
    const withReading = {
      ...view,
      blocks_total: 2,
      modules: [{ id: 'm1', title: 'Module 1', blocks: [reading, quiz('b2', 'Quiz B')] }],
    }

    usePlayer.mockReturnValue({ data: withReading, isLoading: false, error: null })
    const { rerender } = renderWithProviders(<PlayerPage />)

    // The reading is shown first, being the first unfinished block.
    expect(screen.getByText('# Atoms')).toBeInTheDocument()

    // It completes; the server now reports it completed.
    usePlayer.mockReturnValue({
      data: {
        ...withReading,
        blocks_completed: 1,
        modules: [
          { id: 'm1', title: 'Module 1', blocks: [{ ...reading, status: 'completed' }, quiz('b2', 'Quiz B')] },
        ],
      },
      isLoading: false,
      error: null,
    })
    rerender(<PlayerPage />)

    // We must still be on the reading, now showing its completed state.
    expect(screen.getByText('# Atoms')).toBeInTheDocument()
    expect(screen.queryByText('Quiz B question')).not.toBeInTheDocument()
  })
})
