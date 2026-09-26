import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useCreateCourse, useMyCourses } from '../../api/hooks'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Field } from '../../components/Field'
import { Spinner } from '../../components/Spinner'
import { fieldError, type ApiError } from '../../api/errors'

export function CourseListPage() {
  const courses = useMyCourses()
  const create = useCreateCourse()
  const [title, setTitle] = useState('')

  const createError = (create.error as ApiError | null) ?? null

  return (
    <div className="space-y-8">
      <section className="space-y-3">
        <h1 className="text-xl font-semibold tracking-tight">Build</h1>

        <form
          className="flex items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate(title, { onSuccess: () => setTitle('') })
          }}
        >
          <div className="flex-1">
            <Field
              label="Course title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Form 4 Chemistry"
              error={createError ? fieldError(createError, 'title') : undefined}
            />
          </div>
          <Button type="submit" disabled={create.isPending}>
            Create course
          </Button>
        </form>

        {createError && !fieldError(createError, 'title') && <ErrorBanner error={createError} />}
      </section>

      <section className="space-y-2">
        <ErrorBanner error={(courses.error as ApiError | null) ?? null} />
        {courses.isLoading && <Spinner />}

        {!courses.isLoading && (courses.data ?? []).length === 0 && (
          <EmptyState title="No courses yet" hint="Create one above to start building." />
        )}

        <ul className="space-y-2">
          {(courses.data ?? []).map((c) => (
            <li key={c.id}>
              <Link to="/build/$courseId" params={{ courseId: c.id }}>
                <Card className="flex items-center justify-between hover:border-ink/30">
                  <div>
                    <p className="font-medium">{c.title}</p>
                    {c.description && <p className="text-xs text-muted">{c.description}</p>}
                  </div>
                  <span className="text-xs uppercase tracking-wide text-muted">{c.visibility}</span>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
