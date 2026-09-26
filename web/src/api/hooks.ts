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

/** Throws a narrowed ApiError so every component handles one shape. */
function unwrap<T>(result: { data?: T; error?: unknown }): T {
  if (result.error !== undefined) throw apiError(result.error)
  return result.data as T
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
