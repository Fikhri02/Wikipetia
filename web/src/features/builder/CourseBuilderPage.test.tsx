import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CourseBuilderPage } from './CourseBuilderPage'

const useOutline = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ courseId: 'c1' }),
  Link: ({ children, ...rest }: never) => <a {...(rest as object)}>{children}</a>,
}))
vi.mock('../../api/hooks', async () => {
  const actual = await vi.importActual<typeof import('../../api/hooks')>('../../api/hooks')
  return {
    ...actual,
    useOutline: () => useOutline(),
    useCreateModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useRenameModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useArchiveModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useMoveModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useCreateBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useUpdateBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useArchiveBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useMoveBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useUpdateCourse: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useCreateTopic: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useArchiveTopic: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  }
})

beforeEach(() => vi.clearAllMocks())

const outline = {
  course: { id: 'c1', title: 'Chemistry', description: '', visibility: 'private', pet_enabled: false },
  topics: [{ id: 't1', title: 'Atoms', sort_key: 'V' }],
  modules: [
    {
      id: 'm1',
      title: 'Module 1',
      sort_key: 'V',
      blocks: [
        {
          id: 'b1', module_id: 'm1', title: 'Reading', type: 'reading', sort_key: 'V',
          content: { body_md: '# Atoms' }, completion: { kind: 'mark_done' },
          topic_ids: ['t1'], question_count: 0,
        },
      ],
    },
  ],
}

describe('CourseBuilderPage', () => {
  it('renders the course, its modules and its blocks', () => {
    useOutline.mockReturnValue({ data: outline, isLoading: false, error: null })
    renderWithProviders(<CourseBuilderPage />)

    expect(screen.getByRole('heading', { name: 'Chemistry' })).toBeInTheDocument()
    expect(screen.getByLabelText(/module title: module 1/i)).toBeInTheDocument()
    // "Reading" also appears as an <option> in the block-type selector, so
    // assert on the block row's own control instead.
    expect(screen.getByLabelText('Edit Reading')).toBeInTheDocument()
  })

  it('disables moving the only module in either direction', () => {
    useOutline.mockReturnValue({ data: outline, isLoading: false, error: null })
    renderWithProviders(<CourseBuilderPage />)

    expect(screen.getByLabelText(/move module 1 up/i)).toBeDisabled()
    expect(screen.getByLabelText(/move module 1 down/i)).toBeDisabled()
  })

  it('renders an error banner when the outline fails to load', () => {
    useOutline.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: { title: 'Not Found', detail: 'course c1', fields: {} },
    })
    renderWithProviders(<CourseBuilderPage />)
    expect(screen.getByRole('alert')).toHaveTextContent('Not Found')
  })
})
