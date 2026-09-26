import { useId, useState } from 'react'
import { parseBlockContent, type BlockType } from './schemas'
import { Field } from '../components/Field'
import { Button } from '../components/Button'

export type EditorProps = {
  content: unknown
  onSave: (content: unknown) => void
  pending: boolean
}

/** Accepts a bare id, a watch URL, a youtu.be link or an embed URL. */
export function extractVideoId(input: string): string {
  const trimmed = input.trim()
  const patterns = [/[?&]v=([\w-]{11})/, /youtu\.be\/([\w-]{11})/, /\/embed\/([\w-]{11})/]
  for (const pattern of patterns) {
    const match = trimmed.match(pattern)
    if (match) return match[1]
  }
  return trimmed
}

function ReadingEditor({ content, onSave, pending }: EditorProps) {
  const parsed = parseBlockContent('reading', content)
  const [body, setBody] = useState(parsed.ok && 'body_md' in parsed.data ? parsed.data.body_md : '')
  const bodyId = useId()

  return (
    <div className="space-y-2">
      <label htmlFor={bodyId} className="block text-sm font-medium">
        Body (Markdown)
      </label>
      <textarea
        id={bodyId}
        rows={6}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        className="w-full rounded-md border border-line px-2.5 py-1.5 font-mono text-sm"
      />
      <Button onClick={() => onSave({ body_md: body })} disabled={pending}>
        Save reading
      </Button>
    </div>
  )
}

function VideoEditor({ content, onSave, pending }: EditorProps) {
  const parsed = parseBlockContent('video_youtube', content)
  const [videoId, setVideoId] = useState(
    parsed.ok && 'video_id' in parsed.data ? parsed.data.video_id : '',
  )

  return (
    <div className="space-y-2">
      <Field
        label="YouTube video id"
        value={videoId}
        onChange={(e) => setVideoId(extractVideoId(e.target.value))}
        placeholder="dQw4w9WgXcQ"
      />
      <p className="text-xs text-muted">Paste a full YouTube URL and the id is extracted for you.</p>
      <Button onClick={() => onSave({ video_id: videoId })} disabled={pending}>
        Save video
      </Button>
    </div>
  )
}

function QuizEditor() {
  return (
    <p className="text-sm text-muted">
      A quiz block holds no content of its own — add questions below.
    </p>
  )
}

const editors: Record<BlockType, (props: EditorProps) => React.JSX.Element> = {
  reading: ReadingEditor,
  video_youtube: VideoEditor,
  quiz: QuizEditor,
}

export function editorFor(type: string) {
  return editors[type as BlockType]
}
