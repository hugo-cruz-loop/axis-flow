import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SimpleCatalogTable } from '../SimpleCatalogTable'

const items = [
  { id: 1, code: 'MX', name: 'Mexico' },
  { id: 2, code: 'US', name: 'United States' },
]

describe('SimpleCatalogTable', () => {
  it('renders items list', () => {
    render(
      <SimpleCatalogTable
        items={items}
        isLoading={false}
        onAdd={vi.fn()}
        onDelete={vi.fn()}
      />,
    )

    expect(screen.getByText('Mexico')).toBeInTheDocument()
    expect(screen.getByText('United States')).toBeInTheDocument()
    expect(screen.getByText('MX')).toBeInTheDocument()
  })

  it('shows skeleton when loading', () => {
    render(
      <SimpleCatalogTable
        items={[]}
        isLoading={true}
        onAdd={vi.fn()}
        onDelete={vi.fn()}
      />,
    )

    const skeletons = screen.getAllByTestId('skeleton-row')
    expect(skeletons).toHaveLength(3)
  })


  it('renders empty state safely when items is null', () => {
    render(
      <SimpleCatalogTable
        items={null}
        isLoading={false}
        onAdd={vi.fn()}
        onDelete={vi.fn()}
      />,
    )

    expect(screen.queryByText('Mexico')).not.toBeInTheDocument()
  })

  it('calls onDelete when delete button clicked', async () => {
    const onDelete = vi.fn().mockResolvedValue(undefined)
    const user = userEvent.setup()

    render(
      <SimpleCatalogTable
        items={items}
        isLoading={false}
        onAdd={vi.fn()}
        onDelete={onDelete}
      />,
    )

    const deleteButtons = screen.getAllByRole('button', { name: /delete/i })
    await user.click(deleteButtons[0])

    expect(onDelete).toHaveBeenCalledWith(1)
  })
})
