import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoleTable } from '../RoleTable'
import type { RoleWithCount } from '@/types/rbac'

const systemRole: RoleWithCount = {
  id: '1',
  code: 'ADMIN_CHECK_ON',
  name: 'Admin',
  scope: 'GLOBAL',
  is_system: true,
  permission_count: 10,
}

const customRole: RoleWithCount = {
  id: '2',
  code: 'CUSTOM_ROLE',
  name: 'Custom',
  scope: 'TENANT',
  is_system: false,
  permission_count: 3,
}

describe('RoleTable', () => {
  it('renders list of roles with lock icon for system roles', () => {
    render(
      <RoleTable
        roles={[systemRole, customRole]}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        isLoading={false}
        canEdit={true}
        canDelete={true}
      />,
    )

    const rows = screen.getAllByTestId('role-row')
    expect(rows).toHaveLength(2)

    // Lock icon only for system role
    expect(screen.getByTestId('lock-icon')).toBeInTheDocument()
  })

  it('disables edit and delete for system roles', () => {
    render(
      <RoleTable
        roles={[systemRole, customRole]}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        isLoading={false}
        canEdit={true}
        canDelete={true}
      />,
    )

    const editButtons = screen.getAllByTestId('edit-button')
    const deleteButtons = screen.getAllByTestId('delete-button')

    // First row is system role — should be disabled
    expect(editButtons[0]).toBeDisabled()
    expect(deleteButtons[0]).toBeDisabled()

    // Second row is custom role — should be enabled
    expect(editButtons[1]).not.toBeDisabled()
    expect(deleteButtons[1]).not.toBeDisabled()
  })
})
