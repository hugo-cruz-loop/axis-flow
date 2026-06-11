import React from 'react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { DashboardLayout } from '@/layouts/DashboardLayout';
import { useGridNavigation } from '@/hooks/use-grid-navigation';
import { WidgetCard } from '@/components/dashboard/WidgetCard';
import { MetricStat } from '@/components/dashboard/MetricStat';
import { DashboardLiveNotifier } from '@/components/dashboard/DashboardLiveNotifier';
import { useTotalTrabajosActivos, useTotalAusenciaTrabajadores } from '@/hooks/use-dashboard-queries';

export const RHDashboard: React.FC = () => {
  const empresaId = localStorage.getItem('empresa_id') || '';

  const { data: jobPostings, isLoading: jobLoading, isError: jobErr, error: jobErrObj, refetch: refetchJob, isRefetching: jobRefetching } = useTotalTrabajosActivos(empresaId);
  const { data: absenceMetrics, isLoading: absLoading, isError: absErr, error: absErrObj, refetch: refetchAbs, isRefetching: absRefetching } = useTotalAusenciaTrabajadores(empresaId);

  // Initialize keyboard grid navigation (2 columns grid)
  const gridContainerRef = useGridNavigation({ cols: 2 });

  const isRefetching = jobRefetching || absRefetching;
  const isError = jobErr || absErr;

  const comparativaMensual = absenceMetrics?.comparativa_mensual ?? [];

  return (
    <DashboardLayout>
      <header className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-100">
          Human Resources Cockpit
        </h1>
        <p className="text-xs text-slate-500">Recruitment metrics and absenteeism tracking profiles.</p>
      </header>

      {/* Screen Reader Live Region */}
      <DashboardLiveNotifier
        isRefetching={isRefetching}
        isError={isError}
        widgetName="HR Metrics Dashboard"
      />

      <div 
        ref={gridContainerRef}
        className="grid grid-cols-1 md:grid-cols-2 gap-6"
        role="grid"
        aria-label="HR Metrics Dashboard Layout"
      >
        {/* Active job postings widget */}
        <div role="gridcell" className="focus:outline-none">
          <WidgetCard
            title="Active Job Vacancies"
            description="Open listings within job board rosters"
            isLoading={jobLoading}
            isError={jobErr}
            error={jobErrObj}
            onRefetch={refetchJob}
            isRefetching={jobRefetching}
          >
            <MetricStat
              value={jobPostings?.total_vacantes_activas ?? 0}
              label="Vacancies available"
            />
          </WidgetCard>
        </div>

        {/* Absenteeism rate widget */}
        <div role="gridcell" className="focus:outline-none">
          <WidgetCard
            title="Absenteeism Index"
            description="Operational percentage loss due to absent instances"
            isLoading={absLoading}
            isError={absErr}
            error={absErrObj}
            onRefetch={refetchAbs}
            isRefetching={absRefetching}
          >
            <MetricStat
              value={absenceMetrics?.tasa_absentismo ?? 0}
              label="Absenteeism rate %"
              isPercentage
            />
          </WidgetCard>
        </div>

        {/* Detailed absent chart: Spans both columns */}
        <div role="gridcell" className="md:col-span-2 focus:outline-none">
          <WidgetCard
            title="Monthly Absenteeism Trend"
            description="Historical absenteeism rate over months"
            isLoading={absLoading}
            isError={absErr}
            error={absErrObj}
            onRefetch={refetchAbs}
            isRefetching={absRefetching}
          >
            <div className="w-full h-[220px]">
              {/* Screen Reader Tabular Representation */}
              <div className="sr-only">
                <table>
                  <caption>Monthly Absenteeism Metrics</caption>
                  <thead>
                    <tr>
                      <th scope="col">Month</th>
                      <th scope="col">Absences</th>
                    </tr>
                  </thead>
                  <tbody>
                    {comparativaMensual.map((item, idx) => (
                      <tr key={item.mes || idx}>
                        <td>{item.mes}</td>
                        <td>{item.inasistencias} absences</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Visual Graphical Representation (Hidden from Assistive Tech) */}
              <div className="w-full h-full" aria-hidden="true" data-testid="absenteeism-chart-graphic">
                {comparativaMensual.length === 0 ? (
                  <div className="w-full h-full flex justify-center items-center text-slate-400 text-xs">
                    No historical absenteeism data.
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={comparativaMensual} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                      <XAxis dataKey="mes" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip />
                      <Line type="monotone" dataKey="inasistencias" stroke="#f43f5e" strokeWidth={2} activeDot={{ r: 8 }} />
                    </LineChart>
                  </ResponsiveContainer>
                )}
              </div>
            </div>
          </WidgetCard>
        </div>
      </div>
    </DashboardLayout>
  );
};
