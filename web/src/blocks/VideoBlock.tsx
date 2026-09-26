import { videoContent } from './schemas'
import { useYouTubePlayer } from './useYouTubePlayer'
import type { PlayerBlockProps } from './players'

export function VideoBlock({ block, onComplete }: PlayerBlockProps) {
  const parsed = videoContent.safeParse(block.content)
  const videoId = parsed.success ? parsed.data.video_id : ''
  const done = block.status === 'completed'

  const { containerRef, failed } = useYouTubePlayer({
    videoId,
    // Fires on the player's own ENDED state. Posting again after completion is
    // harmless: the API is idempotent and emits no second event.
    onEnded: () => onComplete({ watched_to_end: true }),
  })

  if (!parsed.success) {
    return <p className="text-sm text-red-700">This video could not be displayed.</p>
  }

  return (
    <article className="space-y-4">
      <h2 className="text-lg font-semibold">{block.title}</h2>

      <div className="aspect-video w-full overflow-hidden rounded-lg bg-slate-900">
        <div ref={containerRef} className="h-full w-full" />
      </div>

      {failed && (
        <p className="text-sm text-red-700">
          This video failed to load. It may have been removed or made private by its owner.
        </p>
      )}

      <p className="text-sm text-muted">
        {done ? 'Completed' : 'This completes itself when the video reaches the end.'}
      </p>
    </article>
  )
}
