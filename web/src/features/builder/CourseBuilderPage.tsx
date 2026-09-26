import { useParams } from '@tanstack/react-router'
import { useOutline } from '../../api/hooks'
import { CourseSettings } from './CourseSettings'
import { ModuleList } from './ModuleList'
import { TopicPanel } from './TopicPanel'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Spinner } from '../../components/Spinner'
import type { ApiError } from '../../api/errors'

export function CourseBuilderPage() {
  const { courseId } = useParams({ from: '/build/$courseId' })
  const outline = useOutline(courseId)

  if (outline.isLoading) return <Spinner />
  if (outline.error) return <ErrorBanner error={outline.error as ApiError} />
  if (!outline.data) return null

  const { course, topics, modules } = outline.data

  return (
    <div className="space-y-8">
      <header>
        <h1 className="text-xl font-semibold tracking-tight">{course.title}</h1>
        <p className="text-sm text-muted">
          {modules?.length ?? 0} modules · {topics?.length ?? 0} topics
        </p>
      </header>

      <div className="grid gap-8 lg:grid-cols-[2fr_1fr]">
        <ModuleList courseId={courseId} modules={modules ?? []} topics={topics ?? []} />
        <div className="space-y-6">
          <TopicPanel courseId={courseId} topics={topics ?? []} />
          <CourseSettings
            courseId={courseId}
            title={course.title}
            description={course.description}
            visibility={course.visibility}
            petEnabled={course.pet_enabled}
          />
        </div>
      </div>
    </div>
  )
}
