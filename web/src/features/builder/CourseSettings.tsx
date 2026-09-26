import { useState } from 'react'
import { useUpdateCourse, type CoursePatch } from '../../api/hooks'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { ErrorBanner } from '../../components/ErrorBanner'
import { fieldError, type ApiError } from '../../api/errors'

type Props = {
  courseId: string
  title: string
  description: string
  visibility: string
  petEnabled: boolean
}

export function CourseSettings({ courseId, title, description, visibility, petEnabled }: Props) {
  const update = useUpdateCourse(courseId)
  const [draft, setDraft] = useState<CoursePatch>({ title, description })
  const error = (update.error as ApiError | null) ?? null

  return (
    <Card className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Course settings</h2>

      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault()
          update.mutate(draft)
        }}
      >
        <Field
          label="Title"
          value={draft.title ?? ''}
          onChange={(e) => setDraft({ ...draft, title: e.target.value })}
          error={error ? fieldError(error, 'title') : undefined}
        />
        <Field
          label="Description"
          value={draft.description ?? ''}
          onChange={(e) => setDraft({ ...draft, description: e.target.value })}
        />
        <Button type="submit" disabled={update.isPending}>
          Save
        </Button>
      </form>

      <div className="flex items-center justify-between border-t border-line pt-3">
        <div>
          <p className="text-sm font-medium">Study pet</p>
          <p className="text-xs text-muted">
            Opt in per course. The pet arrives with a later slice; this only records the choice.
          </p>
        </div>
        <Button
          variant="secondary"
          onClick={() => update.mutate({ pet_enabled: !petEnabled })}
          disabled={update.isPending}
        >
          {petEnabled ? 'On' : 'Off'}
        </Button>
      </div>

      <div className="flex items-center justify-between border-t border-line pt-3">
        <div>
          <p className="text-sm font-medium">Visibility</p>
          <p className="text-xs text-muted">
            Recorded but not enforced in this slice — access control arrives with slice 5.
          </p>
        </div>
        <select
          aria-label="Visibility"
          value={visibility}
          onChange={(e) => update.mutate({ visibility: e.target.value as CoursePatch['visibility'] })}
          className="rounded-md border border-line px-2 py-1 text-sm"
        >
          <option value="private">private</option>
          <option value="unlisted">unlisted</option>
          <option value="public">public</option>
        </select>
      </div>

      {error && !fieldError(error, 'title') && <ErrorBanner error={error} />}
    </Card>
  )
}
