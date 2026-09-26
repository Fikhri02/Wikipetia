import { useCompleteBlock } from '../api/hooks'
import { parseBlockContent } from './schemas'
import { playerFor, type PlayerBlockData } from './players'
import { ErrorBanner } from '../components/ErrorBanner'
import { asApiError } from '../api/errors'

export function BlockSurface({
  enrolmentId,
  block,
}: {
  enrolmentId: string
  block: PlayerBlockData
}) {
  const complete = useCompleteBlock(enrolmentId)
  const Player = playerFor(block.type)

  if (!Player) {
    return (
      <p className="text-sm text-red-700">
        This course uses a block type this app does not know: “{block.type}”.
      </p>
    )
  }

  // Quiz blocks carry no content, so only non-quiz payloads are gated here.
  const parsed = parseBlockContent(block.type, block.content)
  if (!parsed.ok && block.type !== 'quiz') {
    return <p className="text-sm text-red-700">This block could not be displayed ({parsed.message}).</p>
  }

  return (
    <div className="space-y-3">
      <Player
        enrolmentId={enrolmentId}
        block={block}
        pending={complete.isPending}
        onComplete={(signal) => complete.mutate({ blockId: block.id, ...signal })}
      />
      <ErrorBanner error={asApiError(complete.error)} />
    </div>
  )
}
