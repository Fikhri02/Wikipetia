// Replaced in Task 9.
export function BlockSurface({ block }: { enrolmentId: string; block: { title: string; type: string } }) {
  return (
    <article className="space-y-2">
      <h2 className="text-lg font-semibold">{block.title}</h2>
      <p className="text-sm text-muted">The {block.type} surface arrives in Task 9.</p>
    </article>
  )
}
