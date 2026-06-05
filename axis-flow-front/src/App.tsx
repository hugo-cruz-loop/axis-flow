import { Routes, Route, Navigate } from 'react-router-dom'
import { LoginPage } from '@/pages/LoginPage'
import { ActivationPage } from '@/pages/ActivationPage'
import { ResetPasswordPage } from '@/pages/ResetPasswordPage'
import { AccountMePage } from '@/pages/AccountMePage'
import { UsersListPage } from '@/pages/admin/UsersListPage'
import { CreateUserPage } from '@/pages/admin/CreateUserPage'
import { DashboardPage } from '@/pages/dashboard/DashboardPage'
import { AnalyticsPage } from '@/pages/dashboard/AnalyticsPage'
import { RolesPage } from '@/pages/dashboard/RolesPage'
import { PermissionsPage } from '@/pages/dashboard/PermissionsPage'
import { SettingsPage } from '@/pages/dashboard/SettingsPage'
import { HelpPage } from '@/pages/dashboard/HelpPage'
import { UsersPage } from '@/pages/dashboard/users/UsersPage'
import { DashboardCreateUserPage } from '@/pages/dashboard/users/CreateUserPage'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { RoleGate } from '@/components/RoleGate'

// Role codes as stored in identity_roles.code and embedded in the JWT.
const ADMIN_ROLES = ['ADMIN_CHECK_ON', 'ADMINISTRADOR'] as const

function App() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/activate-account" element={<ActivationPage />} />
      <Route path="/reset-password" element={<ResetPasswordPage />} />

      {/* Root redirect */}
      <Route path="/" element={<Navigate to="/dashboard" replace />} />

      {/* Dashboard routes */}
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <DashboardPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/analytics"
        element={
          <ProtectedRoute>
            <AnalyticsPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/users"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <UsersPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/users/new"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <DashboardCreateUserPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/roles"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <RolesPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/permissions"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <PermissionsPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/settings"
        element={
          <ProtectedRoute>
            <SettingsPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/help"
        element={
          <ProtectedRoute>
            <HelpPage />
          </ProtectedRoute>
        }
      />

      {/* Legacy routes — kept for backward compatibility */}
      <Route
        path="/account/me"
        element={
          <ProtectedRoute>
            <AccountMePage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/admin/users"
        element={<Navigate to="/dashboard/users" replace />}
      />
      <Route
        path="/admin/users/new"
        element={<Navigate to="/dashboard/users/new" replace />}
      />

      {/* Fallback for old admin pages (kept intact) */}
      <Route
        path="/admin/users-legacy"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <UsersListPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/admin/users-legacy/new"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <CreateUserPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}

export default App
