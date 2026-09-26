import { useState } from 'react'
import { useSubmitQuiz, type SubmittedAnswer } from '../api/hooks'
import { parseQuestionPrompt, parseQuizCompletion } from './schemas'
import { Button } from '../components/Button'
import { ErrorBanner } from '../components/ErrorBanner'
import type { PlayerBlockProps } from './players'
import { asApiError } from '../api/errors'

type Draft = Record<string, SubmittedAnswer>

export function QuizBlock({ enrolmentId, block }: PlayerBlockProps) {
  const submit = useSubmitQuiz(enrolmentId)
  // Local until submit: the server never hears about a half-finished attempt.
  const [draft, setDraft] = useState<Draft>({})

  const questions = block.questions ?? []
  const threshold = parseQuizCompletion(block.completion).threshold
  const result = submit.data
  const resultById = new Map((result?.results ?? []).map((r) => [r.question_id, r.correct]))

  function setAnswer(questionId: string, answer: Omit<SubmittedAnswer, 'question_id'>) {
    setDraft((d) => ({ ...d, [questionId]: { question_id: questionId, ...answer } }))
  }

  return (
    <article className="space-y-5">
      <h2 className="text-lg font-semibold">{block.title}</h2>

      {questions.length === 0 && <p className="text-sm text-muted">This quiz has no questions yet.</p>}

      <ol className="space-y-5">
        {questions.map((q, index) => {
          const parsed = parseQuestionPrompt(q.kind, q.prompt)
          if (!parsed.ok) {
            return (
              <li key={q.id} className="text-sm text-red-700">
                Question {index + 1} could not be displayed.
              </li>
            )
          }

          const outcome = resultById.get(q.id)

          return (
            <li key={q.id} className="space-y-2">
              <p className="text-sm font-medium">{parsed.data.text}</p>

              {q.kind === 'mcq_single' && 'options' in parsed.data && (
                <div className="space-y-1">
                  {parsed.data.options.map((option, optionIndex) => (
                    <label key={option} className="flex items-center gap-2 text-sm">
                      <input
                        type="radio"
                        name={q.id}
                        aria-label={option}
                        checked={draft[q.id]?.choice_index === optionIndex}
                        onChange={() => setAnswer(q.id, { choice_index: optionIndex })}
                      />
                      {option}
                    </label>
                  ))}
                </div>
              )}

              {q.kind === 'true_false' && (
                <div className="flex gap-4">
                  {[true, false].map((value) => (
                    <label key={String(value)} className="flex items-center gap-2 text-sm">
                      <input
                        type="radio"
                        name={q.id}
                        aria-label={value ? 'True' : 'False'}
                        checked={draft[q.id]?.boolean === value}
                        onChange={() => setAnswer(q.id, { boolean: value })}
                      />
                      {value ? 'True' : 'False'}
                    </label>
                  ))}
                </div>
              )}

              {q.kind === 'typed' && (
                <input
                  aria-label={parsed.data.text}
                  value={draft[q.id]?.text ?? ''}
                  onChange={(e) => setAnswer(q.id, { text: e.target.value })}
                  className="w-full rounded-md border border-line px-2.5 py-1.5 text-sm"
                />
              )}

              {outcome !== undefined && (
                <p className={`text-xs ${outcome ? 'text-emerald-700' : 'text-red-700'}`}>
                  {outcome ? 'Correct' : 'Incorrect'}
                </p>
              )}
            </li>
          )
        })}
      </ol>

      {questions.length > 0 && (
        <Button
          onClick={() =>
            submit.mutate({
              blockId: block.id,
              // Only what was actually answered; the server counts the rest wrong.
              answers: questions.map((q) => draft[q.id]).filter(Boolean),
            })
          }
          disabled={submit.isPending}
        >
          Submit answers
        </Button>
      )}

      {result && (
        <div className="rounded-md border border-line p-3 text-sm">
          <p className="font-medium">You scored {result.score}%</p>
          <p className="mt-0.5 text-muted">
            {result.score >= threshold
              ? 'That clears the bar for this block.'
              : `That is below the ${threshold}% needed to complete this block — try again.`}
          </p>
          {(result.mastery ?? []).length > 0 && (
            <p className="mt-2 text-xs text-muted">
              Mastery updated: {(result.mastery ?? []).map((m) => m.level).join(', ')}
            </p>
          )}
        </div>
      )}

      <ErrorBanner error={asApiError(submit.error)} />
    </article>
  )
}
