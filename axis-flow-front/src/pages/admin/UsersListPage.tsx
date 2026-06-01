import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import * as usersClient from '@/api/usersClient'
import { UserTable } from '@/features/users/UserTable'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'

export function UsersListPage() {
  const {
    data: users,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['users'],
    queryFn: () => usersClient.listAllUsers(),
  })

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Users</h1>
        <Link
          to="/admin/users/new"
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          New User
        </Link>
      </div>

      {isLoading && (
        <p className="text-gray-500">Loading users...</p>
      )}

      {isError && (
        <ErrorState
          message="Failed to load users."
          onRetry={() => void refetch()}
        />
      )}

      {!isLoading && !isError && users && users.length === 0 && (
        <EmptyState message="No users found." />
      )}

      {!isLoading && !isError && users && users.length > 0 && (
        <UserTable users={users} />
      )}
    </div>
  )
}
