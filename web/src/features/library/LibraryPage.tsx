import { Link } from '@tanstack/react-router'
import { useEnrol, useEnrolments, useMyCourses } from '../../api/hooks'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Spinner } from '../../components/Spinner'
import type { ApiError } from '../../api/errors'

export function LibraryPage() {
  const enrolments = useEnrolments()
  const courses = useMyCourses()
  const enrol = useEnrol()

  const enrolledCourseIds = new Set((enrolments.data ?? []).map((e) => e.course_id))
  const notStarted = (courses.data ?? []).filter((c) => !enrolledCourseIds.has(c.id))

  return (
    <div className="space-y-8">
      <section className="space-y-3">
        <h1 className="text-xl font-semibold tracking-tight">Library</h1>

        <ErrorBanner error={(enrolments.error as ApiError | null) ?? null} />

        {enrolments.isLoading && <Spinner />}

        {!enrolments.isLoading && (enrolments.data ?? []).length === 0 && (
          <EmptyState
            title="Nothing here yet"
            hint="Enrol in a course below, or build one first."
            action={
              <Link to="/build">
                <Button variant="secondary">Go to the builder</Button>
              </Link>
            }
          />
        )}

        <ul className="space-y-2">
          {(enrolments.data ?? []).map((e) => (
            <li key={e.id}>
              <Link to="/learn/$enrolmentId" params={{ enrolmentId: e.id }}>
                <Card className="hover:border-ink/30">
                  <p className="font-medium">{e.course_title}</p>
                  <p className="text-xs text-muted">
                    Started {new Date(e.started_at).toLocaleDateString()} · {e.mode.replace('_', ' ')}
                  </p>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      {notStarted.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">
            Your courses, not yet started
          </h2>
          <ErrorBanner error={(enrol.error as ApiError | null) ?? null} />
          <ul className="space-y-2">
            {notStarted.map((c) => (
              <li key={c.id}>
                <Card className="flex items-center justify-between">
                  <span className="font-medium">{c.title}</span>
                  <Button onClick={() => enrol.mutate(c.id)} disabled={enrol.isPending}>
                    Start {c.title}
                  </Button>
                </Card>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}
