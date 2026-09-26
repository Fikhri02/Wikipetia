import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../test-utils'
import { QuizBlock } from './QuizBlock'
import type { PlayerBlockData } from './players'

const submitMutate = vi.fn()
const submitState = {
  mutate: submitMutate,
  isPending: false,
  error: null as unknown,
  data: undefined as unknown,
  reset: vi.fn(),
}

vi.mock('../api/hooks', () => ({ useSubmitQuiz: () => submitState }))

beforeEach(() => {
  vi.clearAllMocks()
  submitState.data = undefined
  submitState.error = null
})

const block = {
  id: 'b1',
  title: 'Check yourself',
  type: 'quiz',
  content: {},
  completion: { kind: 'score_threshold', threshold: 70 },
  topic_ids: [],
  status: 'not_started',
  attempts: 0,
  questions: [
    { id: 'q1', kind: 'mcq_single', prompt: { text: 'Which is an element?', options: ['Water', 'Iron'] } },
    { id: 'q2', kind: 'true_false', prompt: { text: 'Iron is an element' } },
    { id: 'q3', kind: 'typed', prompt: { text: 'Symbol for iron?' } },
  ],
  // Typed rather than `as never`: the malformed-prompt test spreads this, and
  // a `never` cannot be spread.
} satisfies PlayerBlockData

const props = { enrolmentId: 'e1', block, onComplete: vi.fn(), pending: false }

describe('QuizBlock', () => {
  it('renders one input per question kind', () => {
    renderWithProviders(<QuizBlock {...props} />)
    expect(screen.getByRole('radio', { name: 'Iron' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /^true$/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/symbol for iron/i)).toBeInTheDocument()
  })

  it('submits the answers the learner actually gave', () => {
    renderWithProviders(<QuizBlock {...props} />)

    fireEvent.click(screen.getByRole('radio', { name: 'Iron' }))
    fireEvent.click(screen.getByRole('radio', { name: /^true$/i }))
    fireEvent.change(screen.getByLabelText(/symbol for iron/i), { target: { value: 'Fe' } })
    fireEvent.click(screen.getByRole('button', { name: /submit/i }))

    expect(submitMutate).toHaveBeenCalledWith({
      blockId: 'b1',
      answers: [
        { question_id: 'q1', choice_index: 1 },
        { question_id: 'q2', boolean: true },
        { question_id: 'q3', text: 'Fe' },
      ],
    })
  })

  it('omits questions left unanswered rather than inventing an answer', () => {
    renderWithProviders(<QuizBlock {...props} />)

    fireEvent.click(screen.getByRole('radio', { name: 'Iron' }))
    fireEvent.click(screen.getByRole('button', { name: /submit/i }))

    expect(submitMutate).toHaveBeenCalledWith({
      blockId: 'b1',
      answers: [{ question_id: 'q1', choice_index: 1 }],
    })
  })

  it('shows the score and per-question outcome after submitting', () => {
    submitState.data = {
      score: 67,
      progress: { block_id: 'b1', status: 'in_progress', score: 67, attempts: 1 },
      results: [
        { question_id: 'q1', correct: true },
        { question_id: 'q2', correct: true },
        { question_id: 'q3', correct: false },
      ],
      mastery: [{ topic_id: 't1', strength: 0.67, level: 'proficient' }],
    }
    renderWithProviders(<QuizBlock {...props} />)

    expect(screen.getByText(/67%/)).toBeInTheDocument()
    expect(screen.getByText(/below the 70% needed/i)).toBeInTheDocument()
  })

  it('never reveals the correct answer, only whether you were right', () => {
    submitState.data = {
      score: 0,
      progress: { block_id: 'b1', status: 'in_progress', score: 0, attempts: 1 },
      results: [{ question_id: 'q1', correct: false }],
      mastery: [],
    }
    renderWithProviders(<QuizBlock {...props} />)

    expect(screen.queryByText(/correct answer/i)).not.toBeInTheDocument()
    expect(screen.getByText(/incorrect/i)).toBeInTheDocument()
  })

  it('reports a malformed prompt without losing the rest of the quiz', () => {
    const broken = {
      ...block,
      questions: [
        { id: 'q1', kind: 'mcq_single', prompt: { nope: true } },
        { id: 'q2', kind: 'true_false', prompt: { text: 'Iron is an element' } },
      ],
    } satisfies PlayerBlockData
    renderWithProviders(<QuizBlock {...props} block={broken} />)

    expect(screen.getByText(/could not be displayed/i)).toBeInTheDocument()
    expect(screen.getByText('Iron is an element')).toBeInTheDocument()
  })
})
