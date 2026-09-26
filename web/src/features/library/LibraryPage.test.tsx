import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { LibraryPage } from './LibraryPage'

const useEnrolments = vi.fn()
const useMyCourses = vi.fn()
const enrolMutate = vi.fn()

vi.mock('../../api/hooks', () => ({
  useEnrolments: () => useEnrolments(),
  useMyCourses: () => useMyCourses(),
  useEnrol: () => ({ mutate: enrolMutate, isPending: false, error: null }),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, ...rest }: never) => <a {...(rest as object)}>{children}</a>,
}))

beforeEach(() => {
  vi.clearAllMocks()
  useMyCourses.mockReturnValue({ data: [], isLoading: false, error: null })
})

describe('LibraryPage', () => {
  it('shows a spinner while loading', () => {
    useEnrolments.mockReturnValue({ data: undefined, isLoading: true, error: null })
    renderWithProviders(<LibraryPage />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('lists enrolments with their course titles', async () => {
    useEnrolments.mockReturnValue({
      data: [
        { id: 'e1', course_id: 'c1', course_title: 'Form 4 Chemistry', mode: 'self_study', started_at: '2026-09-01T00:00:00Z' },
      ],
      isLoading: false,
      error: null,
    })
    renderWithProviders(<LibraryPage />)
    await waitFor(() => expect(screen.getByText('Form 4 Chemistry')).toBeInTheDocument())
  })

  it('offers an empty state when nothing is enrolled', () => {
    useEnrolments.mockReturnValue({ data: [], isLoading: false, error: null })
    renderWithProviders(<LibraryPage />)
    expect(screen.getByText(/nothing here yet/i)).toBeInTheDocument()
  })

  it('renders an error banner rather than crashing', () => {
    useEnrolments.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: { title: 'Network error', detail: 'Failed to fetch', fields: {} },
    })
    renderWithProviders(<LibraryPage />)
    expect(screen.getByRole('alert')).toHaveTextContent('Network error')
  })

  it('enrols in a course that is not yet started', () => {
    useEnrolments.mockReturnValue({ data: [], isLoading: false, error: null })
    useMyCourses.mockReturnValue({
      data: [{ id: 'c9', title: 'Physics', description: '', visibility: 'private', pet_enabled: false }],
      isLoading: false,
      error: null,
    })
    renderWithProviders(<LibraryPage />)
    fireEvent.click(screen.getByRole('button', { name: /start physics/i }))
    expect(enrolMutate).toHaveBeenCalledWith('c9')
  })
})
