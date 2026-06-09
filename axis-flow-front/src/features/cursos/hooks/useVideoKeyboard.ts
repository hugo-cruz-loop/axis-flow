import { useEffect, type RefObject } from 'react'

/**
 * Attaches Space/ArrowLeft/ArrowRight keyboard shortcuts to an HTMLVideoElement ref.
 * Space = play/pause, ArrowRight = +10s, ArrowLeft = -10s.
 */
export function useVideoKeyboard(videoRef: RefObject<HTMLVideoElement | null>): void {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const video = videoRef.current
      if (!video) return

      // Avoid intercepting events when user is typing in an input/textarea
      const target = e.target as HTMLElement
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return

      switch (e.code) {
        case 'Space':
          e.preventDefault()
          if (video.paused) {
            video.play()
          } else {
            video.pause()
          }
          break
        case 'ArrowRight':
          e.preventDefault()
          video.currentTime = Math.min(video.currentTime + 10, video.duration)
          break
        case 'ArrowLeft':
          e.preventDefault()
          video.currentTime = Math.max(video.currentTime - 10, 0)
          break
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [videoRef])
}
