import { useId, useState } from 'react'
import { useArchiveQuestion, useCreateQuestion, useQuestions } from '../../api/hooks'
import { parseQuestionPrompt, QUESTION_KINDS, type QuestionKind } from '../../blocks/schemas'
import { Button } from '../../components/Button'
import { Field } from '../../components/Field'
import { ErrorBanner } from '../../components/ErrorBanner'
import { Spinner } from '../../components/Spinner'
import { asApiError, fieldError } from '../../api/errors'

const kindLabels: Record<QuestionKind, string> = {
  mcq_single: 'Multiple choice',
  true_false: 'True or false',
  typed: 'Typed answer',
}

export function QuestionEditor({
  courseId,
  blockId,
  topics,
}: {
  courseId: string
  blockId: string
  topics: { id: string; title: string }[]
}) {
  // Instance-scoped: a quiz block can be open in more than one module at once,
  // and hardcoded ids would make both labels point at the first select.
  const kindId = useId()
  const topicId_ = useId()
  const questions = useQuestions(blockId, true)
  const create = useCreateQuestion(blockId, courseId)
  const archive = useArchiveQuestion(blockId, courseId)

  const [kind, setKind] = useState<QuestionKind>('mcq_single')
  const [text, setText] = useState('')
  const [topicId, setTopicId] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [correctIndex, setCorrectIndex] = useState(0)
  const [boolAnswer, setBoolAnswer] = useState(true)
  const [accepted, setAccepted] = useState('')

  const error = asApiError(create.error)

  function reset() {
    setText('')
    setOptions(['', ''])
    setCorrectIndex(0)
    setAccepted('')
  }

  function submit() {
    const prompt =
      kind === 'mcq_single' ? { text, options: options.map((o) => o.trim()) } : { text }

    const answer =
      kind === 'mcq_single'
        ? { correct_index: correctIndex }
        : kind === 'true_false'
          ? { correct: boolAnswer }
          : {
              accepted: accepted
                .split(',')
                .map((a) => a.trim())
                .filter(Boolean),
              case_sensitive: false,
            }

    create.mutate({ kind, topic_id: topicId || undefined, prompt, answer }, { onSuccess: reset })
  }

  return (
    <div className="space-y-4 border-t border-line pt-3">
      <p className="text-sm font-medium">Questions</p>

      <div className="space-y-3 rounded-md bg-slate-50 p-3">
        <div className="space-y-1">
          <label htmlFor={kindId} className="block text-sm font-medium">
            Kind
          </label>
          <select
            id={kindId}
            value={kind}
            onChange={(e) => setKind(e.target.value as QuestionKind)}
            className="rounded-md border border-line px-2 py-1.5 text-sm"
          >
            {QUESTION_KINDS.map((k) => (
              <option key={k} value={k}>
                {kindLabels[k]}
              </option>
            ))}
          </select>
        </div>

        <Field
          label="Question"
          value={text}
          onChange={(e) => setText(e.target.value)}
          error={error ? fieldError(error, 'prompt.text') : undefined}
        />

        {kind === 'mcq_single' && (
          <div className="space-y-2">
            {options.map((option, index) => (
              <div key={index} className="flex items-end gap-2">
                <div className="flex-1">
                  <Field
                    label={`Option ${index + 1}`}
                    value={option}
                    onChange={(e) => {
                      const next = options.slice()
                      next[index] = e.target.value
                      setOptions(next)
                    }}
                  />
                </div>
                <label className="flex items-center gap-1.5 pb-2 text-xs text-muted">
                  <input
                    type="radio"
                    name="correct-option"
                    aria-label={`Option ${index + 1} is correct`}
                    checked={correctIndex === index}
                    onChange={() => setCorrectIndex(index)}
                  />
                  correct
                </label>
              </div>
            ))}
            <Button
              variant="secondary"
              type="button"
              onClick={() => setOptions([...options, ''])}
              disabled={options.length >= 6}
            >
              Add option
            </Button>
          </div>
        )}

        {kind === 'true_false' && (
          <div className="flex gap-4">
            {[true, false].map((value) => (
              <label key={String(value)} className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  name="bool-answer"
                  aria-label={value ? 'Answer is true' : 'Answer is false'}
                  checked={boolAnswer === value}
                  onChange={() => setBoolAnswer(value)}
                />
                {value ? 'True' : 'False'}
              </label>
            ))}
          </div>
        )}

        {kind === 'typed' && (
          <Field
            label="Accepted answers (comma separated)"
            value={accepted}
            onChange={(e) => setAccepted(e.target.value)}
            placeholder="Fe, iron"
          />
        )}

        <div className="space-y-1">
          <label htmlFor={topicId_} className="block text-sm font-medium">
            Topic
          </label>
          <select
            id={topicId_}
            value={topicId}
            onChange={(e) => setTopicId(e.target.value)}
            className="rounded-md border border-line px-2 py-1.5 text-sm"
          >
            <option value="">No topic (counts for completion, not mastery)</option>
            {topics.map((t) => (
              <option key={t.id} value={t.id}>
                {t.title}
              </option>
            ))}
          </select>
        </div>

        <Button type="button" onClick={submit} disabled={create.isPending}>
          Add question
        </Button>

        {error && !fieldError(error, 'prompt.text') && <ErrorBanner error={error} />}
      </div>

      {questions.isLoading && <Spinner />}

      <ol className="space-y-1.5">
        {(questions.data ?? []).map((q, index) => {
          const parsed = parseQuestionPrompt(q.kind, q.prompt)
          return (
            <li key={q.id} className="flex items-center justify-between gap-3 text-sm">
              <span className={parsed.ok ? '' : 'text-red-700'}>
                {parsed.ok ? parsed.data.text : `Question ${index + 1} is malformed`}
              </span>
              <Button
                variant="ghost"
                aria-label={`Archive question ${index + 1}`}
                onClick={() => archive.mutate(q.id)}
              >
                ×
              </Button>
            </li>
          )
        })}
      </ol>
    </div>
  )
}
