import React from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { ServicesLocalidadItem } from '../../schemas/dashboard-schemas';

interface ServicesTableProps {
  items: ServicesLocalidadItem[];
}

export const ServicesTable: React.FC<ServicesTableProps> = ({ items }) => {
  // Map combined name for chart display
  const chartData = items.map((item) => ({
    name: item.servicio_nombre,
    displayName: `${item.servicio_nombre} (${item.localidad_nombre})`,
    assigned: item.empleados_asignados,
    key: `${item.localidad_id}-${item.servicio_id}`,
  }));

  return (
    <div className="w-full space-y-4">
      {/* 1. Screen Reader Tabular Representation */}
      <div className="sr-only">
        <table>
          <caption>Service Locations and Assigned Staff</caption>
          <thead>
            <tr>
              <th scope="col">Location</th>
              <th scope="col">Service Name</th>
              <th scope="col">Assigned Staff</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={`${item.localidad_id}-${item.servicio_id}`}>
                <td>{item.localidad_nombre}</td>
                <td>{item.servicio_nombre}</td>
                <td>{item.empleados_asignados} employees</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* 2. Visual Graphical Representation (Hidden from Assistive Tech) */}
      <div className="w-full h-[220px]" aria-hidden="true" data-testid="services-table-graphic">
        {items.length === 0 ? (
          <div className="w-full h-full flex justify-center items-center text-slate-400 text-xs">
            No service locations assigned.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <XAxis dataKey="name" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip formatter={(value) => [`${value} staff`, 'Assigned']} />
              <Bar dataKey="assigned" fill="#3b82f6" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Visual Table for sighted users */}
      {items.length > 0 && (
        <div className="overflow-x-auto border border-slate-200 dark:border-slate-800 rounded-lg" aria-hidden="true">
          <table className="min-w-full divide-y divide-slate-200 dark:divide-slate-800 text-xs">
            <thead className="bg-slate-50 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
              <tr>
                <th scope="col" className="px-4 py-2 text-left font-semibold">Location</th>
                <th scope="col" className="px-4 py-2 text-left font-semibold">Service Name</th>
                <th scope="col" className="px-4 py-2 text-right font-semibold">Assigned Staff</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-900 bg-white dark:bg-slate-900 text-slate-800 dark:text-slate-200">
              {items.map((item) => (
                <tr key={`${item.localidad_id}-${item.servicio_id}`} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-2 font-medium">{item.localidad_nombre}</td>
                  <td className="px-4 py-2">{item.servicio_nombre}</td>
                  <td className="px-4 py-2 text-right font-bold">{item.empleados_asignados}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
