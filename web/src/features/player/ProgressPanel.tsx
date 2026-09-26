import { Card } from '../../components/Card'

type Topic = { topic_id: string; title: string; strength: number; level: string }

const levelStyles: Record<string, string> = {
  new: 'bg-slate-100 text-slate-600',
  learning: 'bg-amber-100 text-amber-800',
  proficient: 'bg-sky-100 text-sky-800',
  mastered: 'bg-emerald-100 text-emerald-800',
}

function percent(part: number, whole: number): number {
  if (whole <= 0) return 0
  return Math.round((part / whole) * 100)
}

function Bar({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span className="tabular-nums text-muted">{value}%</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuenow={value}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2 w-full overflow-hidden rounded-full bg-slate-100"
      >
        <div className={`h-full rounded-full ${tone}`} style={{ width: `${value}%` }} />
      </div>
    </div>
  )
}

export function ProgressPanel({
  blocksCompleted,
  blocksTotal,
  topics,
}: {
  blocksCompleted: number
  blocksTotal: number
  topics: Topic[]
}) {
  const completed = percent(blocksCompleted, blocksTotal)
  const masteredCount = topics.filter((t) => t.level === 'mastered').length
  const mastered = percent(masteredCount, topics.length)

  return (
    <Card className="space-y-5">
      <div className="space-y-3">
        <Bar label="Completed" value={completed} tone="bg-slate-400" />
        <Bar label="Mastered" value={mastered} tone="bg-emerald-500" />
        <p className="text-xs text-muted">
          Completed is what you opened; mastered is what you proved. The gap between them is the point.
        </p>
      </div>

      <div className="space-y-2 border-t border-line pt-4">
        <p className="text-sm font-medium">Topics</p>
        <ul className="space-y-1.5">
          {topics.map((t) => (
            <li key={t.topic_id} className="flex items-center justify-between gap-3">
              <span className="truncate text-sm">{t.title}</span>
              <span
                className={`shrink-0 rounded-full px-2 py-0.5 text-xs ${levelStyles[t.level] ?? levelStyles.new}`}
              >
                {t.level}
              </span>
            </li>
          ))}
          {topics.length === 0 && (
            <li className="text-xs text-muted">This course has no topics, so nothing to master yet.</li>
          )}
        </ul>
      </div>
    </Card>
  )
}
