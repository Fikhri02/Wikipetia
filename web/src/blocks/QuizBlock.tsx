// Replaced in Task 11.
import type { PlayerBlockProps } from './players'
export function QuizBlock({ block }: PlayerBlockProps) {
  return <p className="text-sm text-muted">Quiz “{block.title}” arrives in Task 11.</p>
}
