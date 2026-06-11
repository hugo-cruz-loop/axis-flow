import React from 'react';
import { WidgetErrorBoundary } from './WidgetErrorBoundary';

interface WidgetCardProps {
  title: string;
  description?: string;
  isLoading: boolean;
  isError: boolean;
  error?: Error | null;
  onRefetch: () => void;
  isRefetching?: boolean;
  children: React.ReactNode;
}

export const WidgetCardContent: React.FC<WidgetCardProps> = ({
  title,
  description,
  isLoading,
  isError,
  error,
  onRefetch,
  isRefetching,
  children,
}) => {
  return (
    <section 
      className="flex flex-col h-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-5 shadow-sm transition hover:shadow-md focus-within:ring-2 focus-within:ring-indigo-500"
      aria-labelledby={`widget-title-${title.replace(/\s+/g, '-').toLowerCase()}`}
    >
      {/* Widget Header */}
      <div className="flex justify-between items-start mb-4">
        <div>
          <h3 
            id={`widget-title-${title.replace(/\s+/g, '-').toLowerCase()}`}
            className="text-base font-semibold text-slate-900 dark:text-slate-100"
          >
            {title}
          </h3>
          {description && (
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{description}</p>
          )}
        </div>
        <button
          onClick={onRefetch}
          disabled={isLoading || isRefetching}
          className="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-50 transition focus:outline-none focus:ring-2 focus:ring-indigo-500"
          aria-label={`Refresh data for ${title}`}
        >
          <svg
            className={`h-4 w-4 ${isRefetching ? 'animate-spin' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 7.89M9 11l3-3 3 3m-3-3v12" />
          </svg>
        </button>
      </div>

      {/* Dynamic View States */}
      <div className="flex-1 flex flex-col justify-center min-h-[180px]">
        {isLoading ? (
          <div className="w-full space-y-3 animate-pulse" aria-hidden="true" data-testid="widget-card-loading">
            <div className="h-4 bg-slate-200 dark:bg-slate-700 rounded w-1/3"></div>
            <div className="h-12 bg-slate-100 dark:bg-slate-800 rounded w-full"></div>
            <div className="h-3 bg-slate-200 dark:bg-slate-700 rounded w-2/3"></div>
          </div>
        ) : isError ? (
          <div className="text-center p-4" role="alert" data-testid="widget-card-error">
            <div className="inline-flex items-center justify-center w-10 h-10 rounded-full bg-rose-50 dark:bg-rose-950 text-rose-600 dark:text-rose-400 mb-2">
              ⚠️
            </div>
            <p className="text-sm font-semibold text-slate-800 dark:text-slate-200">
              Error fetching metric data
            </p>
            <p className="text-xs text-slate-500 mt-1">
              {error?.message || 'Please check your connection.'}
            </p>
          </div>
        ) : (
          <div className="w-full h-full flex-1 flex flex-col justify-between" data-testid="widget-card-content">
            {children}
          </div>
        )}
      </div>
    </section>
  );
};

// Safe boundary export
export const WidgetCard: React.FC<WidgetCardProps> = (props) => (
  <WidgetErrorBoundary title={props.title} onReset={props.onRefetch}>
    <WidgetCardContent {...props} />
  </WidgetErrorBoundary>
);
