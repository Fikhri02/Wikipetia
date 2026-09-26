import { editorFor } from '../../blocks/editors'
import { useSetBlockTopics, useUpdateBlock } from '../../api/hooks'
import { QuestionEditor } from './QuestionEditor'
import { ErrorBanner } from '../../components/ErrorBanner'
import { asApiError } from '../../api/errors'

type Props = {
  courseId: string
  block: { id: string; title: string; type: string; content: unknown; topic_ids?: string[] | null }
  topics: { id: string; title: string }[]
}

export function BlockEditor({ courseId, block, topics }: Props) {
  const update = useUpdateBlock(courseId)
  const setTopics = useSetBlockTopics(courseId)
  const Editor = editorFor(block.type)

  const tagged = new Set(block.topic_ids ?? [])

  function toggleTopic(topicId: string) {
    const next = new Set(tagged)
    if (next.has(topicId)) next.delete(topicId)
    else next.add(topicId)
    // PUT semantics: the whole set is replaced, matching the API.
    setTopics.mutate({ blockId: block.id, topicIds: [...next] })
  }

  return (
    <div className="space-y-4 border-t border-line pt-3">
      {Editor ? (
        <Editor
          content={block.content}
          pending={update.isPending}
          onSave={(content) => update.mutate({ id: block.id, content })}
        />
      ) : (
        <p className="text-sm text-red-700">No editor for block type “{block.type}”.</p>
      )}

      <div>
        <p className="text-sm font-medium">Topics</p>
        <div className="mt-1 flex flex-wrap gap-1.5">
          {topics.length === 0 && <p className="text-xs text-muted">Add a topic first.</p>}
          {topics.map((t) => (
            <button
              key={t.id}
              type="button"
              aria-pressed={tagged.has(t.id)}
              onClick={() => toggleTopic(t.id)}
              className={`rounded-full border px-2 py-0.5 text-xs ${
                tagged.has(t.id) ? 'border-ink bg-ink text-white' : 'border-line text-muted hover:border-ink/40'
              }`}
            >
              {t.title}
            </button>
          ))}
        </div>
      </div>

      {block.type === 'quiz' && <QuestionEditor courseId={courseId} blockId={block.id} topics={topics} />}

      <ErrorBanner error={asApiError(update.error ?? setTopics.error)} />
    </div>
  )
}
