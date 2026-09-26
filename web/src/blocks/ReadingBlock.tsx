import { readingContent } from './schemas'
import { Button } from '../components/Button'
import type { PlayerBlockProps } from './players'

export function ReadingBlock({ block, onComplete, pending }: PlayerBlockProps) {
  // Parsed again here so this component is safe to render in isolation.
  const parsed = readingContent.safeParse(block.content)
  const done = block.status === 'completed'

  return (
    <article className="space-y-4">
      <h2 className="text-lg font-semibold">{block.title}</h2>

      {parsed.success ? (
        <div className="whitespace-pre-wrap text-sm leading-relaxed">{parsed.data.body_md}</div>
      ) : (
        <p className="text-sm text-red-700">This reading could not be displayed.</p>
      )}

      {done ? (
        <p className="text-sm text-emerald-700">Completed</p>
      ) : (
        <Button onClick={() => onComplete({ marked_done: true })} disabled={pending}>
          Mark as done
        </Button>
      )}
    </article>
  )
}
