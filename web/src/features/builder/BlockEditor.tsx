// Replaced in Task 7.
export function BlockEditor({ block }: {
  courseId: string
  block: { id: string; title: string; type: string; content: unknown; topic_ids?: string[] | null }
  topics: { id: string; title: string }[]
}) {
  return <p className="border-t border-line pt-3 text-xs text-muted">Editor for “{block.title}” arrives in Task 7.</p>
}
