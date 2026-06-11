import React from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts';
import { TicketStatusBreakdown } from '../../schemas/dashboard-schemas';

interface TicketBreakdownProps {
  metrics?: TicketStatusBreakdown | null;
}

export const TicketBreakdown: React.FC<TicketBreakdownProps> = ({ metrics }) => {
  const data = metrics ? [
    { name: 'Pending', value: metrics.pendientes, color: '#f59e0b' },
    { name: 'In Progress', value: metrics.en_proceso, color: '#3b82f6' },
    { name: 'Completed', value: metrics.finalizados, color: '#10b981' },
  ] : [];

  return (
    <div className="w-full h-[220px]">
      {/* 1. Screen Reader Tabular Representation */}
      <div className="sr-only">
        <table>
          <caption>Service Tickets status</caption>
          <thead>
            <tr>
              <th scope="col">Status</th>
              <th scope="col">Count</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Pending</td>
              <td>{metrics?.pendientes ?? 0}</td>
            </tr>
            <tr>
              <td>In Progress</td>
              <td>{metrics?.en_proceso ?? 0}</td>
            </tr>
            <tr>
              <td>Completed</td>
              <td>{metrics?.finalizados ?? 0}</td>
            </tr>
            <tr>
              <td>Total</td>
              <td>{metrics?.total_tickets ?? 0}</td>
            </tr>
          </tbody>
        </table>
      </div>

      {/* 2. Visual Graphical Representation (Hidden from Assistive Tech) */}
      <div className="w-full h-full" aria-hidden="true" data-testid="ticket-breakdown-graphic">
        {!metrics || metrics.total_tickets === 0 ? (
          <div className="w-full h-full flex justify-center items-center text-slate-400 text-xs">
            No tickets registered.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={data}
                cx="50%"
                cy="50%"
                innerRadius={50}
                outerRadius={70}
                paddingAngle={5}
                dataKey="value"
              >
                {data.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip />
              <Legend verticalAlign="bottom" height={36} iconType="circle" wrapperStyle={{ fontSize: 11 }} />
            </PieChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
};
