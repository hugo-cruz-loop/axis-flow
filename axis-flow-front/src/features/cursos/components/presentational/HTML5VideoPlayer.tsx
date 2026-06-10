import { useRef, type RefObject } from 'react'
import { useVideoKeyboard } from '../../hooks/useVideoKeyboard'

interface HTML5VideoPlayerProps {
  url: string
  onEnded?: () => void
}

export function HTML5VideoPlayer({ url, onEnded }: HTML5VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null) as RefObject<HTMLVideoElement | null>
  useVideoKeyboard(videoRef)

  return (
    <video
      ref={videoRef}
      src={url}
      controls
      onEnded={onEnded}
      aria-label="Video player"
      className="w-full rounded-md bg-black"
    />
  )
}
