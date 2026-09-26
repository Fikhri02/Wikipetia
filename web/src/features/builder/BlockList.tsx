import { useId, useState } from 'react'
import { useArchiveBlock, useCreateBlock, useMoveBlock } from '../../api/hooks'
import { moveWithin } from './reorder'
import { BlockEditor } from './BlockEditor'
import { BLOCK_TYPES, type BlockType } from '../../blocks/schemas'
import { Button } from '../../components/Button'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Field } from '../../components/Field'
import { asApiError } from '../../api/errors'

import type { components } from '../../api/types'

// Straight from the generated contract — see ModuleList.
type BlockShape = components['schemas']['OutlineBlockBody']

const defaultContent: Record<BlockType, unknown> = {
  reading: { body_md: 'Write the reading here.' },
  video_youtube: { video_id: 'dQw4w9WgXcQ' },
  quiz: {},
}

const typeLabels: Record<BlockType, string> = {
  reading: 'Reading',
  video_youtube: 'Video (YouTube)',
  quiz: 'Quiz',
}

export function BlockList({
  courseId,
  moduleId,
  blocks,
  topics,
}: {
  courseId: string
  moduleId: string
  blocks: BlockShape[]
  topics: { id: string; title: string }[]
}) {
  const create = useCreateBlock(courseId)
  const archive = useArchiveBlock(courseId)
  const move = useMoveBlock(courseId)

  const [title, setTitle] = useState('')
  const [type, setType] = useState<BlockType>('reading')
  const [openId, setOpenId] = useState<string | null>(null)
  // One BlockList per module, so this id must be per-instance.
  const typeId = useId()

  function reorder(id: string, direction: -1 | 1) {
    const { before, after } = moveWithin(blocks, id, direction)
    if (!before && !after) return
    move.mutate({ id, before, after })
  }

  const failure = asApiError(create.error ?? archive.error ?? move.error)

  return (
    <div className="space-y-3 border-t border-line pt-3">
      <ErrorBanner error={failure} />

      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          // A new block starts valid: the server rejects an empty reading body,
          // so seeding placeholder content keeps creation one click.
          const value = title.trim()
          if (!value) return
          // Clear on submit, restore on failure — see TopicPanel.
          setTitle('')
          create.mutate(
            { moduleId, title: value, type, content: defaultContent[type] },
            { onError: () => setTitle(value) },
          )
        }}
      >
        <div className="flex-1">
          <Field
            label="Block title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="What is an atom?"
          />
        </div>
        <div className="space-y-1">
          <label htmlFor={typeId} className="block text-sm font-medium">
            Block type
          </label>
          <select
            id={typeId}
            value={type}
            onChange={(e) => setType(e.target.value as BlockType)}
            className="rounded-md border border-line px-2 py-1.5 text-sm"
          >
            {BLOCK_TYPES.map((t) => (
              <option key={t} value={t}>
                {typeLabels[t]}
              </option>
            ))}
          </select>
        </div>
        <Button type="submit" disabled={create.isPending}>
          Add block
        </Button>
      </form>

      {blocks.length === 0 && <p className="text-xs text-muted">No blocks in this module yet.</p>}

      <ol className="space-y-2">
        {blocks.map((b, index) => (
          <li key={b.id} className="rounded-md border border-line p-2">
            <div className="flex items-center gap-2">
              <span className="flex-1 text-sm font-medium">{b.title}</span>
              <span className="text-xs uppercase tracking-wide text-muted">
                {typeLabels[b.type as BlockType] ?? b.type}
                {b.type === 'quiz' && ` · ${b.question_count ?? 0} questions`}
              </span>
              <Button
                variant="ghost"
                aria-label={`Move ${b.title} up`}
                disabled={index === 0}
                onClick={() => reorder(b.id, -1)}
              >
                ↑
              </Button>
              <Button
                variant="ghost"
                aria-label={`Move ${b.title} down`}
                disabled={index === blocks.length - 1}
                onClick={() => reorder(b.id, 1)}
              >
                ↓
              </Button>
              <Button
                variant="ghost"
                aria-label={`Edit ${b.title}`}
                onClick={() => setOpenId(openId === b.id ? null : b.id)}
              >
                {openId === b.id ? 'Close' : 'Edit'}
              </Button>
              <Button variant="ghost" aria-label={`Archive ${b.title}`} onClick={() => archive.mutate(b.id)}>
                ×
              </Button>
            </div>

            {openId === b.id && <BlockEditor courseId={courseId} block={b} topics={topics} />}
          </li>
        ))}
      </ol>
    </div>
  )
}
