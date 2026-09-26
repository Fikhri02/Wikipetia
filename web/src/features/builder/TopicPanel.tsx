import { useState } from 'react'
import { useArchiveTopic, useCreateTopic } from '../../api/hooks'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Field } from '../../components/Field'
import { asApiError } from '../../api/errors'

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
          const value = title.trim()
          if (!value) return
          // Clear on SUBMIT, not on success. Clearing in onSuccess wipes text
          // the user has already typed for the next item, and then submits an
          // empty title the server rejects. Restore it if the create fails.
          setTitle('')
          create.mutate(value, { onError: () => setTitle(value) })
        }}
      >
        <div className="flex-1">
          <Field label="New topic" value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>
        <Button type="submit" disabled={create.isPending}>
          Add
        </Button>
      </form>

      <ErrorBanner error={asApiError(create.error ?? archive.error)} />

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
