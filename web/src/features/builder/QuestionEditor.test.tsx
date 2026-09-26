import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { QuestionEditor } from './QuestionEditor'

const createMutate = vi.fn()
const useQuestions = vi.fn()

vi.mock('../../api/hooks', () => ({
  useQuestions: () => useQuestions(),
  useCreateQuestion: () => ({ mutate: createMutate, isPending: false, error: null }),
  useArchiveQuestion: () => ({ mutate: vi.fn(), isPending: false, error: null }),
}))

beforeEach(() => {
  vi.clearAllMocks()
  useQuestions.mockReturnValue({ data: [], isLoading: false, error: null })
})

const topics = [{ id: 't1', title: 'Atoms' }]

describe('QuestionEditor', () => {
  it('builds an mcq question with its options and correct index', () => {
    renderWithProviders(<QuestionEditor courseId="c1" blockId="b1" topics={topics} />)

    fireEvent.change(screen.getByLabelText('Question'), { target: { value: 'Which is an element?' } })
    fireEvent.change(screen.getByLabelText('Option 1'), { target: { value: 'Water' } })
    fireEvent.change(screen.getByLabelText('Option 2'), { target: { value: 'Iron' } })
    fireEvent.click(screen.getByLabelText('Option 2 is correct'))
    fireEvent.click(screen.getByRole('button', { name: 'Add question' }))

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'mcq_single',
        prompt: { text: 'Which is an element?', options: ['Water', 'Iron'] },
        answer: { correct_index: 1 },
      }),
      expect.anything(),
    )
  })

  it('builds a true/false question', () => {
    renderWithProviders(<QuestionEditor courseId="c1" blockId="b1" topics={topics} />)

    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'true_false' } })
    fireEvent.change(screen.getByLabelText('Question'), { target: { value: 'Iron is an element' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add question' }))

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'true_false', answer: { correct: true } }),
      expect.anything(),
    )
  })

  it('lists existing questions with their prompts', () => {
    useQuestions.mockReturnValue({
      data: [
        { id: 'q1', kind: 'true_false', prompt: { text: 'Iron is an element' }, answer: { correct: true }, sort_key: 'V' },
      ],
      isLoading: false,
      error: null,
    })
    renderWithProviders(<QuestionEditor courseId="c1" blockId="b1" topics={topics} />)
    expect(screen.getByText('Iron is an element')).toBeInTheDocument()
  })

  it('reports a malformed prompt rather than rendering nothing', () => {
    useQuestions.mockReturnValue({
      data: [{ id: 'q2', kind: 'mcq_single', prompt: { nonsense: true }, answer: {}, sort_key: 'V' }],
      isLoading: false,
      error: null,
    })
    renderWithProviders(<QuestionEditor courseId="c1" blockId="b1" topics={topics} />)
    expect(screen.getByText(/malformed/i)).toBeInTheDocument()
  })
})
