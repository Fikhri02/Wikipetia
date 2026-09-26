// Replaced in Task 10.
import type { PlayerBlockProps } from './players'
export function VideoBlock({ block }: PlayerBlockProps) {
  return <p className="text-sm text-muted">Video “{block.title}” arrives in Task 10.</p>
}
