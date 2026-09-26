import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { ProgressPanel } from './ProgressPanel'

const topics = [
  { topic_id: 't1', title: 'Atoms', strength: 0.9, level: 'mastered' },
  { topic_id: 't2', title: 'Bonding', strength: 0.3, level: 'learning' },
  { topic_id: 't3', title: 'Moles', strength: 0, level: 'new' },
]

describe('ProgressPanel', () => {
  it('shows completed and mastered as two separate figures', () => {
    renderWithProviders(<ProgressPanel blocksCompleted={3} blocksTotal={4} topics={topics} />)

    const completed = screen.getByRole('progressbar', { name: /completed/i })
    expect(completed).toHaveAttribute('aria-valuenow', '75')

    // 1 of 3 topics mastered = 33%.
    const mastered = screen.getByRole('progressbar', { name: /mastered/i })
    expect(mastered).toHaveAttribute('aria-valuenow', '33')
  })

  it('lists every topic with its level, including untouched ones', () => {
    renderWithProviders(<ProgressPanel blocksCompleted={0} blocksTotal={4} topics={topics} />)
    expect(screen.getByText('Atoms')).toBeInTheDocument()
    expect(screen.getByText('Moles')).toBeInTheDocument()
    expect(screen.getByText('new')).toBeInTheDocument()
  })

  it('does not divide by zero in a course with no blocks or topics', () => {
    renderWithProviders(<ProgressPanel blocksCompleted={0} blocksTotal={0} topics={[]} />)
    expect(screen.getByRole('progressbar', { name: /completed/i })).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getByRole('progressbar', { name: /mastered/i })).toHaveAttribute('aria-valuenow', '0')
  })

  it('explains why the two numbers differ', () => {
    renderWithProviders(<ProgressPanel blocksCompleted={4} blocksTotal={4} topics={topics} />)
    expect(screen.getByText(/completed is what you opened/i)).toBeInTheDocument()
  })
})
