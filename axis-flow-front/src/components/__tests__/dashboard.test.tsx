import React from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { WidgetCard } from '../dashboard/WidgetCard'
import { WidgetErrorBoundary } from '../dashboard/WidgetErrorBoundary'
import { MetricStat } from '../dashboard/MetricStat'
import { EvaluationChart } from '../dashboard/EvaluationChart'
import { TicketBreakdown } from '../dashboard/TicketBreakdown'
import { ServicesTable } from '../dashboard/ServicesTable'
import { DashboardLiveNotifier } from '../dashboard/DashboardLiveNotifier'

// Mock Recharts to avoid ResizeObserver / SVG rendering issues in test environment
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: any) => React.createElement('div', { 'data-testid': 'responsive-container' }, children),
  BarChart: ({ children }: any) => React.createElement('div', { 'data-testid': 'bar-chart' }, children),
  Bar: () => React.createElement('div', { 'data-testid': 'bar' }),
  XAxis: () => React.createElement('div', { 'data-testid': 'xaxis' }),
  YAxis: () => React.createElement('div', { 'data-testid': 'yaxis' }),
  Tooltip: () => React.createElement('div', { 'data-testid': 'tooltip' }),
  PieChart: ({ children }: any) => React.createElement('div', { 'data-testid': 'pie-chart' }, children),
  Pie: ({ children }: any) => React.createElement('div', { 'data-testid': 'pie' }, children),
  Cell: () => React.createElement('div', { 'data-testid': 'cell' }),
  Legend: () => React.createElement('div', { 'data-testid': 'legend' }),
  LineChart: ({ children }: any) => React.createElement('div', { 'data-testid': 'line-chart' }, children),
  Line: () => React.createElement('div', { 'data-testid': 'line' }),
}))

describe('WidgetCard Accessibility and States', () => {
  it('renders title and content with correct ARIA labelling', () => {
    const onRefetch = vi.fn()
    render(
      <WidgetCard
        title="Test Widget Title"
        description="Test description"
        isLoading={false}
        isError={false}
        onRefetch={onRefetch}
      >
        <div>Widget Content</div>
      </WidgetCard>
    )

    const section = screen.getByRole('region', { name: 'Test Widget Title' })
    expect(section).toBeInTheDocument()
    expect(section).toHaveAttribute('aria-labelledby', 'widget-title-test-widget-title')
    expect(screen.getByText('Widget Content')).toBeInTheDocument()
    expect(screen.getByText('Test description')).toBeInTheDocument()
  })

  it('renders loading skeleton state with aria-hidden', () => {
    render(
      <WidgetCard
        title="Test Widget Title"
        isLoading={true}
        isError={false}
        onRefetch={vi.fn()}
      >
        <div>Content</div>
      </WidgetCard>
    )

    const loadingEl = screen.getByTestId('widget-card-loading')
    expect(loadingEl).toHaveAttribute('aria-hidden', 'true')
    expect(screen.queryByText('Content')).not.toBeInTheDocument()
  })

  it('renders error state with role=alert', () => {
    const errorMsg = new Error('Network timeout')
    render(
      <WidgetCard
        title="Test Widget Title"
        isLoading={false}
        isError={true}
        error={errorMsg}
        onRefetch={vi.fn()}
      >
        <div>Content</div>
      </WidgetCard>
    )

    const alertEl = screen.getByRole('alert')
    expect(alertEl).toBeInTheDocument()
    expect(screen.getByText('Error fetching metric data')).toBeInTheDocument()
    expect(screen.getByText('Network timeout')).toBeInTheDocument()
    expect(screen.queryByText('Content')).not.toBeInTheDocument()
  })

  it('calls onRefetch on refresh button click', async () => {
    const onRefetch = vi.fn()
    const user = userEvent.setup()
    render(
      <WidgetCard
        title="Test Widget Title"
        isLoading={false}
        isError={false}
        onRefetch={onRefetch}
      >
        <div>Content</div>
      </WidgetCard>
    )

    const button = screen.getByRole('button', { name: /refresh data for test widget title/i })
    await user.click(button)
    expect(onRefetch).toHaveBeenCalledOnce()
  })
})

describe('WidgetErrorBoundary Failure Isolation', () => {
  const BadComponent = () => {
    throw new Error('Crashed on render')
  }

  it('isolates crash and displays reset widget controls', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const onReset = vi.fn()
    render(
      <WidgetErrorBoundary title="Crashed Widget" onReset={onReset}>
        <BadComponent />
      </WidgetErrorBoundary>
    )

    expect(screen.getByText('Widget Crashed')).toBeInTheDocument()
    expect(screen.getByText('Error')).toBeInTheDocument()
    
    const resetButton = screen.getByRole('button', { name: /reset widget/i })
    resetButton.click()

    expect(onReset).toHaveBeenCalledOnce()
    spy.mockRestore()
  })
})

