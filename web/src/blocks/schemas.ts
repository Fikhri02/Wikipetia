import { z } from 'zod'

/**
 * These mirror api/internal/course/blockspec.go. The OpenAPI contract types
 * blocks.content, blocks.completion and questions.prompt as `unknown`, because
 * they are polymorphic jsonb on the server. Parsing them here is what restores
 * safety — and a mismatch with the Go validator shows up as a parse failure in
 * the UI rather than as a silently blank block.
 */

export const BLOCK_TYPES = ['reading', 'video_youtube', 'quiz'] as const
export type BlockType = (typeof BLOCK_TYPES)[number]

export const QUESTION_KINDS = ['mcq_single', 'true_false', 'typed'] as const
export type QuestionKind = (typeof QUESTION_KINDS)[number]

// ---- content ----

export const readingContent = z.object({ body_md: z.string().min(1) })
export const videoContent = z.object({ video_id: z.string().min(1) })
export const quizContent = z.object({})

export type ReadingContent = z.infer<typeof readingContent>
export type VideoContent = z.infer<typeof videoContent>

// ---- completion ----

export const markDoneCompletion = z.object({ kind: z.literal('mark_done') })
export const watchedCompletion = z.object({ kind: z.literal('watched_to_end') })
export const quizCompletion = z.object({
  kind: z.literal('score_threshold'),
  threshold: z.number().int().min(0).max(100),
})

export type QuizCompletion = z.infer<typeof quizCompletion>

// ---- question prompts ----

export const mcqPrompt = z.object({
  text: z.string().min(1),
  options: z.array(z.string()).min(2),
})
export const textPrompt = z.object({ text: z.string().min(1) })

export type McqPrompt = z.infer<typeof mcqPrompt>
export type TextPrompt = z.infer<typeof textPrompt>

// ---- parse helpers ----

export type ParseResult<T> = { ok: true; data: T } | { ok: false; message: string }

function run<T>(schema: z.ZodType<T>, value: unknown): ParseResult<T> {
  const parsed = schema.safeParse(value)
  if (parsed.success) return { ok: true, data: parsed.data }
  const first = parsed.error.issues[0]
  const path = first?.path.join('.')
  return { ok: false, message: path ? `${path}: ${first.message}` : (first?.message ?? 'invalid') }
}

const contentSchemas = {
  reading: readingContent,
  video_youtube: videoContent,
  quiz: quizContent,
} as const

export function parseBlockContent(
  type: string,
  value: unknown,
): ParseResult<ReadingContent | VideoContent | Record<string, never>> {
  const schema = contentSchemas[type as BlockType]
  if (!schema) return { ok: false, message: `unknown block type "${type}"` }
  return run(schema, value)
}

const promptSchemas = {
  mcq_single: mcqPrompt,
  true_false: textPrompt,
  typed: textPrompt,
} as const

export function parseQuestionPrompt(
  kind: string,
  value: unknown,
): ParseResult<McqPrompt | TextPrompt> {
  const schema = promptSchemas[kind as QuestionKind]
  if (!schema) return { ok: false, message: `unknown question kind "${kind}"` }
  return run(schema, value)
}

export function parseQuizCompletion(value: unknown): QuizCompletion {
  const parsed = quizCompletion.safeParse(value)
  // A quiz block always has a completion rule; the server defaults it to 70.
  return parsed.success ? parsed.data : { kind: 'score_threshold', threshold: 70 }
}
