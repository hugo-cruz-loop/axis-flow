import { useState, useEffect, useCallback } from 'react'
import type { RespuestaQueja, RespuestaServicio } from '../types'

type AnyRespuesta = RespuestaQueja | RespuestaServicio

/**
 * Polls chat messages at a fixed interval when WebSocket is not available.
 * Stops polling when the component unmounts.
 */
export function useChatPolling(
  queryKey: readonly string[],
  fetchFn: () => Promise<AnyRespuesta[]>,
  intervalMs = 10000,
): { messages: AnyRespuesta[]; isLoading: boolean } {
  const [messages, setMessages] = useState<AnyRespuesta[]>([])
  const [isLoading, setIsLoading] = useState(true)

  // Memoize the key as a stable string for the effect dependency
  const keyString = queryKey.join('|')

  const poll = useCallback(async () => {
    try {
      const data = await fetchFn()
      setMessages(data)
    } catch {
      // Silently swallow polling errors to avoid disrupting the UI
    } finally {
      setIsLoading(false)
    }
  }, // eslint-disable-next-line react-hooks/exhaustive-deps
  [keyString])

  useEffect(() => {
    let active = true

    void (async () => {
      if (active) await poll()
    })()

    const id = setInterval(() => {
      if (active) void poll()
    }, intervalMs)

    return () => {
      active = false
      clearInterval(id)
    }
  }, [poll, intervalMs])

  return { messages, isLoading }
}