describe('MetricStat Content Formatting', () => {
  it('formats large integers with localized commas', () => {
    render(<MetricStat value={15280} label="Total staff" />)
    const val = screen.getByTestId('metric-value')
    expect(val.textContent).toBe('15,280')
  })

  it('appends percentage symbol when isPercentage is true', () => {
    render(<MetricStat value={4.7} label="Absenteeism rate" isPercentage={true} />)
    const val = screen.getByTestId('metric-value')
    expect(val.textContent).toBe('4.7%')
  })
})

describe('EvaluationChart Fallbacks and Tech Hiding', () => {
  const evalData = [
    {
      cliente_id: 'c1-uuid',
      cliente_nombre: 'Acme Corp',
      promedio_puntuacion: 8.5,
      total_evaluaciones: 12,
    },
    {
      cliente_id: 'c2-uuid',
      cliente_nombre: 'Globex Corp',
      promedio_puntuacion: 9.1,
      total_evaluaciones: 25,
    }
  ]

  it('shows empty text when data is empty', () => {
    render(<EvaluationChart data={[]} />)
    expect(screen.getByText('No evaluations registered.')).toBeInTheDocument()
  })

  it('provides visually hidden, fully accessible table and hides graphic container', () => {
    render(<EvaluationChart data={evalData} />)

    // Table elements should be in the DOM for screen readers
    const table = screen.getByRole('table', { name: /client evaluation scores/i })
    expect(table).toBeInTheDocument()

    const rows = screen.getAllByRole('row')
    expect(rows).toHaveLength(3) // 1 header + 2 items

    expect(screen.getByText('Acme Corp')).toBeInTheDocument()
    expect(screen.getByText('8.5 out of 10')).toBeInTheDocument()

    // Graphic component must have aria-hidden
    const graphic = screen.getByTestId('evaluation-chart-graphic')
    expect(graphic).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('TicketBreakdown Fallbacks and Tech Hiding', () => {
  const ticketData = {
    cliente_id: 'c1-uuid',
    pendiente: 3,
    pendientes: 3,
    en_proceso: 5,
    finalizado: 12,
    finalizados: 12,
    total_tickets: 20,
  }

  it('shows empty state when metrics is missing or empty', () => {
    render(<TicketBreakdown metrics={undefined} />)
    expect(screen.getByText('No tickets registered.')).toBeInTheDocument()
  })

  it('renders screen reader table fallback and hides svg chart', () => {
    render(<TicketBreakdown metrics={ticketData} />)

    const table = screen.getByRole('table', { name: /service tickets status/i })
    expect(table).toBeInTheDocument()

    expect(screen.getByText('Pending')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('In Progress')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.getByText('Completed')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()

    const graphic = screen.getByTestId('ticket-breakdown-graphic')
    expect(graphic).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('ServicesTable Fallbacks and Visual Table Grid', () => {
  const serviceData = [
    {
      localidad_id: 'l1',
      localidad_nombre: 'North Side',
      servicio_id: 's1',
      servicio_nombre: 'Cleaning',
      empleados_asignados: 8,
    },
    {
      localidad_id: 'l2',
      localidad_nombre: 'Downtown',
      servicio_id: 's2',
      servicio_nombre: 'Security',
      empleados_asignados: 15,
    }
  ]

  it('shows empty state when no items', () => {
    render(<ServicesTable items={[]} />)
    expect(screen.getByText('No service locations assigned.')).toBeInTheDocument()
  })

  it('renders both visual table and screen reader table fallback while hiding graph', () => {
    const { container } = render(<ServicesTable items={serviceData} />)

    // Fallback table for screen readers
    const srTable = screen.getByRole('table', { name: /service locations and assigned staff/i })
    expect(srTable).toBeInTheDocument()

    // 6 rows total = 3 sr-table rows + 3 visual-table rows in the DOM
    const rows = container.querySelectorAll('tr')
    expect(rows).toHaveLength(6)

    expect(screen.getAllByText('North Side')).toHaveLength(2)
    expect(screen.getAllByText('Downtown')).toHaveLength(2)

    const graphic = screen.getByTestId('services-table-graphic')
    expect(graphic).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('DashboardLiveNotifier Live Region Alerts', () => {
  it('has role=status and polite aria-live configurations', () => {
    render(<DashboardLiveNotifier isRefetching={false} isError={false} widgetName="Test" />)
    
    const notifier = screen.getByTestId('live-notifier')
    expect(notifier).toHaveAttribute('role', 'status')
    expect(notifier).toHaveAttribute('aria-live', 'polite')
    expect(notifier).toHaveAttribute('aria-atomic', 'true')
  })

  it('announces refreshing state, success state, and error state changes', () => {
    const { rerender } = render(<DashboardLiveNotifier isRefetching={true} isError={false} widgetName="Roster" />)
    expect(screen.getByText('Refreshing data for Roster...')).toBeInTheDocument()

    rerender(<DashboardLiveNotifier isRefetching={false} isError={false} widgetName="Roster" />)
    expect(screen.getByText('Roster metrics updated successfully.')).toBeInTheDocument()

    rerender(<DashboardLiveNotifier isRefetching={false} isError={true} widgetName="Roster" />)
    expect(screen.getByText('Failed to update Roster data.')).toBeInTheDocument()
  })
})
