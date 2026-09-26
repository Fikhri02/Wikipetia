import { describe, it, expect } from 'vitest'
import { parseBlockContent, parseQuestionPrompt, quizCompletion } from './schemas'

describe('parseBlockContent', () => {
  it('accepts a valid reading block', () => {
    const result = parseBlockContent('reading', { body_md: '# Atoms' })
    expect(result.ok).toBe(true)
    if (result.ok) expect(result.data).toEqual({ body_md: '# Atoms' })
  })

  it('rejects a reading block with an empty body', () => {
    const result = parseBlockContent('reading', { body_md: '' })
    expect(result.ok).toBe(false)
  })

  it('accepts a valid video block', () => {
    const result = parseBlockContent('video_youtube', { video_id: 'dQw4w9WgXcQ' })
    expect(result.ok).toBe(true)
  })

  it('rejects a video block with no id', () => {
    expect(parseBlockContent('video_youtube', {}).ok).toBe(false)
  })

  it('accepts an empty quiz content object', () => {
    expect(parseBlockContent('quiz', {}).ok).toBe(true)
  })

  it('rejects an unknown block type rather than guessing', () => {
    const result = parseBlockContent('hologram', {})
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.message).toContain('hologram')
  })

  it('rejects null and undefined, which is what a broken API would send', () => {
    expect(parseBlockContent('reading', null).ok).toBe(false)
    expect(parseBlockContent('reading', undefined).ok).toBe(false)
  })
})

describe('parseQuestionPrompt', () => {
  it('accepts an mcq prompt with options', () => {
    const result = parseQuestionPrompt('mcq_single', {
      text: 'Which is an element?',
      options: ['Water', 'Iron'],
    })
    expect(result.ok).toBe(true)
    if (result.ok && 'options' in result.data) {
      expect(result.data.options).toHaveLength(2)
    }
  })

  it('rejects an mcq prompt with fewer than two options', () => {
    expect(parseQuestionPrompt('mcq_single', { text: 'x', options: ['only'] }).ok).toBe(false)
  })

  it('accepts a true/false prompt', () => {
    expect(parseQuestionPrompt('true_false', { text: 'Iron is an element' }).ok).toBe(true)
  })

  it('accepts a typed prompt', () => {
    expect(parseQuestionPrompt('typed', { text: 'Symbol for iron?' }).ok).toBe(true)
  })
})

describe('quizCompletion', () => {
  it('accepts a threshold in range', () => {
    expect(quizCompletion.safeParse({ kind: 'score_threshold', threshold: 70 }).success).toBe(true)
  })

  it('rejects a threshold outside 0-100, matching the server', () => {
    expect(quizCompletion.safeParse({ kind: 'score_threshold', threshold: 150 }).success).toBe(false)
    expect(quizCompletion.safeParse({ kind: 'score_threshold', threshold: -1 }).success).toBe(false)
  })
})
