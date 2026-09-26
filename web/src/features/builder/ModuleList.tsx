import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  qk,
  useArchiveModule,
  useCreateModule,
  useMoveModule,
  useRenameModule,
} from '../../api/hooks'
import { moveWithin } from './reorder'
import { BlockList } from './BlockList'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Field } from '../../components/Field'

type ModuleShape = { id: string; title: string; sort_key: string; blocks?: unknown }
type TopicShape = { id: string; title: string }

export function ModuleList({
  courseId,
  modules,
  topics,
}: {
  courseId: string
  modules: ModuleShape[]
  topics: TopicShape[]
}) {
  const qc = useQueryClient()
  const create = useCreateModule(courseId)
  const rename = useRenameModule(courseId)
  const archive = useArchiveModule(courseId)
  const move = useMoveModule(courseId)
  const [title, setTitle] = useState('')

  function reorder(id: string, direction: -1 | 1) {
    const { next, before, after } = moveWithin(modules, id, direction)
    if (!before && !after) return // already at the edge

    // Optimistic: rewrite the cached outline with the new module order.
    const current = qc.getQueryData(qk.outline(courseId)) as { modules?: ModuleShape[] } | undefined
    const optimistic = current ? { ...current, modules: next } : undefined

    move.mutate({ id, before, after, optimistic })
  }

  return (
    <section className="space-y-4">
      <form
        className="flex items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate(title, { onSuccess: () => setTitle('') })
        }}
      >
        <div className="flex-1">
          <Field
            label="New module"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Module 1 — Matter"
          />
        </div>
        <Button type="submit" disabled={create.isPending}>
          Add module
        </Button>
      </form>

      {modules.length === 0 && (
        <EmptyState title="No modules yet" hint="A course is a list of modules, each a list of blocks." />
      )}

      <ol className="space-y-4">
        {modules.map((m, index) => (
          <li key={m.id}>
            <Card className="space-y-3">
              <div className="flex items-center gap-2">
                <input
                  defaultValue={m.title}
                  aria-label={`Module title: ${m.title}`}
                  onBlur={(e) => {
                    if (e.target.value !== m.title) rename.mutate({ id: m.id, title: e.target.value })
                  }}
                  className="flex-1 rounded border border-transparent px-1 py-0.5 font-medium hover:border-line focus:border-line focus:outline-none"
                />
                <Button
                  variant="ghost"
                  aria-label={`Move ${m.title} up`}
                  disabled={index === 0}
                  onClick={() => reorder(m.id, -1)}
                >
                  ↑
                </Button>
                <Button
                  variant="ghost"
                  aria-label={`Move ${m.title} down`}
                  disabled={index === modules.length - 1}
                  onClick={() => reorder(m.id, 1)}
                >
                  ↓
                </Button>
                <Button
                  variant="ghost"
                  aria-label={`Archive ${m.title}`}
                  onClick={() => archive.mutate(m.id)}
                >
                  Archive
                </Button>
              </div>

              <BlockList courseId={courseId} moduleId={m.id} blocks={(m.blocks as never) ?? []} topics={topics} />
            </Card>
          </li>
        ))}
      </ol>
    </section>
  )
}
