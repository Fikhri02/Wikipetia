import { useEffect, useMemo, useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { usePlayer } from '../../api/hooks'
import { ProgressPanel } from './ProgressPanel'
import { BlockNav } from './BlockNav'
import { BlockSurface } from '../../blocks/BlockSurface'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Spinner } from '../../components/Spinner'
import { asApiError } from '../../api/errors'

export function PlayerPage() {
  const { enrolmentId } = useParams({ from: '/learn/$enrolmentId' })
  const player = usePlayer(enrolmentId)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const blocks = useMemo(
    () => (player.data?.modules ?? []).flatMap((m) => m.blocks ?? []),
    [player.data],
  )

  // Default to the first unfinished block: resuming should not mean scrolling.
  const selected =
    blocks.find((b) => b.id === selectedId) ??
    blocks.find((b) => b.status !== 'completed') ??
    blocks[0]

  // Pin that default as soon as there is one. Without this the fallback is
  // re-evaluated on every render, so completing a block yanks the learner to
  // the next one and they never see the confirmation they just earned.
  useEffect(() => {
    if (selectedId === null && selected) setSelectedId(selected.id)
  }, [selectedId, selected])

  if (player.isLoading) return <Spinner />
  if (player.error) return <ErrorBanner error={asApiError(player.error)} />
  if (!player.data) return null

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-xl font-semibold tracking-tight">{player.data.course_title}</h1>
        <p className="text-sm text-muted">
          Started {new Date(player.data.started_at).toLocaleDateString()}
        </p>
      </header>

      <div className="grid gap-8 lg:grid-cols-[1fr_2fr_1fr]">
        <BlockNav
          modules={player.data.modules ?? []}
          selectedId={selected?.id ?? null}
          onSelect={setSelectedId}
        />

        <div>
          {selected ? (
            // `key` is load-bearing: without it React reuses the same
            // BlockSurface instance across a block change, so a quiz score, a
            // stale error, and a torn-down YouTube iframe all leak into the
            // next block.
            <BlockSurface key={selected.id} enrolmentId={enrolmentId} block={selected} />
          ) : (
            <p className="text-muted">This course has no blocks yet.</p>
          )}
        </div>

        <ProgressPanel
          blocksCompleted={player.data.blocks_completed}
          blocksTotal={player.data.blocks_total}
          topics={player.data.topics ?? []}
        />
      </div>
    </div>
  )
}
