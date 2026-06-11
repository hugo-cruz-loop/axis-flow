import React from 'react';
import { DashboardLayout } from '@/layouts/DashboardLayout';
import { useGridNavigation } from '@/hooks/use-grid-navigation';
import { WidgetCard } from '@/components/dashboard/WidgetCard';
import { ServicesTable } from '@/components/dashboard/ServicesTable';
import { TicketBreakdown } from '@/components/dashboard/TicketBreakdown';
import { DashboardLiveNotifier } from '@/components/dashboard/DashboardLiveNotifier';
import { useServiciosLocalidad, useEstatusAtencionSeguimiento } from '@/hooks/use-dashboard-queries';

export const ClienteDashboard: React.FC = () => {
  const clienteId = localStorage.getItem('cliente_id') || '';

  const { data: services, isLoading: srvLoading, isError: srvErr, error: srvErrObj, refetch: refetchSrv, isRefetching: srvRefetching } = useServiciosLocalidad(clienteId);
  const { data: tickets, isLoading: tktLoading, isError: tktErr, error: tktErrObj, refetch: refetchTkt, isRefetching: tktRefetching } = useEstatusAtencionSeguimiento(clienteId);

  // Initialize keyboard grid navigation (2 columns grid)
  const gridContainerRef = useGridNavigation({ cols: 2 });

  const isRefetching = srvRefetching || tktRefetching;
  const isError = srvErr || tktErr;

  return (
    <DashboardLayout>
      <header className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-100">
          Client Operations Center
        </h1>
        <p className="text-xs text-slate-500">Real-time status of service locations and active tickets.</p>
      </header>

      {/* Screen Reader Live Region */}
      <DashboardLiveNotifier
        isRefetching={isRefetching}
        isError={isError}
        widgetName="Client Operations Dashboard"
      />

      <div 
        ref={gridContainerRef}
        className="grid grid-cols-1 md:grid-cols-3 gap-6"
        role="grid"
        aria-label="Client Operations Dashboard Layout"
      >
        {/* Services Table widget: spans 2 columns */}
        <div role="gridcell" className="md:col-span-2 focus:outline-none">
          <WidgetCard
            title="Service Locations and Roster"
            description="Employees assigned across geographic branch offices"
            isLoading={srvLoading}
            isError={srvErr}
            error={srvErrObj}
            onRefetch={refetchSrv}
            isRefetching={srvRefetching}
          >
            <ServicesTable items={services ?? []} />
          </WidgetCard>
        </div>

        {/* Ticket breakdown chart widget: spans 1 column */}
        <div role="gridcell" className="md:col-span-1 focus:outline-none">
          <WidgetCard
            title="Service Ticket Breakdown"
            description="Distribution of support requests by priority state"
            isLoading={tktLoading}
            isError={tktErr}
            error={tktErrObj}
            onRefetch={refetchTkt}
            isRefetching={tktRefetching}
          >
            <TicketBreakdown metrics={tickets} />
          </WidgetCard>
        </div>
      </div>
    </DashboardLayout>
  );
};
