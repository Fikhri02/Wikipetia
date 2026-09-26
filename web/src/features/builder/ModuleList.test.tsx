import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { ModuleList } from './ModuleList'

const moveState = { mutate: vi.fn(), isPending: false, error: null as unknown }
const archiveState = { mutate: vi.fn(), isPending: false, error: null as unknown }

vi.mock('../../api/hooks', async () => {
  const actual = await vi.importActual<typeof import('../../api/hooks')>('../../api/hooks')
  return {
    ...actual,
    useCreateModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useRenameModule: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useArchiveModule: () => archiveState,
    useMoveModule: () => moveState,
    useCreateBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useArchiveBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useMoveBlock: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  }
})

const modules = [
  { id: 'm1', title: 'Module 1', sort_key: 'V', blocks: [] },
  { id: 'm2', title: 'Module 2', sort_key: 'k', blocks: [] },
]

beforeEach(() => {
  vi.clearAllMocks()
  moveState.error = null
  archiveState.error = null
})

describe('ModuleList', () => {
  it('surfaces a failed reorder instead of silently snapping back', () => {
    // The optimistic update rolls back on error, so without a visible message
    // the list just jumps to its old order and the user cannot tell a rejection
    // from a missed click.
    moveState.error = { title: 'Conflict', detail: 'module was moved elsewhere', fields: {} }

    renderWithProviders(<ModuleList courseId="c1" modules={modules} topics={[]} />)

    expect(screen.getByRole('alert')).toHaveTextContent('Conflict')
  })

  it('surfaces a failed archive', () => {
    archiveState.error = { title: 'Forbidden', detail: 'not yours', fields: {} }

    renderWithProviders(<ModuleList courseId="c1" modules={modules} topics={[]} />)

    expect(screen.getByRole('alert')).toHaveTextContent('Forbidden')
  })

  it('shows no alert when nothing has failed', () => {
    renderWithProviders(<ModuleList courseId="c1" modules={modules} topics={[]} />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
