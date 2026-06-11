import React from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { ClientEvaluationItem } from '../../schemas/dashboard-schemas';

interface EvaluationChartProps {
  data: ClientEvaluationItem[];
}

export const EvaluationChart: React.FC<EvaluationChartProps> = ({ data }) => {
  return (
    <div className="w-full h-[220px]">
      {/* 1. Screen Reader Tabular Representation */}
      <div className="sr-only">
        <table>
          <caption>Client Evaluation Scores</caption>
          <thead>
            <tr>
              <th scope="col">Client Name</th>
              <th scope="col">Evaluation Index</th>
              <th scope="col">Reviews Count</th>
            </tr>
          </thead>
          <tbody>
            {data.map((item) => (
              <tr key={item.cliente_id}>
                <td>{item.cliente_nombre}</td>
                <td>{item.promedio_evaluacion} out of 10</td>
                <td>{item.total_evaluaciones} reviews</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* 2. Visual Graphical Representation (Hidden from Assistive Tech) */}
      <div className="w-full h-full" aria-hidden="true" data-testid="evaluation-chart-graphic">
        {data.length === 0 ? (
          <div className="w-full h-full flex justify-center items-center text-slate-400 text-xs">
            No evaluations registered.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <XAxis dataKey="cliente_nombre" tick={{ fontSize: 10 }} />
              <YAxis domain={[0, 10]} tick={{ fontSize: 10 }} />
              <Tooltip />
              <Bar dataKey="promedio_evaluacion" fill="#4f46e5" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
};
