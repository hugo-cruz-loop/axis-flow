import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { OnboardingWizard } from '../OnboardingWizard'

vi.mock('@/api/catalogosClient', () => ({
  listSubscriptionPlans: vi.fn().mockResolvedValue([
    { id: 1, code: 'BASIC', name: 'Basic Plan', amount: 500 },
    { id: 2, code: 'PRO', name: 'Pro Plan', amount: 1500 },
  ]),
}))

vi.mock('@/api/empresasClient', () => ({
  registerEmpresa: vi.fn(),
  createCheckoutSession: vi.fn(),
}))

function renderWizard() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <OnboardingWizard />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('OnboardingWizard', () => {
  it('renders step 1 with plan selection', async () => {
    renderWizard()
    await waitFor(() => {
      expect(screen.getByText('Basic Plan')).toBeInTheDocument()
      expect(screen.getByText('Pro Plan')).toBeInTheDocument()
    })
    expect(screen.getByText('Choose a Plan')).toBeInTheDocument()
  })

  it('advances to step 2 after plan selected', async () => {
    const user = userEvent.setup()
    renderWizard()
    await waitFor(() => {
      expect(screen.getByText('Basic Plan')).toBeInTheDocument()
    })
    // Select a plan
    const selectButtons = screen.getAllByRole('button', { name: /select/i })
    await user.click(selectButtons[0])
    // Click Next
    const nextBtn = screen.getByRole('button', { name: /next/i })
    await user.click(nextBtn)
    await waitFor(() => {
      expect(screen.getByText('Company Information')).toBeInTheDocument()
    })
  })
})
