import { describe, it, expect, beforeEach, vi } from 'vitest'
import { loadYouTubeApi, __resetYouTubeApiForTests } from './useYouTubePlayer'

describe('loadYouTubeApi', () => {
  beforeEach(() => {
    document.head.innerHTML = ''
    __resetYouTubeApiForTests()
    // @ts-expect-error clearing the global between tests
    delete window.YT
  })

  it('injects the iframe API script exactly once, even when two blocks ask', () => {
    void loadYouTubeApi()
    void loadYouTubeApi()

    const scripts = document.querySelectorAll('script[src*="youtube.com/iframe_api"]')
    expect(scripts).toHaveLength(1)
  })

  it('resolves immediately when the API is already present', async () => {
    // @ts-expect-error minimal stub
    window.YT = { Player: vi.fn(), PlayerState: { ENDED: 0 } }
    await expect(loadYouTubeApi()).resolves.toBeDefined()
    expect(document.querySelectorAll('script')).toHaveLength(0)
  })

  it('resolves for every caller once the API announces itself', async () => {
    const first = loadYouTubeApi()
    const second = loadYouTubeApi()

    // @ts-expect-error minimal stub
    window.YT = { Player: vi.fn(), PlayerState: { ENDED: 0 } }
    window.onYouTubeIframeAPIReady?.()

    await expect(Promise.all([first, second])).resolves.toHaveLength(2)
  })
})
