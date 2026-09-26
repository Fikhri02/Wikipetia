import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CourseListPage } from './CourseListPage'

const useMyCourses = vi.fn()
const createMutate = vi.fn()
const createState = { mutate: createMutate, isPending: false, error: null as unknown }

vi.mock('../../api/hooks', () => ({
  useMyCourses: () => useMyCourses(),
  useCreateCourse: () => createState,
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, ...rest }: never) => <a {...(rest as object)}>{children}</a>,
}))

beforeEach(() => {
  vi.clearAllMocks()
  createState.error = null
})

describe('CourseListPage', () => {
  it('lists the courses I own', () => {
    useMyCourses.mockReturnValue({
      data: [{ id: 'c1', title: 'Chemistry', description: '', visibility: 'private', pet_enabled: false }],
      isLoading: false,
      error: null,
    })
    renderWithProviders(<CourseListPage />)
    expect(screen.getByText('Chemistry')).toBeInTheDocument()
  })

  it('creates a course from the form', () => {
    useMyCourses.mockReturnValue({ data: [], isLoading: false, error: null })
    renderWithProviders(<CourseListPage />)

    fireEvent.change(screen.getByLabelText(/course title/i), { target: { value: 'Physics' } })
    fireEvent.click(screen.getByRole('button', { name: /create course/i }))

    expect(createMutate).toHaveBeenCalledWith('Physics', expect.anything())
  })

  it('shows the server field error against the title input', () => {
    useMyCourses.mockReturnValue({ data: [], isLoading: false, error: null })
    createState.error = { title: 'Unprocessable Entity', detail: 'validation failed', fields: { title: 'must not be empty' } }

    renderWithProviders(<CourseListPage />)
    expect(screen.getByText('must not be empty')).toBeInTheDocument()
    expect(screen.getByLabelText(/course title/i)).toHaveAttribute('aria-invalid', 'true')
  })
})
