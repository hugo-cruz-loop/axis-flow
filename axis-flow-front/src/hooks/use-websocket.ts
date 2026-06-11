import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuthStore } from '@/store/authStore'
import { wsEventPayloadSchema } from '@/schemas/notificaciones-schemas'
import type { WsEventEnvelope } from '@/schemas/notificaciones-schemas'

export type ConnectionState =
  | 'CONNECTING'
  | 'CONNECTED'
  | 'DISCONNECTED'
  | 'RECONNECTING'
  | 'ERROR'

export interface UseWebSocketOptions {
  /** WS server URL (http/https will be converted to ws/wss automatically) */
  url: string
  onMessage?: (envelope: WsEventEnvelope) => void
  onOpen?: () => void
  onClose?: () => void
  onError?: (event: Event) => void
  /** Start the connection immediately on mount (default: true) */
  autoConnect?: boolean
  /** Maximum reconnect attempts before entering ERROR state (default: 8) */
  maxRetries?: number
  /** Base delay in ms for exponential backoff (default: 500) */
  baseDelayMs?: number
}

function buildWsUrl(rawUrl: string, token: string | null): string {
  const wsUrl = rawUrl.replace(/^http/, 'ws')
  if (!token) return wsUrl
  const separator = wsUrl.includes('?') ? '&' : '?'
  return `${wsUrl}${separator}token=${encodeURIComponent(token)}`
}

function jitteredDelay(attempt: number, baseMs: number): number {
  const exp = Math.min(baseMs * 2 ** attempt, 30_000)
  return exp / 2 + Math.random() * (exp / 2)
}

export function useWebSocket({
  url,
  onMessage,
  onOpen,
  onClose,
  onError,
  autoConnect = true,
  maxRetries = 8,
  baseDelayMs = 500,
}: UseWebSocketOptions) {
  const [connectionState, setConnectionState] = useState<ConnectionState>('DISCONNECTED')
  const wsRef = useRef<WebSocket | null>(null)
  const attemptsRef = useRef(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isMountedRef = useRef(true)

  const clearRetryTimer = () => {
    if (retryTimerRef.current !== null) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
  }

  const disconnect = useCallback(() => {
    clearRetryTimer()
    if (wsRef.current) {
      wsRef.current.onclose = null // prevent reconnect loop
      wsRef.current.close()
      wsRef.current = null
    }
    if (isMountedRef.current) setConnectionState('DISCONNECTED')
  }, [])

  const connect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState <= WebSocket.OPEN) return

    const token = useAuthStore.getState().accessToken
    const resolvedUrl = buildWsUrl(url, token)

    setConnectionState(attemptsRef.current === 0 ? 'CONNECTING' : 'RECONNECTING')

    const ws = new WebSocket(resolvedUrl)
    wsRef.current = ws

    ws.onopen = () => {
      if (!isMountedRef.current) return
      attemptsRef.current = 0
      setConnectionState('CONNECTED')
      onOpen?.()
    }

    ws.onmessage = (event: MessageEvent<unknown>) => {
      if (!isMountedRef.current) return
      let raw: unknown
      try {
        raw = JSON.parse(event.data as string)
      } catch {
        return
      }
      const result = wsEventPayloadSchema.safeParse(raw)
      if (result.success) {
        onMessage?.(result.data)
      }
    }

    ws.onerror = (event: Event) => {
      if (!isMountedRef.current) return
      onError?.(event)
    }

    ws.onclose = () => {
      if (!isMountedRef.current) return
      wsRef.current = null

      if (attemptsRef.current >= maxRetries) {
        setConnectionState('ERROR')
        return
      }

      attemptsRef.current += 1
      setConnectionState('RECONNECTING')
      const delay = jitteredDelay(attemptsRef.current, baseDelayMs)
      retryTimerRef.current = setTimeout(() => {
        if (isMountedRef.current) connect()
      }, delay)
      onClose?.()
    }
  }, [url, onMessage, onOpen, onClose, onError, maxRetries, baseDelayMs]) // eslint-disable-line react-hooks/exhaustive-deps

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  const reconnect = useCallback(() => {
    attemptsRef.current = 0
    disconnect()
    connect()
  }, [connect, disconnect])

  useEffect(() => {
    isMountedRef.current = true
    if (autoConnect) connect()
    return () => {
      isMountedRef.current = false
      clearRetryTimer()
      if (wsRef.current) {
        wsRef.current.onclose = null
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return { connectionState, connect, disconnect, reconnect, send }
}
