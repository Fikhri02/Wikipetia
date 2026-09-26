export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <p role="status" className="text-sm text-muted">
      {label}…
    </p>
  )
}
