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
import { HelpPage } from '@/pages/dashboard/HelpPage'
import { UsersPage } from '@/pages/dashboard/users/UsersPage'
import { DashboardCreateUserPage } from '@/pages/dashboard/users/CreateUserPage'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { RoleGate } from '@/components/RoleGate'
import { CatalogosDashboard } from '@/pages/dashboard/CatalogosDashboard'
import { EmpresaDashboard } from '@/pages/dashboard/empresa/EmpresaDashboard'
import { ClientesPage } from '@/pages/dashboard/clientes/ClientesPage'
import { ClientDetailPage } from '@/pages/dashboard/clientes/ClientDetailPage'
import { EmpleadosPage } from '@/pages/dashboard/empleados/EmpleadosPage'
import { EmpleadoDetailPage } from '@/pages/dashboard/empleados/EmpleadoDetailPage'
import { CursosPage } from '@/features/cursos/pages/CursosPage'
import { CursoPlayerPage } from '@/features/cursos/pages/CursoPlayerPage'
import { QuizPage } from '@/features/cursos/pages/QuizPage'
import { CertificadoPage } from '@/features/cursos/pages/CertificadoPage'
import { JobBoardPage } from '@/features/recruitment/pages/JobBoardPage'
import { RecruiterPage } from '@/features/recruitment/pages/RecruiterPage'
import { PipelinePage } from '@/features/recruitment/pages/PipelinePage'
import { EmployeePage } from '@/features/atencion/pages/EmployeePage'
import { HRPage } from '@/features/atencion/pages/HRPage'
import { ClientPage } from '@/features/atencion/pages/ClientPage'
import { SupervisorPage } from '@/features/atencion/pages/SupervisorPage'
import { FormBuilderPage } from '@/features/formularios/pages/FormBuilderPage'
import { FormAssignmentPage } from '@/features/formularios/pages/FormAssignmentPage'
import { MobileFormCapturePage } from '@/features/formularios/pages/MobileFormCapturePage'
import { AdminEmpresaDashboard } from '@/screens/AdminEmpresaDashboard'
import { ClienteDashboard } from '@/screens/ClienteDashboard'
import { RHDashboard } from '@/screens/RHDashboard'
import { AdminSettingsPage } from '@/features/parametrizacion'


// Role codes as stored in identity_roles.code and embedded in the JWT.
const ADMIN_ROLES = ['ADMIN_CHECK_ON', 'ADMINISTRADOR'] as const
const EMPLEADOS_ROLES = ['ADMIN_CHECK_ON', 'ADMINISTRADOR', 'RH'] as const

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
        path="/dashboard/admin-empresa"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <AdminEmpresaDashboard />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/cliente"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={['ADMIN_CHECK_ON', 'ADMINISTRADOR', 'CLIENTE']}>
              <ClienteDashboard />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/rh"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...EMPLEADOS_ROLES]}>
              <RHDashboard />
            </RoleGate>
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
        path="/dashboard/catalogos"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <CatalogosDashboard />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/admin/settings"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <AdminSettingsPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/settings"
        element={<Navigate to="/admin/settings" replace />}
      />
      <Route
        path="/dashboard/help"
        element={
          <ProtectedRoute>
            <HelpPage />
          </ProtectedRoute>
        }
      />

      <Route
        path="/dashboard/clientes"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <ClientesPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/clientes/:id"
        element={
          <ProtectedRoute>
            <ClientDetailPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/empleados"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...EMPLEADOS_ROLES]}>
              <EmpleadosPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/empleados/:numEmpleado"
        element={
          <ProtectedRoute>
            <EmpleadoDetailPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/empresa/:id"
        element={
          <ProtectedRoute>
            <EmpresaDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/empresa"
        element={<Navigate to="/dashboard" replace />}
      />

      {/* Cursos routes */}
      <Route
        path="/cursos"
        element={
          <ProtectedRoute>
            <CursosPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/cursos/:cursoId/player"
        element={
          <ProtectedRoute>
            <CursoPlayerPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/cursos/:cursoId/examen"
        element={
          <ProtectedRoute>
            <QuizPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/cursos/:examenId/certificado"
        element={
          <ProtectedRoute>
            <CertificadoPage />
          </ProtectedRoute>
        }
      />

      {/* Recruitment routes */}
      <Route path="/jobs" element={<JobBoardPage />} />
      <Route
        path="/recruiter"
        element={
          <ProtectedRoute>
            <RecruiterPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/recruiter/pipeline/:trabajoId"
        element={
          <ProtectedRoute>
            <PipelinePage />
          </ProtectedRoute>
        }
      />

      {/* Atencion routes */}
      <Route
        path="/atencion/empleado"
        element={
          <ProtectedRoute>
            <EmployeePage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/atencion/rh"
        element={
          <ProtectedRoute>
            <HRPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/atencion/cliente"
        element={
          <ProtectedRoute>
            <ClientPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/atencion/supervisor"
        element={
          <ProtectedRoute>
            <SupervisorPage />
          </ProtectedRoute>
        }
      />

      {/* Formularios routes (admin + mobile capture) */}
      <Route
        path="/dashboard/formularios/builder"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <FormBuilderPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/dashboard/formularios/assignment"
        element={
          <ProtectedRoute>
            <RoleGate allowedRoles={[...ADMIN_ROLES]}>
              <FormAssignmentPage />
            </RoleGate>
          </ProtectedRoute>
        }
      />
      <Route
        path="/mobile/formularios/capture/:eventoId"
        element={
          <ProtectedRoute>
            <MobileFormCapturePage />
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
