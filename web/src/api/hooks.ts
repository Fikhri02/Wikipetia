import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, list } from './client'
import { apiError, type ApiError } from './errors'

/** Query keys in one place, so an invalidation can never miss by a typo. */
export const qk = {
  enrolments: ['enrolments'] as const,
  player: (id: string) => ['enrolments', id] as const,
  courses: ['courses'] as const,
  course: (id: string) => ['courses', id] as const,
  outline: (id: string) => ['courses', id, 'outline'] as const,
  questions: (blockId: string) => ['blocks', blockId, 'questions'] as const,
}

/**
 * Throws a narrowed ApiError so every component handles one shape.
 *
 * The `response.ok` check is not belt-and-braces. A dead API behind the Vite
 * dev proxy answers 500 with a plain-text body, so openapi-fetch leaves `error`
 * undefined — and returning `undefined` then made a down server render as an
 * EMPTY library, telling the learner they had no courses. Exported for tests.
 */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error !== undefined) throw apiError(result.error)
  if (!result.response.ok) {
    throw apiError(
      new Error(`server returned ${result.response.status} ${result.response.statusText}`.trim()),
    )
  }
  if (result.data === undefined) {
    throw apiError(new Error('the server returned an empty response body'))
  }
  return result.data
}

/**
 * For the 204 No Content endpoints (archives and moves). These cannot use
 * `unwrap`, which requires a body — but they must not use a bare
 * `error !== undefined` check either: openapi-fetch leaves `error` undefined for
 * ANY response with no body, non-ok included, so a 502 from a gateway was
 * indistinguishable from a successful 204. An archive that failed reported
 * success, and a failed move never rolled its optimistic update back.
 */
export function expectNoContent(result: { error?: unknown; response: Response }): void {
  if (result.error !== undefined) throw apiError(result.error)
  if (!result.response.ok) {
    throw apiError(
      new Error(`server returned ${result.response.status} ${result.response.statusText}`.trim()),
    )
  }
}

export type { ApiError }

export function useEnrolments() {
  return useQuery({
    queryKey: qk.enrolments,
    queryFn: async () => {
      const res = await api.GET('/enrolments', {})
      return list(unwrap(res).enrolments)
    },
  })
}

export function useMyCourses() {
  return useQuery({
    queryKey: qk.courses,
    queryFn: async () => {
      const res = await api.GET('/courses', {})
      return list(unwrap(res).courses)
    },
  })
}

export function useOutline(courseId: string) {
  return useQuery({
    queryKey: qk.outline(courseId),
    queryFn: async () => {
      const res = await api.GET('/courses/{id}/outline', { params: { path: { id: courseId } } })
      return unwrap(res)
    },
  })
}

export function usePlayer(enrolmentId: string) {
  return useQuery({
    queryKey: qk.player(enrolmentId),
    queryFn: async () => {
      const res = await api.GET('/enrolments/{id}', { params: { path: { id: enrolmentId } } })
      return unwrap(res)
    },
  })
}

export function useEnrol() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (courseId: string) => {
      const res = await api.POST('/enrolments', {
        body: { course_id: courseId, mode: 'self_study' },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.enrolments }),
  })
}

export function useCreateCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (title: string) => {
      const res = await api.POST('/courses', { body: { title } })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.courses }),
  })
}

export type CoursePatch = {
  title?: string
  description?: string
  visibility?: 'public' | 'unlisted' | 'private'
  pet_enabled?: boolean
}

export function useUpdateCourse(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (patch: CoursePatch) => {
      const res = await api.PATCH('/courses/{id}', {
        params: { path: { id: courseId } },
        body: patch,
      })
      return unwrap(res)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.courses })
      qc.invalidateQueries({ queryKey: qk.outline(courseId) })
    },
  })
}

export function useArchiveCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (courseId: string) => {
      const res = await api.POST('/courses/{id}/archive', {
        params: { path: { id: courseId } },
      })
      expectNoContent(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.courses }),
  })
}

