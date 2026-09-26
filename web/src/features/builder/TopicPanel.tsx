// Replaced in Task 7.
export function TopicPanel({ topics }: { courseId: string; topics: { id: string; title: string }[] }) {
  return <p className="text-xs text-muted">{topics.length} topics</p>
}
