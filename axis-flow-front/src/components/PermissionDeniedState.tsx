export function PermissionDeniedState() {
  return (
    <div
      data-testid="permission-denied"
      className="flex flex-col items-center justify-center py-12 text-gray-700"
      role="alert"
    >
      <h2 className="text-lg font-semibold">Access Denied</h2>
      <p className="mt-2 text-sm text-gray-500">
        You do not have permission to view this page.
      </p>
    </div>
  )
}
