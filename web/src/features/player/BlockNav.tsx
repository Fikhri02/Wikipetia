const marks: Record<string, string> = {
  not_started: '·',
  in_progress: '◐',
  completed: '●',
}

type NavModule = {
  id: string
  title: string
  blocks?: { id: string; title: string; status: string }[] | null
}

export function BlockNav({
  modules,
  selectedId,
  onSelect,
}: {
  modules: NavModule[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  return (
    <nav className="space-y-4">
      {modules.map((m) => (
        <div key={m.id}>
          <p className="text-xs font-semibold uppercase tracking-wide text-muted">{m.title}</p>
          <ul className="mt-1 space-y-0.5">
            {(m.blocks ?? []).map((b) => (
              <li key={b.id}>
                <button
                  type="button"
                  onClick={() => onSelect(b.id)}
                  aria-current={b.id === selectedId ? 'true' : undefined}
                  className={`flex w-full items-baseline gap-2 rounded px-1.5 py-1 text-left text-sm hover:bg-slate-50 ${
                    b.id === selectedId ? 'bg-slate-100 font-medium' : ''
                  }`}
                >
                  <span
                    aria-label={b.status.replace('_', ' ')}
                    className={b.status === 'completed' ? 'text-emerald-600' : 'text-muted'}
                  >
                    {marks[b.status] ?? '·'}
                  </span>
                  <span className="truncate">{b.title}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  )
}
