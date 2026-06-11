import React from 'react';
import { DashboardLayout } from '@/layouts/DashboardLayout';
import { useGridNavigation } from '@/hooks/use-grid-navigation';
import { WidgetCard } from '@/components/dashboard/WidgetCard';
import { MetricStat } from '@/components/dashboard/MetricStat';
import { EvaluationChart } from '@/components/dashboard/EvaluationChart';
import { DashboardLiveNotifier } from '@/components/dashboard/DashboardLiveNotifier';
import {
  useEvaluacionesClientes,
  useEmpleadosCount,
  useActividadesRealizadas,
  useTodasAusencias,
} from '@/hooks/use-dashboard-queries';

export const AdminEmpresaDashboard: React.FC = () => {
  const empresaId = localStorage.getItem('empresa_id') || '';
  
  // React Query fetchers
  const { data: evaluations, isLoading: evalLoading, isError: evalErr, error: evalErrObj, refetch: refetchEval, isRefetching: evalRefetching } = useEvaluacionesClientes(empresaId);
  const { data: employees, isLoading: empLoading, isError: empErr, error: empErrObj, refetch: refetchEmp, isRefetching: empRefetching } = useEmpleadosCount(empresaId);
  const { data: activities, isLoading: actLoading, isError: actErr, error: actErrObj, refetch: refetchAct, isRefetching: actRefetching } = useActividadesRealizadas(empresaId);
  const { data: absences, isLoading: absLoading, isError: absErr, error: absErrObj, refetch: refetchAbs, isRefetching: absRefetching } = useTodasAusencias(empresaId);

  // Initialize keyboard 2D grid focus navigation (3 columns x 2 rows grid)
  const gridContainerRef = useGridNavigation({ cols: 3 });

  const isRefetching = empRefetching || actRefetching || absRefetching || evalRefetching;
  const isError = empErr || actErr || absErr || evalErr;

  return (
    <DashboardLayout>
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-100">
            Enterprise Management Dashboard
          </h1>
          <p className="text-xs text-slate-500">Aggregated operations and evaluation indices.</p>
        </div>
      </div>

      {/* Screen Reader Live Region */}
      <DashboardLiveNotifier
        isRefetching={isRefetching}
        isError={isError}
        widgetName="Enterprise Management Dashboard"
      />

      {/* Grid container with reference for arrow navigation */}
      <div 
        ref={gridContainerRef}
        className="grid grid-cols-1 md:grid-cols-3 gap-6"
        role="grid"
        aria-label="Admin Dashboard Grid Layout"
      >
        {/* KPI 1: Employees Count */}
        <div role="gridcell" className="focus:outline-none">
          <WidgetCard
            title="Total Employees"
            description="Active headcount in system rosters"
            isLoading={empLoading}
            isError={empErr}
            error={empErrObj}
            onRefetch={refetchEmp}
            isRefetching={empRefetching}
          >
            <MetricStat 
              value={employees?.total_empleados ?? 0} 
              label="Staff members active" 
            />
          </WidgetCard>
        </div>

        {/* KPI 2: Completed Activities */}
        <div role="gridcell" className="focus:outline-none">
          <WidgetCard
            title="Completed Activities"
            description="Operational routines executed"
            isLoading={actLoading}
            isError={actErr}
            error={actErrObj}
            onRefetch={refetchAct}
            isRefetching={actRefetching}
          >
            <MetricStat 
              value={activities?.total_actividades_finalizadas ?? 0} 
              label="Completed tasks" 
            />
          </WidgetCard>
        </div>

        {/* KPI 3: Historical Absences */}
        <div role="gridcell" className="focus:outline-none">
          <WidgetCard
            title="Historical Absences"
            description="Aggregated employee absent instances"
            isLoading={absLoading}
            isError={absErr}
            error={absErrObj}
            onRefetch={refetchAbs}
            isRefetching={absRefetching}
          >
            <MetricStat 
              value={absences?.total_historico_inasistencias ?? 0} 
              label="Absences logged" 
            />
          </WidgetCard>
        </div>

        {/* Evaluation Chart: Spans across all 3 grid columns */}
        <div role="gridcell" className="md:col-span-3 focus:outline-none">
          <WidgetCard
            title="Customer Service Evaluations"
            description="Mean rating points across active customer accounts"
            isLoading={evalLoading}
            isError={evalErr}
            error={evalErrObj}
            onRefetch={refetchEval}
            isRefetching={evalRefetching}
          >
            <EvaluationChart data={evaluations ?? []} />
          </WidgetCard>
        </div>
      </div>
    </DashboardLayout>
  );
};
