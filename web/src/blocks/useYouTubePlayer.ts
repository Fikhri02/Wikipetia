import { useEffect, useRef, useState } from 'react'

/**
 * @types/youtube declares the YT namespace but NOT this global callback, which
 * is how the IFrame API announces readiness. Declaring it here keeps the loader
 * cast-free.
 */
declare global {
  interface Window {
    onYouTubeIframeAPIReady?: () => void
  }
}

let apiPromise: Promise<typeof YT> | null = null

/** Test-only hook to clear module state between cases. */
export function __resetYouTubeApiForTests() {
  apiPromise = null
}

/**
 * Loads the IFrame Player API once per page, however many video blocks ask.
 * The API announces readiness through a single global callback, so every
 * caller waits on one shared promise.
 */
export function loadYouTubeApi(): Promise<typeof YT> {
  if (window.YT?.Player) return Promise.resolve(window.YT)
  if (apiPromise) return apiPromise

  apiPromise = new Promise<typeof YT>((resolve) => {
    const previous = window.onYouTubeIframeAPIReady
    window.onYouTubeIframeAPIReady = () => {
      previous?.()
      resolve(window.YT)
    }

    const script = document.createElement('script')
    script.src = 'https://www.youtube.com/iframe_api'
    script.async = true
    document.head.appendChild(script)
  })

  return apiPromise
}

/**
 * Mounts a YouTube player and calls onEnded when the video genuinely finishes.
 *
 * This is why the video block is verifiable at all: the player reports its own
 * ENDED state, so completion is observed rather than asserted by the learner.
 */
export function useYouTubePlayer({
  videoId,
  onEnded,
}: {
  videoId: string
  onEnded: () => void
}) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [ready, setReady] = useState(false)
  const [failed, setFailed] = useState(false)

  // Kept in a ref so re-renders never tear down and rebuild the player.
  const onEndedRef = useRef(onEnded)
  onEndedRef.current = onEnded

  useEffect(() => {
    let cancelled = false
    let player: YT.Player | undefined

    loadYouTubeApi()
      .then((api) => {
        if (cancelled || !containerRef.current) return
        player = new api.Player(containerRef.current, {
          videoId,
          playerVars: { rel: 0, modestbranding: 1 },
          events: {
            onReady: () => {
              if (!cancelled) setReady(true)
            },
            onStateChange: (event) => {
              if (event.data === api.PlayerState.ENDED) onEndedRef.current()
            },
            onError: () => {
              if (!cancelled) setFailed(true)
            },
          },
        })
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })

    return () => {
      cancelled = true
      player?.destroy()
    }
  }, [videoId])

  return { containerRef, ready, failed }
}
