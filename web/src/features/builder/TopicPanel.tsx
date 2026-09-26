import { useState } from 'react'
import { useArchiveTopic, useCreateTopic } from '../../api/hooks'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'

export function TopicPanel({
  courseId,
  topics,
}: {
  courseId: string
  topics: { id: string; title: string }[]
}) {
  const create = useCreateTopic(courseId)
  const archive = useArchiveTopic(courseId)
  const [title, setTitle] = useState('')

  return (
    <Card className="space-y-3">
      <div>
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Topics</h2>
        <p className="mt-1 text-xs text-muted">
          Mastery is measured per topic. Tag blocks and questions so progress means something.
        </p>
      </div>

      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate(title, { onSuccess: () => setTitle('') })
        }}
      >
        <div className="flex-1">
          <Field label="New topic" value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>
        <Button type="submit" disabled={create.isPending}>
          Add
        </Button>
      </form>

      <ul className="space-y-1">
        {topics.map((t) => (
          <li key={t.id} className="flex items-center justify-between text-sm">
            <span>{t.title}</span>
            <Button variant="ghost" aria-label={`Archive ${t.title}`} onClick={() => archive.mutate(t.id)}>
              ×
            </Button>
          </li>
        ))}
      </ul>
    </Card>
  )
}
