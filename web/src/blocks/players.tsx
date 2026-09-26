import type { BlockType } from './schemas'
import { ReadingBlock } from './ReadingBlock'
import { VideoBlock } from './VideoBlock'
import { QuizBlock } from './QuizBlock'

export type PlayerBlockData = {
  id: string
  title: string
  type: string
  content: unknown
  completion: unknown
  topic_ids?: string[] | null
  status: string
  score?: number
  attempts: number
  questions?: { id: string; kind: string; prompt: unknown }[] | null
}

export type CompleteSignal = { marked_done?: boolean; watched_to_end?: boolean }

export type PlayerBlockProps = {
  enrolmentId: string
  block: PlayerBlockData
  onComplete: (signal: CompleteSignal) => void
  pending: boolean
}

/**
 * The registry. Adding a block type is: one entry here, one editor entry, one
 * zod schema. Nothing else in the app changes — no switch in the player, no
 * branch in progress.
 */
const players: Record<BlockType, (props: PlayerBlockProps) => React.JSX.Element> = {
  reading: ReadingBlock,
  video_youtube: VideoBlock,
  quiz: QuizBlock,
}

export function playerFor(type: string) {
  return players[type as BlockType]
}
