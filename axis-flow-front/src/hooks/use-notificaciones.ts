import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { notificacionesService } from '@/api/notificaciones-service'
import type { TokenRegistrationInput } from '@/schemas/notificaciones-schemas'

const UNREAD_COUNT_KEY = (clientId: string) => ['notificaciones', 'unread', clientId]
const PUSH_HISTORY_KEY = (userId: string) => ['notificaciones', 'history', userId]

export const useUnreadCount = (clientId: string) =>
  useQuery({
    queryKey: UNREAD_COUNT_KEY(clientId),
    queryFn: () => notificacionesService.getUnreadCount(clientId),
    enabled: !!clientId,
    staleTime: 15 * 1000,
    refetchInterval: 60 * 1000,
  })

export const usePushHistory = (userId: string, page?: number, pageSize?: number) =>
  useQuery({
    queryKey: [...PUSH_HISTORY_KEY(userId), page, pageSize],
    queryFn: () => notificacionesService.getPushHistory(userId, page, pageSize),
    enabled: !!userId,
    staleTime: 5 * 60 * 1000,
  })

export const useRegisterTokenMutation = () =>
  useMutation({
    mutationFn: (data: TokenRegistrationInput) => notificacionesService.registerToken(data),
  })

export const useTriggerRecoveryEmailMutation = () =>
  useMutation({
    mutationFn: (email: string) => notificacionesService.triggerRecoveryEmail(email),
  })

export const useTriggerNewUserEmailMutation = () =>
  useMutation({
    mutationFn: ({
      userId,
      email,
      name,
    }: {
      userId: string
      email: string
      name: string
    }) => notificacionesService.triggerNewUserEmail(userId, email, name),
  })

export const useMarkAsReadMutation = (clientId: string) => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notificationId: string) =>
      notificacionesService.markAsRead(notificationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: UNREAD_COUNT_KEY(clientId) })
    },
  })
}
