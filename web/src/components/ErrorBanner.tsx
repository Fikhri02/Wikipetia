import type { ApiError } from '../api/errors'

export function ErrorBanner({ error }: { error: ApiError | null }) {
  if (!error) return null
  return (
    <div role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">
      <p className="font-medium">{error.title}</p>
      {error.detail && <p className="mt-0.5">{error.detail}</p>}
    </div>
  )
}