export function useCreateModule(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (title: string) => {
      const res = await api.POST('/courses/{id}/modules', {
        params: { path: { id: courseId } },
        body: { title },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useRenameModule(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, title }: { id: string; title: string }) => {
      const res = await api.PATCH('/modules/{id}', {
        params: { path: { id } },
        body: { title },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useArchiveModule(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await api.POST('/modules/{id}/archive', { params: { path: { id } } })
      expectNoContent(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

/**
 * Optimistic: the list reorders under the cursor, and rolls back if the server
 * disagrees. One of only two optimistic mutations in the app.
 */
export function useMoveModule(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      id: string
      before?: string
      after?: string
      /** The reordered outline to show while the request is in flight. */
      optimistic?: unknown
    }) => {
      const res = await api.POST('/modules/{id}/move', {
        params: { path: { id: vars.id } },
        body: { before: vars.before, after: vars.after },
      })
      expectNoContent(res)
    },
    onMutate: async (vars) => {
      await qc.cancelQueries({ queryKey: qk.outline(courseId) })
      const previous = qc.getQueryData(qk.outline(courseId))
      if (vars.optimistic) {
        qc.setQueryData(qk.outline(courseId), vars.optimistic)
      }
      return { previous }
    },
    onError: (_err, _vars, context) => {
      // Put the server's truth back; the UI briefly showed a lie.
      if (context?.previous) qc.setQueryData(qk.outline(courseId), context.previous)
    },
    onSettled: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useCreateBlock(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      moduleId: string
      title: string
      type: 'reading' | 'video_youtube' | 'quiz'
      content?: unknown
    }) => {
      const res = await api.POST('/modules/{id}/blocks', {
        params: { path: { id: vars.moduleId } },
        body: { title: vars.title, type: vars.type, content: vars.content },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useUpdateBlock(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      id: string
      title?: string
      content?: unknown
      completion?: unknown
    }) => {
      const res = await api.PATCH('/blocks/{id}', {
        params: { path: { id: vars.id } },
        body: { title: vars.title, content: vars.content, completion: vars.completion },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useArchiveBlock(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await api.POST('/blocks/{id}/archive', { params: { path: { id } } })
      expectNoContent(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useMoveBlock(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { id: string; before?: string; after?: string }) => {
      const res = await api.POST('/blocks/{id}/move', {
        params: { path: { id: vars.id } },
        body: { before: vars.before, after: vars.after },
      })
      expectNoContent(res)
    },
    onSettled: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useCreateTopic(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (title: string) => {
      const res = await api.POST('/courses/{id}/topics', {
        params: { path: { id: courseId } },
        body: { title },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useArchiveTopic(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await api.POST('/topics/{id}/archive', { params: { path: { id } } })
      expectNoContent(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useSetBlockTopics(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { blockId: string; topicIds: string[] }) => {
      const res = await api.PUT('/blocks/{id}/topics', {
        params: { path: { id: vars.blockId } },
        body: { topic_ids: vars.topicIds },
      })
      return unwrap(res)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.outline(courseId) }),
  })
}

export function useQuestions(blockId: string, enabled: boolean) {
  return useQuery({
    queryKey: qk.questions(blockId),
    enabled,
    queryFn: async () => {
      const res = await api.GET('/blocks/{id}/questions', { params: { path: { id: blockId } } })
      return list(unwrap(res).questions)
    },
  })
}

export function useCreateQuestion(blockId: string, courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      kind: 'mcq_single' | 'true_false' | 'typed'
      topic_id?: string
      prompt: unknown
      answer: unknown
    }) => {
      const res = await api.POST('/blocks/{id}/questions', {
        params: { path: { id: blockId } },
        body: vars,
      })
      return unwrap(res)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.questions(blockId) })
      // question_count lives on the outline.
      qc.invalidateQueries({ queryKey: qk.outline(courseId) })
    },
  })
}

export function useArchiveQuestion(blockId: string, courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await api.POST('/questions/{id}/archive', { params: { path: { id } } })
      expectNoContent(res)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.questions(blockId) })
      qc.invalidateQueries({ queryKey: qk.outline(courseId) })
    },
  })
}

export function useCompleteBlock(enrolmentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      blockId: string
      marked_done?: boolean
      watched_to_end?: boolean
    }) => {
      const res = await api.POST('/enrolments/{enrolment_id}/blocks/{block_id}/complete', {
        params: { path: { enrolment_id: enrolmentId, block_id: vars.blockId } },
        body: { marked_done: vars.marked_done, watched_to_end: vars.watched_to_end },
      })
      return unwrap(res)
    },
    // Optimistic: marking a reading done should feel instantaneous.
    onMutate: async (vars) => {
      await qc.cancelQueries({ queryKey: qk.player(enrolmentId) })
      const previous = qc.getQueryData(qk.player(enrolmentId))

      qc.setQueryData(qk.player(enrolmentId), (old: unknown) => {
        if (!old || typeof old !== 'object') return old
        const view = old as {
          blocks_completed: number
          modules?: { blocks?: { id: string; status: string }[] | null }[] | null
        }
        let newlyCompleted = 0
        const modules = (view.modules ?? []).map((m) => ({
          ...m,
          blocks: (m.blocks ?? []).map((b) => {
            if (b.id !== vars.blockId || b.status === 'completed') return b
            newlyCompleted += 1
            return { ...b, status: 'completed' }
          }),
        }))
        return { ...view, modules, blocks_completed: view.blocks_completed + newlyCompleted }
      })

      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) qc.setQueryData(qk.player(enrolmentId), context.previous)
    },
    // The server owns mastery and the real completion time; always reconcile.
    onSettled: () => qc.invalidateQueries({ queryKey: qk.player(enrolmentId) }),
  })
}

export type SubmittedAnswer = {
  question_id: string
  choice_index?: number
  boolean?: boolean
  text?: string
}

export function useSubmitQuiz(enrolmentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { blockId: string; answers: SubmittedAnswer[] }) => {
      const res = await api.POST('/enrolments/{enrolment_id}/blocks/{block_id}/submit', {
        params: { path: { enrolment_id: enrolmentId, block_id: vars.blockId } },
        body: { answers: vars.answers },
      })
      return unwrap(res)
    },
    // Not optimistic: only the server knows the score, and guessing it would be
    // the one lie a learner would actually notice.
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.player(enrolmentId) }),
  })
}
