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

describe('loadYouTubeApi failure', () => {
  beforeEach(() => {
    document.head.innerHTML = ''
    __resetYouTubeApiForTests()
    // @ts-expect-error clearing the global between tests
    delete window.YT
  })

  it('rejects when the script fails to load instead of hanging forever', async () => {
    // Offline, blocked by an extension, or blocked by corporate DNS: without an
    // onerror handler the promise never settles, so VideoBlock shows a black box
    // and the text "this completes itself when the video reaches the end" —
    // forever, with no error and no way to complete the block.
    const promise = loadYouTubeApi()

    const script = document.querySelector('script[src*="youtube.com/iframe_api"]')
    expect(script).not.toBeNull()
    script!.dispatchEvent(new Event('error'))

    await expect(promise).rejects.toThrow()
  })

  it('allows a retry after a failure rather than caching the rejection', async () => {
    const first = loadYouTubeApi()
    document.querySelector('script[src*="youtube.com/iframe_api"]')!.dispatchEvent(new Event('error'))
    await expect(first).rejects.toThrow()

    // A remount must be able to try again.
    document.head.innerHTML = ''
    const second = loadYouTubeApi()
    expect(document.querySelectorAll('script[src*="youtube.com/iframe_api"]')).toHaveLength(1)
    // @ts-expect-error minimal stub
    window.YT = { Player: vi.fn(), PlayerState: { ENDED: 0 } }
    window.onYouTubeIframeAPIReady?.()
    await expect(second).resolves.toBeDefined()
  })
})
